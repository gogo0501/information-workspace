// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/reader/parser"
)

var ErrFeedNotFound = errors.New("workspace: no feed declaration found")

type DiscoveryResult struct {
	RequestedURL string
	FeedURL      string
	SiteURL      string
	Discovery    string
	Feed         *model.Feed
}

type SourceDiscoverer struct {
	Client  *http.Client
	MaxBody int64
}

func NewSourceDiscoverer(client *http.Client) *SourceDiscoverer {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &SourceDiscoverer{Client: client, MaxBody: 4 << 20}
}

func (d *SourceDiscoverer) Discover(ctx context.Context, rawURL string) (*DiscoveryResult, error) {
	if err := validateSourceURL(rawURL); err != nil {
		return nil, err
	}
	response, body, err := d.fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if feed, parseErr := parser.ParseFeed(response.Request.URL.String(), bytes.NewReader(body)); parseErr == nil {
		feedURL := response.Request.URL.String()
		if feed.FeedURL != "" {
			feedURL = feed.FeedURL
		}
		return &DiscoveryResult{RequestedURL: rawURL, FeedURL: feedURL, SiteURL: feed.SiteURL, Discovery: "direct", Feed: feed}, nil
	}

	links, err := discoverHTMLFeedLinks(response.Request.URL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		feedResponse, feedBody, fetchErr := d.fetch(ctx, link)
		if fetchErr != nil {
			continue
		}
		feed, parseErr := parser.ParseFeed(feedResponse.Request.URL.String(), bytes.NewReader(feedBody))
		if parseErr != nil {
			continue
		}
		feedURL := feedResponse.Request.URL.String()
		if feed.FeedURL != "" {
			feedURL = feed.FeedURL
		}
		return &DiscoveryResult{RequestedURL: rawURL, FeedURL: feedURL, SiteURL: feed.SiteURL, Discovery: "html_link", Feed: feed}, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrFeedNotFound, rawURL)
}

func (d *SourceDiscoverer) fetch(ctx context.Context, rawURL string) (*http.Response, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/feed+json, application/json, text/html")
	client := *d.Client
	redirects := 0
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		redirects++
		if redirects > 5 {
			return errors.New("workspace: too many redirects")
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("workspace: source returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, d.MaxBody))
	if err != nil {
		return nil, nil, err
	}
	return response, body, nil
}

func discoverHTMLFeedLinks(baseURL string, reader io.Reader) ([]string, error) {
	document, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	allowedTypes := map[string]bool{
		"application/rss+xml":   true,
		"application/atom+xml":  true,
		"application/feed+json": true,
		"application/json":      true,
	}
	seen := make(map[string]struct{})
	links := make([]string, 0, 2)
	document.Find("link[rel~='alternate'][href]").Each(func(_ int, selection *goquery.Selection) {
		typeName := strings.ToLower(strings.TrimSpace(selection.AttrOr("type", "")))
		if !allowedTypes[typeName] {
			return
		}
		href := strings.TrimSpace(selection.AttrOr("href", ""))
		resolved, resolveErr := base.Parse(href)
		if resolveErr != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") {
			return
		}
		value := resolved.String()
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		links = append(links, value)
	})
	return links, nil
}

func validateSourceURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("workspace: source URL must use http or https")
	}
	if parsed.User != nil {
		return fmt.Errorf("workspace: source URL userinfo is not allowed")
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
		return fmt.Errorf("workspace: private source address is not allowed")
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") || strings.HasSuffix(strings.ToLower(host), ".local") {
		return fmt.Errorf("workspace: local source address is not allowed")
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
