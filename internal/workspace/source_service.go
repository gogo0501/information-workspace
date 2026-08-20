// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/storage"
)

type SubscribeRequest struct {
	URL                    string `json:"url"`
	Title                  string `json:"title"`
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	NotificationEnabled    bool   `json:"notification_enabled"`
	Channel                string `json:"channel"`
	WebhookURL             string `json:"webhook_url"`
	NtfyTopic              string `json:"ntfy_topic"`
}

type SourceService struct {
	store      *storage.Storage
	discoverer *SourceDiscoverer
}

func NewSourceService(store *storage.Storage) *SourceService {
	return &SourceService{store: store, discoverer: NewSourceDiscoverer(&http.Client{Timeout: 15 * time.Second})}
}

func (s *SourceService) Subscribe(ctx context.Context, userID int64, request SubscribeRequest) (*model.WorkspaceSource, error) {
	discovery, err := s.discoverer.Discover(ctx, request.URL)
	if err != nil {
		return nil, err
	}
	canonicalURL := discovery.FeedURL
	if existing, lookupErr := s.store.WorkspaceSourceByCanonicalURL(canonicalURL); lookupErr != nil {
		return nil, lookupErr
	} else if existing != nil {
		subscription := &model.WorkspaceSubscription{
			UserID: userID, SourceID: existing.ID, NotificationEnabled: request.NotificationEnabled,
			Channel: request.Channel, WebhookURL: request.WebhookURL, NtfyTopic: request.NtfyTopic,
		}
		if err := s.store.CreateWorkspaceSource(existing, subscription); err != nil {
			return nil, err
		}
		return existing, nil
	}

	category, err := s.store.FirstCategory(userID)
	if err != nil {
		return nil, fmt.Errorf("workspace: unable to find default category: %w", err)
	}
	feed := discovery.Feed
	feed.UserID = userID
	feed.FeedURL = discovery.FeedURL
	feed.SiteURL = discovery.SiteURL
	if feed.SiteURL == "" {
		feed.SiteURL = request.URL
	}
	if request.Title != "" {
		feed.Title = strings.TrimSpace(request.Title)
	}
	if feed.Title == "" {
		feed.Title = feed.SiteURL
	}
	feed.WithCategoryID(category.ID)
	if err := s.store.CreateFeed(feed); err != nil {
		return nil, fmt.Errorf("workspace: unable to create canonical feed: %w", err)
	}
	interval := request.RefreshIntervalSeconds
	if interval < 60 {
		interval = 1800
	}
	source := &model.WorkspaceSource{
		CanonicalURL: canonicalURL, FeedURL: discovery.FeedURL, FeedID: feed.ID,
		Title: feed.Title, SiteURL: feed.SiteURL, Discovery: discovery.Discovery,
		SourceType: "feed", Enabled: true, RefreshIntervalSeconds: interval,
	}
	subscription := &model.WorkspaceSubscription{
		UserID: userID, NotificationEnabled: request.NotificationEnabled,
		Channel: request.Channel, WebhookURL: request.WebhookURL, NtfyTopic: request.NtfyTopic,
	}
	if err := s.store.CreateWorkspaceSource(source, subscription); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *SourceService) Unsubscribe(userID, sourceID int64) error {
	return s.store.UnsubscribeWorkspace(userID, sourceID)
}
