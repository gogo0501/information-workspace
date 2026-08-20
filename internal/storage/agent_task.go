// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"miniflux.app/v2/internal/model"
)

func (s *Storage) ClaimAgentTask(workerID string, lease time.Duration) (*model.AgentTask, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	task := &model.AgentTask{}
	err = tx.QueryRow(`
		SELECT a.id, a.entry_id, a.source_id, f.user_id, a.attempt
		FROM agent_tasks a
		JOIN workspace_sources ws ON ws.id=a.source_id
		JOIN feeds f ON f.id=ws.feed_id
		WHERE (a.status IN ('pending','retry') AND a.next_run_at <= now())
		   OR (a.status='running' AND a.lease_expires_at <= now())
		ORDER BY a.next_run_at,a.id FOR UPDATE OF a SKIP LOCKED LIMIT 1`,
	).Scan(&task.ID, &task.EntryID, &task.SourceID, &task.UserID, &task.Attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	leaseUntil := time.Now().Add(lease)
	if _, err = tx.Exec(`UPDATE agent_tasks SET status='running', attempt=attempt+1, lease_owner=$1, lease_expires_at=$2, last_error='' WHERE id=$3`, workerID, leaseUntil, task.ID); err != nil {
		return nil, err
	}
	task.Attempt++
	task.LeaseOwner = workerID
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Storage) CompleteAgentTask(task *model.AgentTask, insight map[string]any) error {
	payload, err := json.Marshal(insight)
	if err != nil {
		return fmt.Errorf("storage: unable to encode entry insight: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE entries SET insight=$1::jsonb, changed_at=now() WHERE id=$2`, payload, task.EntryID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE agent_tasks SET status='completed', completed_at=now(), lease_owner='', lease_expires_at=NULL WHERE id=$1`, task.ID); err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO notification_outbox (user_id, entry_id, source_id, channel, target, payload)
		SELECT s.user_id, $1, s.source_id, s.channel,
		       CASE WHEN s.channel='ntfy' THEN s.ntfy_topic ELSE s.webhook_url END,
		       jsonb_build_object(
		           'entry_id', e.id, 'title', e.title, 'url', e.url,
		           'published_at', e.published_at, 'insight', $2::jsonb)
		FROM workspace_subscriptions s
		JOIN entries e ON e.id=$1
		WHERE s.source_id=$3 AND s.enabled=true AND s.notification_enabled=true
		  AND s.channel <> ''
		  AND CASE WHEN s.channel='ntfy' THEN s.ntfy_topic ELSE s.webhook_url END <> ''
		ON CONFLICT (user_id, entry_id, channel, target) DO NOTHING`,
		task.EntryID, payload, task.SourceID)
	if err != nil {
		return fmt.Errorf("storage: unable to create notification outbox: %w", err)
	}
	return tx.Commit()
}

func (s *Storage) RetryAgentTask(taskID int64, attempt int, cause error) error {
	backoff := time.Duration(1<<min(attempt, 8)) * time.Second
	status := "retry"
	if attempt >= 5 {
		status = "failed"
	}
	_, err := s.db.Exec(`UPDATE agent_tasks SET status=$1, next_run_at=now()+$2::interval, last_error=$3, lease_owner='', lease_expires_at=NULL WHERE id=$4`, status, fmt.Sprintf("%f seconds", backoff.Seconds()), cause.Error(), taskID)
	return err
}
