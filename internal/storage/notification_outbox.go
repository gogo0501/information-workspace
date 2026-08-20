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

func (s *Storage) ClaimNotificationOutbox(workerID string, lease time.Duration) (*model.NotificationOutbox, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	event := &model.NotificationOutbox{}
	err = tx.QueryRow(`
		SELECT id,user_id,entry_id,source_id,channel,target,payload,attempt
		FROM notification_outbox
		WHERE (status IN ('pending','retry') AND next_run_at <= now())
		   OR (status='running' AND lease_expires_at <= now())
		ORDER BY next_run_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(
		&event.ID, &event.UserID, &event.EntryID, &event.SourceID,
		&event.Channel, &event.Target, &event.Payload, &event.Attempt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	leaseUntil := time.Now().Add(lease)
	if _, err = tx.Exec(`UPDATE notification_outbox SET status='running', attempt=attempt+1, lease_owner=$1, lease_expires_at=$2, last_error='' WHERE id=$3`, workerID, leaseUntil, event.ID); err != nil {
		return nil, err
	}
	event.Attempt++
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *Storage) CompleteNotificationOutbox(id int64) error {
	_, err := s.db.Exec(`UPDATE notification_outbox SET status='delivered', delivered_at=now(), lease_owner='', lease_expires_at=NULL WHERE id=$1`, id)
	return err
}

func (s *Storage) RetryNotificationOutbox(id int64, attempt int, cause error) error {
	backoff := time.Duration(1<<min(attempt, 8)) * time.Second
	status := "retry"
	if attempt >= 5 {
		status = "failed"
	}
	_, err := s.db.Exec(`UPDATE notification_outbox SET status=$1, next_run_at=now()+$2::interval, last_error=$3, lease_owner='', lease_expires_at=NULL WHERE id=$4`, status, fmt.Sprintf("%f seconds", backoff.Seconds()), cause.Error(), id)
	return err
}
