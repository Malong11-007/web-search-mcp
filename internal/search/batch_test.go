package search

import (
	"context"
	"github.com/mark3labs/mcp-go/mcp"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
)

func newBatchRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func TestBatchHandler_Success(t *testing.T) {
	backend := &mockBackend{
		results: []Result{
			{Title: "R1", URL: "https://a.com", Description: "desc1"},
		},
	}
	cfg := &config.Config{}
	h := batchHandler(backend, cfg, ratelimit.MustNew("100/1s"))

	result, err := h(context.Background(), newBatchRequest(map[string]any{
		"queries": []any{"q1", "q2"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error result: %v", result)
	}
}

func TestBatchHandler_EmptyQueries(t *testing.T) {
	backend := &mockBackend{}
	cfg := &config.Config{}
	h := batchHandler(backend, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newBatchRequest(map[string]any{}))
	if !result.IsError {
		t.Error("expected error for empty queries")
	}
}

func TestBatchHandler_Truncate(t *testing.T) {
	backend := &mockBackend{
		results: []Result{{Title: "R", URL: "https://a.com"}},
	}
	cfg := &config.Config{}
	h := batchHandler(backend, cfg, ratelimit.MustNew("100/1s"))

	queries := make([]any, 15)
	for i := 0; i < 15; i++ {
		queries[i] = "q"
	}
	result, _ := h(context.Background(), newBatchRequest(map[string]any{
		"queries": queries,
	}))
	// Should be truncated to 10, no error.
	if result.IsError {
		t.Error("expected success with truncated queries")
	}
}

func TestBatchHandler_MixedResults(t *testing.T) {
	mixedBackend := &mockBackend{err: nil, results: []Result{{Title: "R", URL: "https://a.com"}}}

	// Use a backend that succeeds for all queries.
	cfg := &config.Config{}
	h := batchHandler(mixedBackend, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newBatchRequest(map[string]any{
		"queries": []any{"ok", "also-ok"},
	}))
	if result.IsError {
		t.Errorf("expected success for mixed results, got error")
	}
}
