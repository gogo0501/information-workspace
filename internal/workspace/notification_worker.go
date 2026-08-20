// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"miniflux.app/v2/internal/storage"
)

type NotificationWorker struct {
	store  *storage.Storage
	client *http.Client
	name   string
}

func NewNotificationWorker(store *storage.Storage, name string) *NotificationWorker {
	return &NotificationWorker{store: store, name: name, client: &http.Client{Timeout: 10 * time.Second}}
}

func (w *NotificationWorker) Run(shutdown <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-shutdown:
			return
		case <-ticker.C:
			w.consumeOne()
		}
	}
}

func (w *NotificationWorker) consumeOne() {
	event, err := w.store.ClaimNotificationOutbox(w.name, 2*time.Minute)
	if err != nil {
		slog.Error("workspace notification claim failed", slog.Any("error", err))
		return
	}
	if event == nil {
		return
	}
	target := event.Target
	contentType := "application/json"
	if event.Channel == "ntfy" {
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			target = "https://ntfy.sh/" + strings.TrimPrefix(target, "/")
		}
		contentType = "text/plain; charset=utf-8"
	}
	request, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(event.Payload))
	if err == nil {
		request.Header.Set("Content-Type", contentType)
		var response *http.Response
		response, err = w.client.Do(request)
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("notification target returned HTTP %d", response.StatusCode)
			}
		}
	}
	if err != nil {
		w.store.RetryNotificationOutbox(event.ID, event.Attempt, err)
		return
	}
	if err := w.store.CompleteNotificationOutbox(event.ID); err != nil {
		slog.Error("workspace notification completion failed", slog.Any("error", err))
	}
}
