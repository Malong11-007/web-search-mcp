package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/dan/web-search-mcp/internal/config"
	"github.com/dan/web-search-mcp/internal/httpclient"
	"github.com/dan/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterBatchTool adds the batch_scrape tool.
func RegisterBatchTool(s *server.MCPServer, cfg *config.Config) {
	client := httpclient.New(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	tool := mcp.NewTool("batch_scrape",
		mcp.WithDescription(
			"Scrape up to 20 URLs in parallel. Best for: reading multiple search results after a search, "+
				"comparing content across several pages, or bulk-extracting known URLs. "+
				"Each URL is fetched independently and rate-limited — all return together when the slowest completes. "+
				"NOT for: a single URL (use scrape_page — it's simpler), discovering URLs (use site_map), "+
				"or deep crawling (use crawl with depth/max_pages). "+
				"Returns each URL's content under its own heading, truncated to 5000 chars per page in the output. "+
				"Tip: use page_metadata first on unknown URLs to check for paywalls or very long pages before committing to a batch scrape.",
		),
		mcp.WithArray("urls",
			mcp.Required(),
			mcp.Description("List of full URLs to scrape (max 20). All must be valid, reachable URLs with https://."),
		),
		mcp.WithString("format",
			mcp.Description("Output format for all pages. \"markdown\" gives formatted content, \"text\" is plain text, \"json\" is structured."),
			mcp.Enum("markdown", "json", "text"),
		),
	)

	s.AddTool(tool, batchHandler(client, cfg, limiter))
}

type batchResult struct {
	URL     string `json:"url"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

func batchHandler(client *httpclient.Client, cfg *config.Config, limiter *ratelimit.Limiter) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		urls := parseStringArray(request, "urls")
		if len(urls) == 0 {
			return mcp.NewToolResultError("urls is required (array of strings)"), nil
		}
		if len(urls) > 20 {
			urls = urls[:20]
		}

		format := "markdown"
		if f, ok := request.Params.Arguments["format"].(string); ok && f != "" {
			format = f
		}

		var (
			mu      sync.Mutex
			results []batchResult
			wg      sync.WaitGroup
		)

		for _, u := range urls {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()

				if err := limiter.Wait(ctx); err != nil {
					mu.Lock()
					results = append(results, batchResult{URL: url, Error: err.Error()})
					mu.Unlock()
					return
				}

				content, err := scrapeOne(ctx, client, cfg, url, format)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					results = append(results, batchResult{URL: url, Error: err.Error()})
				} else {
					results = append(results, batchResult{URL: url, Content: content})
				}
			}(u)
		}
		wg.Wait()

		// Return as formatted text.
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Batch scrape: %d URLs\n\n", len(urls)))
		for _, r := range results {
			sb.WriteString(fmt.Sprintf("## %s\n", r.URL))
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf("Error: %s\n\n", r.Error))
			} else {
				sb.WriteString(truncate(r.Content, 5000))
				sb.WriteString("\n\n")
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ensure we don't conflict with the function in batch.go
var _ = time.Now // avoid unused import if needed

// parseStringArray extracts a string array argument from the request.
func parseStringArray(request mcp.CallToolRequest, key string) []string {
	raw, ok := request.Params.Arguments[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// scrapeOne fetches and formats a single URL's content.
func scrapeOne(ctx context.Context, client *httpclient.Client, cfg *config.Config, pageURL, format string) (string, error) {
	body, err := client.GetBody(pageURL)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}

	contentSel := extractMainContent(doc)
	contentHTML, err := contentSel.Html()
	if err != nil {
		return "", fmt.Errorf("extract content: %w", err)
	}

	var result string
	switch format {
	case "json":
		result, err = formatJSON(contentHTML, contentSel)
	case "text":
		result = contentSel.Text()
	case "markdown":
		fallthrough
	default:
		result, err = formatMarkdown(contentHTML, pageURL)
	}
	if err != nil {
		return "", err
	}
	if result == "" {
		result = "(empty page)"
	}
	return result, nil
}

// Ensure json is used.
var _ = json.Marshal
