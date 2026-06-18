package search

import (
	"context"
	"errors"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
)

// mockBackend implements Backend for testing.
type mockBackend struct {
	results []Result
	err     error
}

func (m *mockBackend) Search(_ context.Context, _ string, _ int) ([]Result, error) {
	return m.results, m.err
}

func newSearchRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func TestHandler_Success(t *testing.T) {
	h := &handler{
		backend: &mockBackend{
			results: []Result{
				{Title: "Test", URL: "https://example.com", Description: "A test result"},
			},
		},
		cfg:     config.Load(),
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newSearchRequest(map[string]any{"query": "test"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestHandler_MissingQuery(t *testing.T) {
	h := &handler{
		backend: &mockBackend{},
		cfg:     config.Load(),
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newSearchRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing query")
	}
}

func TestHandler_BackendError(t *testing.T) {
	h := &handler{
		backend: &mockBackend{err: errors.New("backend down")},
		cfg:     config.Load(),
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newSearchRequest(map[string]any{"query": "test"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for backend failure")
	}
}

func TestHandler_NoResults(t *testing.T) {
	h := &handler{
		backend: &mockBackend{results: []Result{}},
		cfg:     config.Load(),
		limiter: ratelimit.NewNoop(),
	}

	result, err := h.handle(context.Background(), newSearchRequest(map[string]any{"query": "noresults"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %v", result)
	}
}
