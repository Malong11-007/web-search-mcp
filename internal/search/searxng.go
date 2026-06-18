package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
)

// SearXNG searches via a SearXNG instance's JSON API.
type SearXNG struct {
	client *httpclient.Client
	cfg    *config.Config
}

// NewSearXNG creates a SearXNG backend.
func NewSearXNG(client *httpclient.Client, cfg *config.Config) *SearXNG {
	return &SearXNG{client: client, cfg: cfg}
}

type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

type searxngResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Engine  string  `json:"engine"`
	Score   float64 `json:"score"`
}

// Search executes a search against the configured SearXNG instance.
func (s *SearXNG) Search(ctx context.Context, query string, numResults int) ([]Result, error) {
	searchURL := fmt.Sprintf("%s/search?format=json&q=%s",
		s.cfg.SearXNGURL,
		url.QueryEscape(query),
	)

	// Use simple GET — search APIs don't need browser emulation headers.
	resp, err := s.client.GetSimple(searchURL)
	if err != nil {
		return nil, fmt.Errorf("searxng: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng: unexpected status %d", resp.StatusCode)
	}

	var sr searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("searxng: decode response: %w", err)
	}

	results := make([]Result, 0, len(sr.Results))
	for i, r := range sr.Results {
		if i >= numResults {
			break
		}
		results = append(results, Result{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Content,
			Engine:      r.Engine,
		})
	}

	return results, nil
}

// Ensure SearXNG implements Backend.
var _ Backend = (*SearXNG)(nil)
