// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"log/slog"
	"time"

	feedHandler "miniflux.app/v2/internal/reader/handler"
	"miniflux.app/v2/internal/storage"
)

type IngestWorker struct {
	store *storage.Storage
	name  string
}

func NewIngestWorker(store *storage.Storage, name string) *IngestWorker {
	return &IngestWorker{store: store, name: name}
}

func (w *IngestWorker) Run(shutdown <-chan struct{}) {
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

func (w *IngestWorker) consumeOne() {
	job, err := w.store.ClaimFeedRefreshJob(w.name, 2*time.Minute)
	if err != nil {
		slog.Error("workspace ingest claim failed", slog.Any("error", err))
		return
	}
	if job == nil {
		return
	}
	source, err := w.store.WorkspaceSourceByID(job.SourceID)
	if err != nil || source == nil {
		if err == nil {
			err = fmt.Errorf("workspace source %d not found", job.SourceID)
		}
		w.store.RetryFeedRefreshJob(job.ID, job.Attempt, err)
		return
	}
	userID, err := w.store.WorkspaceSourceOwner(job.SourceID)
	if err == nil {
		if refreshErr := feedHandler.RefreshFeed(w.store, userID, source.FeedID, false); refreshErr != nil {
			err = refreshErr.Error()
		}
	}
	if err != nil {
		if retryErr := w.store.RetryFeedRefreshJob(job.ID, job.Attempt, err); retryErr != nil {
			slog.Error("workspace ingest retry update failed", slog.Any("error", retryErr))
		}
		return
	}
	interval := time.Duration(source.RefreshIntervalSeconds) * time.Second
	if interval < time.Minute {
		interval = 30 * time.Minute
	}
	if err := w.store.CompleteFeedRefreshJob(job.ID, job.SourceID, time.Now().Add(interval)); err != nil {
		slog.Error("workspace ingest completion failed", slog.Any("error", err))
	}
}
