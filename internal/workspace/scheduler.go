// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"log/slog"
	"time"

	"miniflux.app/v2/internal/storage"
)

type Scheduler struct{ store *storage.Storage }

func NewScheduler(store *storage.Storage) *Scheduler { return &Scheduler{store: store} }

func (s *Scheduler) Run(shutdown <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-shutdown:
			return
		case <-ticker.C:
			if err := s.store.ScheduleDueWorkspaceJobs(); err != nil {
				slog.Error("workspace scheduler failed", slog.Any("error", err))
			}
		}
	}
}
