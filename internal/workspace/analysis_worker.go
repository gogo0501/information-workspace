// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"log/slog"
	"time"

	"miniflux.app/v2/internal/storage"
)

type AnalysisWorker struct {
	store *storage.Storage
	name  string
}

func NewAnalysisWorker(store *storage.Storage, name string) *AnalysisWorker {
	return &AnalysisWorker{store: store, name: name}
}

func (w *AnalysisWorker) Run(shutdown <-chan struct{}) {
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

func (w *AnalysisWorker) consumeOne() {
	task, err := w.store.ClaimAgentTask(w.name, 2*time.Minute)
	if err != nil {
		slog.Error("workspace analysis claim failed", slog.Any("error", err))
		return
	}
	if task == nil {
		return
	}
	entry, err := w.store.NewEntryQueryBuilder(task.UserID).WithEntryIDs(task.EntryID).GetEntry()
	if err == nil && entry == nil {
		err = fmt.Errorf("entry %d not found", task.EntryID)
	}
	if err != nil {
		w.store.RetryAgentTask(task.ID, task.Attempt, err)
		return
	}
	chain := NewKnowledgeChain(NewService(w.store))
	insight := chain.AnalyzeEntry(entry)
	if err := w.store.CompleteAgentTask(task, insight); err != nil {
		if retryErr := w.store.RetryAgentTask(task.ID, task.Attempt, err); retryErr != nil {
			slog.Error("workspace analysis retry failed", slog.Any("error", retryErr))
		}
	}
}
