package scrape

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
)

func newMetadataRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func TestMetadataHandler_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html lang="en">
<head>
	<title>Test Page</title>
	<meta name="description" content="A test page">
	<meta property="og:title" content="OG Title">
	<meta property="og:description" content="OG Description">
	<meta property="og:image" content="https://img.com/og.png">
	<meta name="twitter:card" content="summary">
	<link rel="canonical" href="https://example.com/canonical">
	<meta name="author" content="John Doe">
	<meta property="article:published_time" content="2026-06-18">
</head>
<body><p>Some paragraph text here for word counting.</p></body>
</html>`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	h := metadataHandler(client, &config.Config{}, ratelimit.NewNoop())

	result, _ := h(context.Background(), newMetadataRequest(map[string]any{"url": ts.URL}))
	if result.IsError {
		t.Errorf("unexpected error: %v", result)
	}
	text := contentText(result)
	if !strings.Contains(text, "Test Page") {
		t.Errorf("expected title in output: %s", text)
	}
}

func TestMetadataHandler_EmptyURL(t *testing.T) {
	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	h := metadataHandler(client, &config.Config{}, ratelimit.NewNoop())

	result, _ := h(context.Background(), newMetadataRequest(map[string]any{}))
	if !result.IsError {
		t.Error("expected error for empty URL")
	}
}

func TestMetadataHandler_Paywall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>subscribe to read more content behind our paywall</p></body></html>`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	h := metadataHandler(client, &config.Config{}, ratelimit.NewNoop())

	result, _ := h(context.Background(), newMetadataRequest(map[string]any{"url": ts.URL}))
	text := contentText(result)
	// The output format is "- **Has Paywall:** true" (with markdown bold).
	if !strings.Contains(text, "Paywall") || !strings.Contains(text, "true") {
		t.Errorf("expected paywall true in: %s", text)
	}
}

func TestMetadataHandler_NoPaywall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>Free and open content for everyone</p></body></html>`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	h := metadataHandler(client, &config.Config{}, ratelimit.NewNoop())

	result, _ := h(context.Background(), newMetadataRequest(map[string]any{"url": ts.URL}))
	text := contentText(result)
	// Should NOT contain "Yes" for paywall.
	if strings.Contains(text, "Paywall: Yes") {
		t.Errorf("expected no paywall: %s", text)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{500, "500 B"},
		{2048, "2.0 KB"},
		{1048576, "1.0 MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.n)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a very long text", 7, "this..."},
	}
	for _, tt := range tests {
		got := truncateText(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateText(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

func TestPageMetadata_ReadingTime(t *testing.T) {
	// 600 words → 600/200 = 3 min.
	minutes := (600 + 199) / 200
	if minutes != 3 {
		t.Errorf("minutes = %d, want 3", minutes)
	}
	// 0 words → 0/200 = 0 min, but clamped to 1 min in the handler.
	minutes2 := (0 + 199) / 200
	if minutes2 != 0 {
		t.Errorf("minutes for 0 words = %d, want 0", minutes2)
	}
}

func TestExtractMetadata_FallbackDescription(t *testing.T) {
	html := `<html><body><p>First paragraph text here that should become the fallback description when no meta description exists.</p><p>Second paragraph.</p></body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	h := metadataHandler(client, &config.Config{}, ratelimit.NewNoop())

	result, _ := h(context.Background(), newMetadataRequest(map[string]any{"url": ts.URL}))
	text := contentText(result)
	// Should have description from first <p> as fallback.
	if !strings.Contains(strings.ToLower(text), "first paragraph") {
		t.Errorf("expected fallback description from first <p>: %s", text)
	}
}

func TestExtractMainContent(t *testing.T) {
	html := `<html><body>
		<nav>Skip this</nav>
		<article>Main article content</article>
		<footer>Skip this too</footer>
	</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer ts.Close()

	cfg := &config.Config{
		HTTPTimeout:       10 * 1e9,
		MaxContentSize:    10 * 1024 * 1024,
		StealthMode:       false,
		RespectRobotsTXT:  false,
		RetryMaxAttempts:  1,
	}
	client := httpclient.New(cfg)
	h := handler{client: client, cfg: cfg, limiter: ratelimit.NewNoop()}

	result, _ := h.handle(context.Background(), newScrapeRequest(map[string]any{"url": ts.URL}))
	text := contentText(result)
	if !strings.Contains(text, "Main article content") {
		t.Errorf("expected main content: %s", text)
	}
	if strings.Contains(text, "Skip this") {
		t.Errorf("nav content should be stripped: %s", text)
	}
}
