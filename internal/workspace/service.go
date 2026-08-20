// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace // import "miniflux.app/v2/internal/workspace"

import (
	"fmt"
	"strings"
	"time"

	"miniflux.app/v2/internal/storage"
)

type Document struct {
	ID        int64          `json:"id"`
	Title     string         `json:"title"`
	URL       string         `json:"url"`
	Feed      string         `json:"feed"`
	Published time.Time      `json:"published_at"`
	Snippet   string         `json:"snippet"`
	Insight   map[string]any `json:"insight,omitempty"`
}

type Answer struct {
	Route     string     `json:"route"`
	Answer    string     `json:"answer"`
	Documents []Document `json:"documents"`
	Grounded  bool       `json:"grounded"`
}

type Service struct{ store *storage.Storage }

func NewService(store *storage.Storage) *Service { return &Service{store: store} }

func (s *Service) Search(userID int64, query string, limit int) ([]Document, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	entries, err := s.store.WorkspaceDocuments(userID, strings.TrimSpace(query), limit)
	if err != nil {
		return nil, err
	}
	documents := make([]Document, 0, len(entries))
	for _, entry := range entries {
		documents = append(documents, Document{
			ID: entry.ID, Title: entry.Title, URL: entry.URL, Feed: entry.Feed,
			Published: entry.Published, Snippet: snippet(entry.Snippet, 240), Insight: entry.Insight,
		})
	}
	return documents, nil
}

// Ask is the default grounded chain. It intentionally does not invent an
// answer: every statement is derived from a PostgreSQL full-text query result.
func (s *Service) Ask(userID int64, question string) (*Answer, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}
	documents, err := s.Search(userID, question, 8)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return &Answer{Route: "default", Answer: "当前资料库没有足够证据回答这个问题。", Documents: documents, Grounded: false}, nil
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "检索到 %d 篇相关资料：\n", len(documents))
	for _, document := range documents {
		fmt.Fprintf(&builder, "- %s（%s）\n", document.Title, document.URL)
	}
	return &Answer{Route: "default", Answer: builder.String(), Documents: documents, Grounded: true}, nil
}

func snippet(content string, limit int) string {
	content = strings.Join(strings.Fields(content), " ")
	if len(content) <= limit {
		return content
	}
	return content[:limit] + "..."
}
