// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model // import "miniflux.app/v2/internal/model"

import "time"

// WorkspaceOutbox represents a notification that must be delivered after an
// entry has been committed. The record is intentionally independent from the
// HTTP request and survives a process restart.
type WorkspaceOutbox struct {
	ID             int64
	UserID         int64
	EntryID        int64
	Attempts       int
	LeaseOwner     string
	LeaseExpiresAt *time.Time
}
