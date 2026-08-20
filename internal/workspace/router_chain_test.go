// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import "testing"

func TestRouteIntentSelectsDedicatedChains(t *testing.T) {
	tests := map[string]string{
		"订阅 https://go.dev/blog/feed.atom": "subscribe",
		"总结最近的 Go 文章":                      "knowledge",
		"最近有哪些资料":                          "default",
	}
	for input, expected := range tests {
		if got := routeIntent(input); got != expected {
			t.Errorf("routeIntent(%q)=%q, want %q", input, got, expected)
		}
	}
}

func TestExtractHTTPURL(t *testing.T) {
	got := extractHTTPURL("请订阅 https://example.com/feed.xml 谢谢")
	if got != "https://example.com/feed.xml" {
		t.Fatalf("extractHTTPURL()=%q", got)
	}
}
