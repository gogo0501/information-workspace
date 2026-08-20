// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"miniflux.app/v2/internal/model"
)

func scanWorkspaceSource(row interface{ Scan(...any) error }) (*model.WorkspaceSource, error) {
	source := &model.WorkspaceSource{}
	err := row.Scan(
		&source.ID, &source.CanonicalURL, &source.FeedURL, &source.FeedID,
		&source.Title, &source.SiteURL, &source.Discovery, &source.SourceType,
		&source.Enabled, &source.RefreshIntervalSeconds, &source.NextCheckAt, &source.LastError,
	)
	return source, err
}

func (s *Storage) WorkspaceSourceByCanonicalURL(canonicalURL string) (*model.WorkspaceSource, error) {
	source, err := scanWorkspaceSource(s.db.QueryRow(`
		SELECT id, canonical_url, feed_url, feed_id, title, site_url, discovery,
		       source_type, enabled, refresh_interval_seconds, next_check_at, last_error
		FROM workspace_sources WHERE canonical_url=$1`, canonicalURL))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: unable to find workspace source: %w", err)
	}
	return source, nil
}

func (s *Storage) WorkspaceSourceByID(sourceID int64) (*model.WorkspaceSource, error) {
	source, err := scanWorkspaceSource(s.db.QueryRow(`
		SELECT id, canonical_url, feed_url, feed_id, title, site_url, discovery,
		       source_type, enabled, refresh_interval_seconds, next_check_at, last_error
		FROM workspace_sources WHERE id=$1`, sourceID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("storage: unable to find workspace source: %w", err)
	}
	return source, nil
}

func (s *Storage) WorkspaceSourceOwner(sourceID int64) (int64, error) {
	var userID int64
	err := s.db.QueryRow(`SELECT f.user_id FROM workspace_sources ws JOIN feeds f ON f.id=ws.feed_id WHERE ws.id=$1`, sourceID).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("storage: unable to find workspace source owner: %w", err)
	}
	return userID, nil
}

func (s *Storage) CreateWorkspaceSource(source *model.WorkspaceSource, subscription *model.WorkspaceSubscription) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("storage: unable to start source transaction: %w", err)
	}
	defer tx.Rollback()

	err = tx.QueryRow(`
		INSERT INTO workspace_sources
			(canonical_url, feed_url, feed_id, title, site_url, discovery, source_type, refresh_interval_seconds)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (canonical_url) DO UPDATE SET updated_at=now()
		RETURNING id, canonical_url, feed_url, feed_id, title, site_url, discovery,
		          source_type, enabled, refresh_interval_seconds, next_check_at, last_error`,
		source.CanonicalURL, source.FeedURL, source.FeedID, source.Title, source.SiteURL,
		source.Discovery, source.SourceType, source.RefreshIntervalSeconds,
	).Scan(
		&source.ID, &source.CanonicalURL, &source.FeedURL, &source.FeedID, &source.Title,
		&source.SiteURL, &source.Discovery, &source.SourceType, &source.Enabled,
		&source.RefreshIntervalSeconds, &source.NextCheckAt, &source.LastError,
	)
	if err != nil {
		return fmt.Errorf("storage: unable to create workspace source: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO workspace_subscriptions
			(user_id, source_id, enabled, notification_enabled, channel, webhook_url, ntfy_topic)
		VALUES ($1,$2,true,$3,$4,$5,$6)
		ON CONFLICT (user_id, source_id) DO UPDATE SET
			enabled=true, notification_enabled=EXCLUDED.notification_enabled,
			channel=EXCLUDED.channel, webhook_url=EXCLUDED.webhook_url,
			ntfy_topic=EXCLUDED.ntfy_topic, updated_at=now()`,
		subscription.UserID, source.ID, subscription.NotificationEnabled,
		subscription.Channel, subscription.WebhookURL, subscription.NtfyTopic)
	if err != nil {
		return fmt.Errorf("storage: unable to create workspace subscription: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO feed_refresh_jobs (source_id, next_run_at)
		VALUES ($1, now())
		ON CONFLICT (source_id) WHERE status IN ('pending','running','retry') DO NOTHING`, source.ID)
	if err != nil {
		return fmt.Errorf("storage: unable to schedule initial feed refresh: %w", err)
	}
	return tx.Commit()
}

func (s *Storage) UnsubscribeWorkspace(userID, sourceID int64) error {
	_, err := s.db.Exec(`UPDATE workspace_subscriptions SET enabled=false, updated_at=now() WHERE user_id=$1 AND source_id=$2`, userID, sourceID)
	if err != nil {
		return fmt.Errorf("storage: unable to disable workspace subscription: %w", err)
	}
	return nil
}

func (s *Storage) CreateAgentTask(tx *sql.Tx, entry *model.Entry) error {
	_, err := tx.Exec(`
		INSERT INTO agent_tasks (entry_id, source_id)
		SELECT $1, id FROM workspace_sources WHERE feed_id=$2
		ON CONFLICT (entry_id) DO NOTHING`, entry.ID, entry.FeedID)
	if err != nil {
		return fmt.Errorf("storage: unable to create agent task: %w", err)
	}
	return nil
}

func (s *Storage) ScheduleDueWorkspaceJobs() error {
	_, err := s.db.Exec(`
		INSERT INTO feed_refresh_jobs (source_id, next_run_at)
		SELECT id, now() FROM workspace_sources
		WHERE enabled=true AND next_check_at <= now()
		ON CONFLICT (source_id) WHERE status IN ('pending','running','retry') DO NOTHING`)
	if err != nil {
		return fmt.Errorf("storage: unable to schedule workspace jobs: %w", err)
	}
	return nil
}

func (s *Storage) ClaimFeedRefreshJob(workerID string, lease time.Duration) (*model.FeedRefreshJob, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job := &model.FeedRefreshJob{}
	err = tx.QueryRow(`
		SELECT id, source_id, attempt FROM feed_refresh_jobs
		WHERE (status IN ('pending','retry') AND next_run_at <= now())
		   OR (status='running' AND lease_expires_at <= now())
		ORDER BY next_run_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&job.ID, &job.SourceID, &job.Attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	leaseUntil := time.Now().Add(lease)
	if _, err = tx.Exec(`
		UPDATE feed_refresh_jobs SET status='running', attempt=attempt+1,
			lease_owner=$1, lease_expires_at=$2, started_at=now(), last_error=''
		WHERE id=$3`, workerID, leaseUntil, job.ID); err != nil {
		return nil, err
	}
	job.Attempt++
	job.LeaseOwner = workerID
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Storage) CompleteFeedRefreshJob(jobID, sourceID int64, nextCheck time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE feed_refresh_jobs SET status='completed', finished_at=now(), lease_owner='', lease_expires_at=NULL WHERE id=$1`, jobID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE workspace_sources SET next_check_at=$1, last_error='', updated_at=now() WHERE id=$2`, nextCheck, sourceID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Storage) RetryFeedRefreshJob(jobID int64, attempt int, cause error) error {
	backoff := time.Duration(1<<min(attempt, 8)) * time.Second
	status := "retry"
	if attempt >= 5 {
		status = "failed"
	}
	_, err := s.db.Exec(`UPDATE feed_refresh_jobs SET status=$1, next_run_at=now()+$2::interval, last_error=$3, lease_owner='', lease_expires_at=NULL WHERE id=$4`, status, fmt.Sprintf("%f seconds", backoff.Seconds()), cause.Error(), jobID)
	return err
}
