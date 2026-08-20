// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"miniflux.app/v2/internal/model"
)

type KnowledgeChain struct{ service *Service }

func NewKnowledgeChain(service *Service) *KnowledgeChain { return &KnowledgeChain{service: service} }

func (c *KnowledgeChain) Run(userID int64, input string) (*Answer, error) {
	documents, err := c.service.Search(userID, input, 8)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return &Answer{Route: "knowledge", Answer: "当前可见资料中没有足够证据完成总结。", Grounded: false}, nil
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "根据 %d 篇已入库资料归纳：\n", len(documents))
	for _, document := range documents {
		summary, _ := document.Insight["summary"].(string)
		if summary == "" {
			summary = document.Snippet
		}
		fmt.Fprintf(&builder, "- %s：%s [%s]\n", document.Title, summary, document.URL)
	}
	return &Answer{Route: "knowledge", Answer: builder.String(), Documents: documents, Grounded: true}, nil
}

func (c *KnowledgeChain) AnalyzeEntry(entry *model.Entry) map[string]any {
	text := plainText(entry.Content)
	summary := truncateRunes(text, 360)
	confidence := 0.85
	if summary == "" {
		summary = truncateRunes(entry.Title, 360)
		confidence = 0.45
	}
	keywords := topKeywords(entry.Title+" "+text, 6)
	return map[string]any{
		"summary": summary, "topics": keywords, "keywords": keywords,
		"confidence": confidence, "evidence_entry_ids": []int64{entry.ID},
	}
}

func plainText(content string) string {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return strings.Join(strings.Fields(content), " ")
	}
	return strings.Join(strings.Fields(document.Text()), " ")
}

func truncateRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}

func topKeywords(value string, limit int) []string {
	counts := make(map[string]int)
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		if len([]rune(token)) < 3 {
			continue
		}
		counts[token]++
	}
	words := make([]string, 0, len(counts))
	for word := range counts {
		words = append(words, word)
	}
	sort.Slice(words, func(i, j int) bool {
		if counts[words[i]] == counts[words[j]] {
			return words[i] < words[j]
		}
		return counts[words[i]] > counts[words[j]]
	})
	if len(words) > limit {
		words = words[:limit]
	}
	return words
}
