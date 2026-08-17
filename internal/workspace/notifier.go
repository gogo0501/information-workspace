// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workspace contains the small amount of application logic added by
// the information workspace. Miniflux remains responsible for fetching and
// parsing feeds; this package only consumes durable post-ingest events.
package workspace // import "miniflux.app/v2/internal/workspace"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"miniflux.app/v2/internal/storage"
)

// Notifier consumes the PostgreSQL outbox. A webhook is used when configured
// on the feed; otherwise the event is acknowledged so an unconfigured local
// installation never builds an unbounded queue.
type Notifier struct {
	store  *storage.Storage
	client *http.Client
	name   string
}

func NewNotifier(store *storage.Storage, name string) *Notifier {
	return &Notifier{store: store, client: &http.Client{Timeout: 10 * time.Second}, name: name}
}

func (n *Notifier) Run(shutdown <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-shutdown:
			return
		case <-ticker.C:
			n.consumeOne()
		}
	}
}

func (n *Notifier) consumeOne() {
	event, err := n.store.ClaimWorkspaceOutbox(n.name, 2*time.Minute)
	if err != nil {
		slog.Error("workspace outbox claim failed", slog.Any("error", err))
		return
	}
	if event == nil {
		return
	}

	entry, err := n.store.NewEntryQueryBuilder(event.UserID).WithEntryIDs(event.EntryID).GetEntry()
	if err != nil {
		n.fail(event.ID, event.Attempts, err)
		return
	}
	if entry == nil || entry.Feed == nil || entry.Feed.WebhookURL == "" {
		// The durable event is still useful for auditability, but delivery is
		// optional. It is acknowledged when no channel is configured.
		n.store.MarkWorkspaceOutboxDelivered(event.ID)
		return
	}

	payload := map[string]any{
		"title":    entry.Title,
		"url":      entry.URL,
		"content":  entry.Content,
		"feed":     entry.Feed.Title,
		"entry_id": entry.ID,
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequest(http.MethodPost, entry.Feed.WebhookURL, bytes.NewReader(body))
	if err == nil {
		request.Header.Set("Content-Type", "application/json")
		var response *http.Response
		response, err = n.client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
			}
		}
	}
	if err != nil {
		n.fail(event.ID, event.Attempts, err)
		return
	}
	if err := n.store.MarkWorkspaceOutboxDelivered(event.ID); err != nil {
		slog.Error("workspace outbox update failed", slog.Any("error", err), slog.Int64("id", event.ID))
	}
}

func (n *Notifier) fail(id int64, attempts int, err error) {
	if err == nil {
		err = errors.New("unknown delivery error")
	}
	if retryErr := n.store.RetryWorkspaceOutbox(id, attempts, err); retryErr != nil {
		slog.Error("workspace outbox retry update failed", slog.Any("error", retryErr), slog.Int64("id", id))
	}
}
