package research

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/Malong11-007/web-search-mcp/internal/search"
	"github.com/mark3labs/mcp-go/mcp"
)

type mockBackend struct {
	results []search.Result
	err     error
}

func (m *mockBackend) Search(ctx context.Context, query string, numResults int) ([]search.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func newResearchRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func researchContentText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if tc, ok := result.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func TestResearch_SearchFailure(t *testing.T) {
	backend := &mockBackend{err: errors.New("search down")}
	cfg := &config.Config{MaxContentSize: 10 * 1024}
	client := httpclient.New(cfg)
	h := handler(backend, client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newResearchRequest(map[string]any{"query": "test"}))
	if !result.IsError {
		t.Error("expected error when search fails")
	}
	text := researchContentText(result)
	if !strings.Contains(text, "search failed") {
		t.Errorf("expected search failure message: %s", text)
	}
}

func TestResearch_EmptyResults(t *testing.T) {
	backend := &mockBackend{results: []search.Result{}}
	cfg := &config.Config{MaxContentSize: 10 * 1024}
	client := httpclient.New(cfg)
	h := handler(backend, client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newResearchRequest(map[string]any{"query": "obscure term"}))
	if result.IsError {
		t.Error("expected success but no results message")
	}
	text := researchContentText(result)
	if !strings.Contains(text, "No results found") {
		t.Errorf("expected 'No results found': %s", text)
	}
}

func TestResearch_EmptyQuery(t *testing.T) {
	backend := &mockBackend{}
	cfg := &config.Config{}
	client := httpclient.New(cfg)
	h := handler(backend, client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newResearchRequest(map[string]any{}))
	if !result.IsError {
		t.Error("expected error for missing query")
	}
}

func TestResearch_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><article>Scraped research content here.</article></body></html>`))
	}))
	defer ts.Close()

	backend := &mockBackend{
		results: []search.Result{
			{Title: "Source 1", URL: ts.URL + "/page1"},
		},
	}
	cfg := &config.Config{
		MaxContentSize:    10 * 1024,
		HTTPTimeout:       10 * 1e9,
		RespectRobotsTXT:  false,
		StealthMode:       false,
	}
	client := httpclient.New(cfg)
	h := handler(backend, client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newResearchRequest(map[string]any{
		"query":       "test topic",
		"num_sources": 1,
	}))
	text := researchContentText(result)
	if !strings.Contains(text, "Scraped research content") {
		t.Errorf("expected scraped content in output: %s", text)
	}
}

func TestResearch_NumSourcesBounds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>content</p></body></html>`))
	}))
	defer ts.Close()

	backend := &mockBackend{
		results: []search.Result{
			{Title: "S1", URL: ts.URL + "/1"},
			{Title: "S2", URL: ts.URL + "/2"},
		},
	}
	cfg := &config.Config{
		MaxContentSize:    10 * 1024,
		HTTPTimeout:       10 * 1e9,
		RespectRobotsTXT:  false,
		StealthMode:       false,
	}
	client := httpclient.New(cfg)
	h := handler(backend, client, cfg, ratelimit.MustNew("100/1s"))

	// num_sources=0 should be clamped to 1.
	result, _ := h(context.Background(), newResearchRequest(map[string]any{
		"query":       "test",
		"num_sources": float64(0),
	}))
	text := researchContentText(result)
	if !strings.Contains(text, "S1") {
		t.Errorf("expected at least source S1 when num_sources=0 (clamped): %s", text)
	}
}
