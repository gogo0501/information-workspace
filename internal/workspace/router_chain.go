// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"miniflux.app/v2/internal/storage"
)

type RouterChain struct {
	defaultChain   *DefaultChain
	subscribeChain *SubscribeChain
	knowledgeChain *KnowledgeChain
}

func NewRouterChain(store *storage.Storage) *RouterChain {
	service := NewService(store)
	return &RouterChain{
		defaultChain:   NewDefaultChain(service),
		subscribeChain: NewSubscribeChain(NewSourceService(store)),
		knowledgeChain: NewKnowledgeChain(service),
	}
}

func (r *RouterChain) Run(ctx context.Context, userID int64, input string) (*Answer, error) {
	switch routeIntent(input) {
	case "subscribe":
		return r.subscribeChain.Run(ctx, userID, input)
	case "knowledge":
		return r.knowledgeChain.Run(userID, input)
	default:
		return r.defaultChain.Run(userID, input)
	}
}

func routeIntent(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if strings.Contains(lower, "订阅") || strings.Contains(lower, "unsubscribe") || strings.Contains(lower, "subscribe") || strings.Contains(lower, "取消订阅") {
		return "subscribe"
	}
	for _, keyword := range []string{"总结", "归纳", "分析", "对比", "趋势", "摘要", "knowledge"} {
		if strings.Contains(lower, keyword) {
			return "knowledge"
		}
	}
	return "default"
}

var httpURLPattern = regexp.MustCompile(`https?://[^\s]+`)
var sourceIDPattern = regexp.MustCompile(`\d+`)

func extractHTTPURL(input string) string {
	value := httpURLPattern.FindString(input)
	return strings.TrimRight(value, "，。,.!?；;）)")
}

func extractSourceID(input string) int64 {
	value := sourceIDPattern.FindString(input)
	id, _ := strconv.ParseInt(value, 10, 64)
	return id
}

func subscribeAnswer(sourceID int64, title string) *Answer {
	return &Answer{Route: "subscribe", Grounded: true, Answer: fmt.Sprintf("已订阅 %s（来源 #%d），首次抓取任务已进入持久化队列。", title, sourceID)}
}
