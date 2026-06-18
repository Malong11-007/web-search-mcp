package sitemap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
)

func newMapClient() *httpclient.Client {
	return httpclient.New(&config.Config{
		HTTPTimeout:     10 * 1e9,
		MaxContentSize:  10 * 1024 * 1024,
		StealthMode:     false,
	})
}

func newMapRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func mapContentText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if tc, ok := result.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func TestMapHandler_EmptyURL(t *testing.T) {
	h := handler(newMapClient(), &config.Config{}, ratelimit.NewNoop())
	_, err := h(context.Background(), newMapRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMapHandler_InvalidURL(t *testing.T) {
	h := handler(newMapClient(), &config.Config{}, ratelimit.NewNoop())
	result, _ := h(context.Background(), newMapRequest(map[string]any{"url": "://bad"}))
	if !result.IsError {
		t.Error("expected error for invalid URL")
	}
}

func TestMapHandler_PageLinks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
			<a href="/page1">Duplicate</a>
			<a href="https://other.com/external">External</a>
			<a href="javascript:void(0)">JS</a>
			<a href="#section">Anchor</a>
		</body></html>`))
	}))
	defer ts.Close()

	h := handler(newMapClient(), &config.Config{RespectRobotsTXT: false}, ratelimit.NewNoop())
	result, _ := h(context.Background(), newMapRequest(map[string]any{
		"url":            ts.URL + "/",
		"check_sitemap":  false,
	}))
	text := mapContentText(result)
	if !strings.Contains(text, "/page1") || !strings.Contains(text, "/page2") {
		t.Errorf("expected /page1 and /page2 in output, got:\n%s", text)
	}
	if strings.Contains(text, "other.com") {
		t.Error("external links should be excluded")
	}
}

func TestMapHandler_SitemapEnabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sitemap.xml") {
			w.Write([]byte(`<?xml><urlset><url><loc>/from-sitemap</loc></url></urlset>`))
			return
		}
		w.Write([]byte(`<html><a href="/from-page">link</a></html>`))
	}))
	defer ts.Close()

	h := handler(newMapClient(), &config.Config{RespectRobotsTXT: false}, ratelimit.NewNoop())
	result, _ := h(context.Background(), newMapRequest(map[string]any{
		"url":           ts.URL + "/",
		"check_sitemap": true,
	}))
	text := mapContentText(result)
	if !strings.Contains(text, "/from-page") || !strings.Contains(text, "/from-sitemap") {
		t.Errorf("expected both page and sitemap URLs:\n%s", text)
	}
}

func TestMapHandler_Truncation(t *testing.T) {
	// Generate >200 unique links.
	var links strings.Builder
	for i := 0; i < 250; i++ {
		links.WriteString(`<a href="/page` + fmt.Sprint(i) + `">link</a>`)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>` + links.String() + `</body></html>`))
	}))
	defer ts.Close()

	h := handler(newMapClient(), &config.Config{RespectRobotsTXT: false}, ratelimit.NewNoop())
	result, _ := h(context.Background(), newMapRequest(map[string]any{
		"url":           ts.URL + "/",
		"check_sitemap": false,
	}))
	text := mapContentText(result)
	if !strings.Contains(text, "truncated") {
		t.Errorf("expected truncation notice for >200 URLs:\n%s", text)
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/sub/")

	tests := []struct {
		href string
		want string
	}{
		{"", ""},
		{"#anchor", ""},
		{"javascript:alert(1)", ""},
		{"mailto:a@b.com", ""},
		{"/absolute", "https://example.com/absolute"},
		{"relative", "https://example.com/sub/relative"},
		{"https://other.com/page", ""}, // cross-domain
	}
	for _, tt := range tests {
		got := resolveURL(base, tt.href)
		if got != tt.want {
			t.Errorf("resolveURL(%q, %q) = %q, want %q", base, tt.href, got, tt.want)
		}
	}
}
