// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage // import "miniflux.app/v2/internal/storage"

import (
	"database/sql"
	"fmt"
	"time"

	"miniflux.app/v2/internal/model"
)

func (s *Storage) createWorkspaceOutbox(tx *sql.Tx, entry *model.Entry) error {
	_, err := tx.Exec(`
		INSERT INTO workspace_outbox (user_id, entry_id)
		VALUES ($1, $2)
		ON CONFLICT (entry_id) DO NOTHING`, entry.UserID, entry.ID)
	if err != nil {
		return fmt.Errorf("storage: unable to enqueue workspace notification: %w", err)
	}
	return nil
}

// ClaimWorkspaceOutbox atomically leases one pending event. SKIP LOCKED lets
// several process instances consume the table without delivering an entry twice.
func (s *Storage) ClaimWorkspaceOutbox(workerID string, lease time.Duration) (*model.WorkspaceOutbox, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("storage: unable to start outbox transaction: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRow(`
		SELECT id, user_id, entry_id, attempts
		FROM workspace_outbox
		WHERE (status IN ('pending', 'retry') AND next_attempt_at <= now())
			OR (status = 'running' AND lease_expires_at <= now())
		ORDER BY next_attempt_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT 1`)

	outbox := &model.WorkspaceOutbox{}
	if err := row.Scan(&outbox.ID, &outbox.UserID, &outbox.EntryID, &outbox.Attempts); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("storage: unable to claim workspace notification: %w", err)
	}

	leaseUntil := time.Now().Add(lease)
	if _, err := tx.Exec(`
		UPDATE workspace_outbox
		SET status='running', attempts=attempts+1, lease_owner=$1, lease_expires_at=$2
		WHERE id=$3`, workerID, leaseUntil, outbox.ID); err != nil {
		return nil, fmt.Errorf("storage: unable to lease workspace notification: %w", err)
	}
	outbox.Attempts++
	outbox.LeaseOwner = workerID
	outbox.LeaseExpiresAt = &leaseUntil

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("storage: unable to commit outbox lease: %w", err)
	}
	return outbox, nil
}

func (s *Storage) MarkWorkspaceOutboxDelivered(id int64) error {
	_, err := s.db.Exec(`
		UPDATE workspace_outbox
		SET status='delivered', delivered_at=now(), lease_owner='', lease_expires_at=NULL, last_error=''
		WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("storage: unable to mark workspace notification delivered: %w", err)
	}
	return nil
}

func (s *Storage) RetryWorkspaceOutbox(id int64, attempts int, cause error) error {
	backoff := min(time.Duration(1<<min(attempts, 8))*time.Second, time.Hour)
	status := "retry"
	if attempts >= 5 {
		status = "failed"
		backoff = 0
	}
	_, err := s.db.Exec(`
		UPDATE workspace_outbox
		SET status=$1, next_attempt_at=now()+$2::interval, lease_owner='', lease_expires_at=NULL, last_error=$3
		WHERE id=$4`, status, fmt.Sprintf("%f seconds", backoff.Seconds()), cause.Error(), id)
	if err != nil {
		return fmt.Errorf("storage: unable to retry workspace notification: %w", err)
	}
	return nil
}
