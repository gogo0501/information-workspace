// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package model

import "time"

type WorkspaceSource struct {
	ID                     int64     `json:"id"`
	CanonicalURL           string    `json:"canonical_url"`
	FeedURL                string    `json:"feed_url"`
	FeedID                 int64     `json:"feed_id"`
	Title                  string    `json:"title"`
	SiteURL                string    `json:"site_url"`
	Discovery              string    `json:"discovery"`
	SourceType             string    `json:"source_type"`
	Enabled                bool      `json:"enabled"`
	RefreshIntervalSeconds int       `json:"refresh_interval_seconds"`
	NextCheckAt            time.Time `json:"next_check_at"`
	LastError              string    `json:"last_error,omitempty"`
}

type WorkspaceSubscription struct {
	UserID              int64  `json:"user_id"`
	SourceID            int64  `json:"source_id"`
	Enabled             bool   `json:"enabled"`
	NotificationEnabled bool   `json:"notification_enabled"`
	Channel             string `json:"channel"`
	WebhookURL          string `json:"webhook_url,omitempty"`
	NtfyTopic           string `json:"ntfy_topic,omitempty"`
}

type FeedRefreshJob struct {
	ID         int64
	SourceID   int64
	Attempt    int
	LeaseOwner string
}

type AgentTask struct {
	ID         int64
	EntryID    int64
	SourceID   int64
	UserID     int64
	Attempt    int
	LeaseOwner string
}

type NotificationOutbox struct {
	ID       int64
	UserID   int64
	EntryID  int64
	SourceID int64
	Channel  string
	Target   string
	Payload  []byte
	Attempt  int
}

type WorkspaceDocument struct {
	ID        int64
	Title     string
	URL       string
	Feed      string
	Published time.Time
	Snippet   string
	Insight   map[string]any
}
