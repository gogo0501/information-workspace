// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"
	"strings"
)

type SubscribeChain struct{ service *SourceService }

func NewSubscribeChain(service *SourceService) *SubscribeChain {
	return &SubscribeChain{service: service}
}

func (c *SubscribeChain) Run(ctx context.Context, userID int64, input string) (*Answer, error) {
	if strings.Contains(input, "取消订阅") || strings.Contains(strings.ToLower(input), "unsubscribe") {
		sourceID := extractSourceID(input)
		if sourceID == 0 {
			return &Answer{Route: "subscribe", Answer: "请提供需要取消的来源编号。", Grounded: false}, nil
		}
		if err := c.service.Unsubscribe(userID, sourceID); err != nil {
			return nil, err
		}
		return &Answer{Route: "subscribe", Answer: fmt.Sprintf("已取消来源 #%d 的订阅。", sourceID), Grounded: true}, nil
	}
	url := extractHTTPURL(input)
	if url == "" {
		return &Answer{Route: "subscribe", Answer: "请提供要订阅的 http/https 网站或 Feed 地址。", Grounded: false}, nil
	}
	source, err := c.service.Subscribe(ctx, userID, SubscribeRequest{URL: url, RefreshIntervalSeconds: 1800})
	if err != nil {
		return nil, err
	}
	return subscribeAnswer(source.ID, source.Title), nil
}
