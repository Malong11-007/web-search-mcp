package search

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dan/web-search-mcp/internal/httpclient"
)

// DuckDuckGo searches via DuckDuckGo's HTML interface.
type DuckDuckGo struct {
	client *httpclient.Client
}

// NewDuckDuckGo creates a DuckDuckGo backend.
func NewDuckDuckGo(client *httpclient.Client) *DuckDuckGo {
	return &DuckDuckGo{client: client}
}

// Search executes a search against DuckDuckGo's HTML endpoint.
func (d *DuckDuckGo) Search(ctx context.Context, query string, numResults int) ([]Result, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s",
		url.QueryEscape(query),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: build request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo: unexpected status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: parse HTML: %w", err)
	}

	var results []Result
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= numResults {
			return
		}

		title := strings.TrimSpace(s.Find(".result__a").First().Text())
		rawURL, _ := s.Find(".result__a").First().Attr("href")
		description := strings.TrimSpace(s.Find(".result__snippet").First().Text())

		if title == "" || rawURL == "" {
			return
		}

		// DuckDuckGo wraps URLs in its own redirect; extract the real URL.
		cleanURL := extractDuckDuckGoURL(rawURL)

		results = append(results, Result{
			Title:       title,
			URL:         cleanURL,
			Description: description,
			Engine:      "duckduckgo",
		})
	})

	return results, nil
}

// extractDuckDuckGoURL extracts the target URL from DuckDuckGo's redirect wrapper.
// DuckDuckGo HTML results use the form: //duckduckgo.com/l/?uddg=<encoded_url>&...
func extractDuckDuckGoURL(raw string) string {
	// The href looks like "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&..."
	const prefix = "uddg="
	idx := strings.Index(raw, prefix)
	if idx < 0 {
		return raw
	}

	uddg := raw[idx+len(prefix):]
	// Cut at the next &
	if ampIdx := strings.Index(uddg, "&"); ampIdx > 0 {
		uddg = uddg[:ampIdx]
	}

	decoded, err := url.QueryUnescape(uddg)
	if err != nil {
		return raw
	}
	return decoded
}

// Ensure DuckDuckGo implements Backend.
var _ Backend = (*DuckDuckGo)(nil)
