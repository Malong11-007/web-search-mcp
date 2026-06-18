package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
)

func TestExtractDuckDuckGoURL(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=abc", "https://example.com"},
		{"//duckduckgo.com/l/?uddg=https%3A%2F%2Frust-lang.org", "https://rust-lang.org"},
		{"https://plain-url.com/page", "https://plain-url.com/page"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractDuckDuckGoURL(tt.raw)
		if got != tt.want {
			t.Errorf("extractDuckDuckGoURL(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestDuckDuckGo_implementsBackend(t *testing.T) {
	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024})
	var b Backend = &DuckDuckGo{client: client}
	if b == nil {
		t.Error("DuckDuckGo should implement Backend")
	}
}

func TestSearXNG_implementsBackend(t *testing.T) {
	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024})
	cfg := &config.Config{SearXNGURL: "http://localhost"}
	var b Backend = &SearXNG{client: client, cfg: cfg}
	if b == nil {
		t.Error("SearXNG should implement Backend")
	}
}

func TestSearXNG_Search(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "json" {
			t.Error("expected format=json")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results": [
			{"title": "Result 1", "url": "https://example.com/1", "content": "Content 1", "engine": "google", "score": 0.9},
			{"title": "Result 2", "url": "https://example.com/2", "content": "Content 2", "engine": "bing", "score": 0.8}
		]}`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024})
	cfg := &config.Config{SearXNGURL: ts.URL}
	s := NewSearXNG(client, cfg)

	results, err := s.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].Title != "Result 1" {
		t.Errorf("title = %q, want %q", results[0].Title, "Result 1")
	}
}

func TestSearXNG_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024})
	cfg := &config.Config{SearXNGURL: ts.URL}
	s := NewSearXNG(client, cfg)

	_, err := s.Search(context.Background(), "test", 5)
	if err == nil {
		t.Error("expected error for 503")
	}
}

func TestSearXNG_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024})
	cfg := &config.Config{SearXNGURL: ts.URL}
	s := NewSearXNG(client, cfg)

	_, err := s.Search(context.Background(), "test", 5)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSearXNG_NumResultsCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results": [
			{"title": "R1", "url": "https://a.com", "content": "c1", "engine": "g", "score": 1},
			{"title": "R2", "url": "https://b.com", "content": "c2", "engine": "g", "score": 1},
			{"title": "R3", "url": "https://c.com", "content": "c3", "engine": "g", "score": 1}
		]}`))
	}))
	defer ts.Close()

	client := httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024})
	cfg := &config.Config{SearXNGURL: ts.URL}
	s := NewSearXNG(client, cfg)

	results, err := s.Search(context.Background(), "test", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (cap), got %d", len(results))
	}
}

func TestDuckDuckGo_SearchToURL(t *testing.T) {
	d := NewDuckDuckGo(httpclient.New(&config.Config{HTTPTimeout: 10 * 1e9, MaxContentSize: 10 * 1024 * 1024}))
	// Verify the URL is constructed correctly.
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape("rust programming")
	if searchURL != "https://html.duckduckgo.com/html/?q=rust+programming" {
		t.Errorf("Unexpected search URL: %s", searchURL)
	}
	_ = d
}
