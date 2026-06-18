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

func newBatchScrapeRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func batchContentText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if tc, ok := result.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func TestBatchScrape_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><article>Page content</article></body></html>`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	cfg := &config.Config{RespectRobotsTXT: false}
	h := batchHandler(client, cfg, ratelimit.MustNew("100/1s"))

	result, err := h(context.Background(), newBatchScrapeRequest(map[string]any{
		"urls": []any{ts.URL + "/1", ts.URL + "/2"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := batchContentText(result)
	if !strings.Contains(text, "Page content") {
		t.Errorf("expected content in output: %s", text)
	}
}

func TestBatchScrape_EmptyURLs(t *testing.T) {
	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	cfg := &config.Config{}
	h := batchHandler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newBatchScrapeRequest(map[string]any{}))
	if !result.IsError {
		t.Error("expected error for empty urls")
	}
}

func TestBatchScrape_Truncate20(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>ok</p></body></html>`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	cfg := &config.Config{RespectRobotsTXT: false}
	h := batchHandler(client, cfg, ratelimit.MustNew("100/1s"))

	urls := make([]any, 25)
	for i := 0; i < 25; i++ {
		urls[i] = ts.URL + "/p"
	}
	result, _ := h(context.Background(), newBatchScrapeRequest(map[string]any{"urls": urls}))
	text := batchContentText(result)
	// Should only process 20 URLs.
	if strings.Count(text, "ok") > 20 {
		t.Errorf("expected at most 20 results, got count: %d", strings.Count(text, "ok"))
	}
}

func TestBatchScrape_FormatText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>plain text content</p></body></html>`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024, StealthMode: false})
	cfg := &config.Config{RespectRobotsTXT: false}
	h := batchHandler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newBatchScrapeRequest(map[string]any{
		"urls":   []any{ts.URL + "/page"},
		"format": "text",
	}))
	text := batchContentText(result)
	if !strings.Contains(text, "plain text content") {
		t.Errorf("expected plain text content: %s", text)
	}
}

func TestParseStringArray(t *testing.T) {
	req := newBatchScrapeRequest(map[string]any{
		"urls": []any{"a", "b", 123, "c"},
	})
	result := parseStringArray(req, "urls")
	if len(result) != 3 {
		t.Errorf("expected 3 strings, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestParseStringArray_Empty(t *testing.T) {
	req := newBatchScrapeRequest(map[string]any{})
	result := parseStringArray(req, "urls")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}
