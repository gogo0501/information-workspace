// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"strings"
	"testing"
)

func TestDiscoverFeedLinkResolvesRelativeURLsAndDeduplicates(t *testing.T) {
	html := `<html><head>
		<link rel="alternate" type="application/rss+xml" href="/feed.xml">
		<link rel="alternate" type="application/rss+xml" href="/feed.xml">
		<link rel="alternate" type="application/atom+xml" href="/atom.xml">
	</head></html>`
	links, err := discoverHTMLFeedLinks("https://example.com/blog/", strings.NewReader(html))
	if err != nil {
		t.Fatalf("discoverHTMLFeedLinks() error = %v", err)
	}
	if len(links) != 2 || links[0] != "https://example.com/feed.xml" || links[1] != "https://example.com/atom.xml" {
		t.Fatalf("unexpected links: %#v", links)
	}
}

func TestValidateSourceURLRejectsPrivateAndNonHTTPURLs(t *testing.T) {
	for _, rawURL := range []string{"file:///tmp/feed.xml", "http://localhost/feed", "http://127.0.0.1/feed", "http://10.0.0.1/feed"} {
		if err := validateSourceURL(rawURL); err == nil {
			t.Errorf("validateSourceURL(%q) accepted an unsafe URL", rawURL)
		}
	}
	if err := validateSourceURL("https://example.com/feed.xml"); err != nil {
		t.Fatalf("validateSourceURL() rejected public URL: %v", err)
	}
}
