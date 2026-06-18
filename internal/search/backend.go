package search

import "context"

// Result represents a single search result from any backend.
type Result struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Engine      string `json:"engine,omitempty"`
}

// Backend performs web searches.
type Backend interface {
	Search(ctx context.Context, query string, numResults int) ([]Result, error)
}
