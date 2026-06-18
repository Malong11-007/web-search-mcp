package scrape

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
)

func newScrapeRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func newTestClient(cfg *config.Config) *httpclient.Client {
	return httpclient.New(cfg)
}

func TestHandler_ScrapePage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<nav>Skip me</nav>
<article>
<h1>Hello World</h1>
<p>This is <strong>test</strong> content.</p>
</article>
<footer>Skip me too</footer>
</body>
</html>`))
	}))
	defer ts.Close()

	cfg := config.Load()
	h := &handler{
		client:  newTestClient(cfg),
		cfg:     cfg,
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newScrapeRequest(map[string]any{
		"url":    ts.URL,
		"format": "markdown",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}

	text := contentText(result)
	if !strings.Contains(text, "Hello World") {
		t.Errorf("expected 'Hello World' in output, got: %s", text)
	}
	if !strings.Contains(text, "test") {
		t.Errorf("expected 'test' in output, got: %s", text)
	}
	if strings.Contains(text, "Skip me") {
		t.Errorf("nav should have been stripped, got: %s", text)
	}
	if strings.Contains(text, "Skip me too") {
		t.Errorf("footer should have been stripped, got: %s", text)
	}
}

func TestHandler_MissingURL(t *testing.T) {
	cfg := config.Load()
	h := &handler{
		client:  newTestClient(cfg),
		cfg:     cfg,
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newScrapeRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing url")
	}
}

func TestHandler_TextFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html><body><p>Plain text</p></body></html>`))
	}))
	defer ts.Close()

	cfg := config.Load()
	h := &handler{
		client:  newTestClient(cfg),
		cfg:     cfg,
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newScrapeRequest(map[string]any{
		"url":    ts.URL,
		"format": "text",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}
	text := contentText(result)
	if !strings.Contains(text, "Plain text") {
		t.Errorf("expected 'Plain text' in output, got: %s", text)
	}
}

func TestHandler_JSONFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html><body><p>JSON test</p></body></html>`))
	}))
	defer ts.Close()

	cfg := config.Load()
	h := &handler{
		client:  newTestClient(cfg),
		cfg:     cfg,
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newScrapeRequest(map[string]any{
		"url":    ts.URL,
		"format": "json",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}
	text := contentText(result)
	if !strings.Contains(text, "JSON test") {
		t.Errorf("expected 'JSON test' in output, got: %s", text)
	}
	if !strings.Contains(text, `"content"`) {
		t.Errorf("expected JSON structure, got: %s", text)
	}
}

func TestHandler_BadURL(t *testing.T) {
	cfg := config.Load()
	h := &handler{
		client:  newTestClient(cfg),
		cfg:     cfg,
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newScrapeRequest(map[string]any{
		"url": "http://127.0.0.1:1/nonexistent",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for bad URL")
	}
}

func contentText(result *mcp.CallToolResult) string {
	if len(result.Content) > 0 {
		if tc, ok := result.Content[0].(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
