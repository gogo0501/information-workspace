// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"miniflux.app/v2/internal/model"
)

func (s *Storage) WorkspaceDocuments(userID int64, query string, limit int) ([]model.WorkspaceDocument, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	args := []any{userID}
	where := `s.user_id=$1 AND s.enabled=true`
	snippet := `left(regexp_replace(coalesce(e.content,''), '<[^>]+>', ' ', 'g'), 280)`
	if query != "" {
		where += ` AND e.document_vectors @@ plainto_tsquery('simple', $2)`
		snippet = `ts_headline('simple', coalesce(e.content,''), plainto_tsquery('simple', $2), 'MaxWords=45, MinWords=12')`
		args = append(args, query)
	}
	args = append(args, limit)
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT e.id, e.title, e.url, f.title, e.published_at, %s, e.insight
		FROM entries e
		JOIN workspace_sources ws ON ws.feed_id=e.feed_id AND ws.enabled=true
		JOIN workspace_subscriptions s ON s.source_id=ws.id
		JOIN feeds f ON f.id=e.feed_id
		WHERE %s
		ORDER BY e.published_at DESC, e.id DESC
		LIMIT $%d`, snippet, where, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("storage: unable to search workspace documents: %w", err)
	}
	defer rows.Close()
	result := make([]model.WorkspaceDocument, 0)
	for rows.Next() {
		document := model.WorkspaceDocument{}
		var rawInsight []byte
		if err := rows.Scan(&document.ID, &document.Title, &document.URL, &document.Feed, &document.Published, &document.Snippet, &rawInsight); err != nil {
			return nil, fmt.Errorf("storage: unable to scan workspace document: %w", err)
		}
		if len(rawInsight) > 0 && string(rawInsight) != "{}" {
			if err := json.Unmarshal(rawInsight, &document.Insight); err != nil && err != sql.ErrNoRows {
				return nil, fmt.Errorf("storage: unable to decode workspace insight: %w", err)
			}
		}
		result = append(result, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: workspace document query failed: %w", err)
	}
	return result, nil
}
