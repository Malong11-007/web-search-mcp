package research

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/pii"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/Malong11-007/web-search-mcp/internal/search"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTool adds the deep_research tool.
func RegisterTool(s *server.MCPServer, cfg *config.Config) {
	client := httpclient.New(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	// Build search backend.
	var backends []search.Backend
	for _, name := range cfg.SearchBackends {
		switch strings.ToLower(name) {
		case "duckduckgo":
			backends = append(backends, search.NewDuckDuckGo(client))
		case "searxng":
			if cfg.SearXNGURL != "" {
				backends = append(backends, search.NewSearXNG(client, cfg))
			}
		}
	}
	if len(backends) == 0 {
		backends = append(backends, search.NewDuckDuckGo(client))
	}
	var backend search.Backend
	if len(backends) == 1 {
		backend = backends[0]
	} else {
		backend = search.NewFallback(backends...)
	}

	tool := mcp.NewTool("deep_research",
		mcp.WithDescription(
			"Research a topic end-to-end: searches the web, scrapes the top results, and returns full content from each source in a synthesized report. "+
				"Best for: \"research X and tell me what you find\", \"what's the current thinking on Y\", "+
				"questions requiring cross-referencing multiple sources, or when the user asks for a comprehensive answer with sources. "+
				"This does search + scrape in one call — it's the highest-signal tool for research questions. "+
				"Each source is fetched independently and content is truncated to 10,000 chars per source. "+
				"⚠️ Takes longer than web_search (multiple HTTP fetches) — use web_search for quick lookups. "+
				"NOT for: a known URL (use scrape_page directly), comparing specific pages (use batch_scrape), "+
				"or questions answerable from a single source (use web_search + scrape_page for more control). "+
				"Tip: start with num_sources=3, increase if the report is too shallow.",
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The research question or topic. Phrase it as you would ask a researcher — detailed questions get better search results."),
		),
		mcp.WithNumber("num_sources",
			mcp.Description("How many search results to scrape and include (default 3, max 10). More sources = broader coverage but slower. 3-5 is usually sufficient."),
		),
	)

	s.AddTool(tool, handler(backend, client, cfg, limiter))
}

type sourceResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func handler(backend search.Backend, client *httpclient.Client, cfg *config.Config, limiter *ratelimit.Limiter) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(request, "query", "")
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}

		numSources := 3
		if n, ok := request.Params.Arguments["num_sources"].(float64); ok {
			numSources = int(n)
			if numSources < 1 {
				numSources = 1
			}
			if numSources > 10 {
				numSources = 10
			}
		}

		// Step 1: Search.
		if err := limiter.Wait(ctx); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		results, err := backend.Search(ctx, query, numSources)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No results found for: %s", query)), nil
		}

		// Step 2: Scrape each result.
		converter := md.NewConverter("", true, nil)
		var (
			mu      sync.Mutex
			sources []sourceResult
			wg      sync.WaitGroup
		)

		for _, r := range results {
			wg.Add(1)
			go func(res search.Result) {
				defer wg.Done()

				if err := limiter.Wait(ctx); err != nil {
					return
				}

				src := sourceResult{
					Title: res.Title,
					URL:   res.URL,
				}

				body, err := client.GetBody(res.URL)
				if err != nil {
					src.Error = err.Error()
					mu.Lock()
					sources = append(sources, src)
					mu.Unlock()
					return
				}

				markdown, err := converter.ConvertString(string(body))
				if err != nil {
					src.Error = err.Error()
					mu.Lock()
					sources = append(sources, src)
					mu.Unlock()
					return
				}

				// Trim to a reasonable size per source.
				if len(markdown) > 10000 {
					markdown = markdown[:10000] + "\n\n[truncated]"
				}
				if cfg.RedactPII {
					markdown = pii.Redact(markdown)
				}

				src.Content = markdown
				mu.Lock()
				sources = append(sources, src)
				mu.Unlock()
			}(r)
		}
		wg.Wait()

		// Step 3: Build synthesized report.
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Research: %s\n\n", query))
		sb.WriteString(fmt.Sprintf("Searched for: \"%s\"\n", query))
		sb.WriteString(fmt.Sprintf("Found %d results, scraped %d sources\n\n", len(results), len(sources)))
		sb.WriteString("---\n\n")

		for i, src := range sources {
			sb.WriteString(fmt.Sprintf("## Source %d: %s\n", i+1, src.Title))
			sb.WriteString(fmt.Sprintf("URL: %s\n\n", src.URL))
			if src.Error != "" {
				sb.WriteString(fmt.Sprintf("> Error: %s\n\n", src.Error))
			} else {
				sb.WriteString(src.Content)
				sb.WriteString("\n\n")
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}
