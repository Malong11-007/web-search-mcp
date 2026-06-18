package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/Malong11-007/web-search-mcp/internal/retry"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// buildBackend creates a search backend chain from config.
func buildBackend(cfg *config.Config) Backend {
	client := httpclient.New(cfg)
	var backends []Backend
	for _, name := range cfg.SearchBackends {
		switch strings.ToLower(name) {
		case "duckduckgo":
			backends = append(backends, NewDuckDuckGo(client))
		case "searxng":
			if cfg.SearXNGURL != "" {
				backends = append(backends, NewSearXNG(client, cfg))
			}
		}
	}
	if len(backends) == 0 {
		backends = append(backends, NewDuckDuckGo(client))
	}
	if len(backends) == 1 {
		return backends[0]
	}
	return NewFallback(backends...)
}

// RegisterTool adds the web_search tool and its handler to the MCP server.
func RegisterTool(s *server.MCPServer, cfg *config.Config) {
	backend := buildBackend(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	h := &handler{
		backend: backend,
		cfg:     cfg,
		limiter: limiter,
	}

	tool := mcp.NewTool("web_search",
		mcp.WithDescription(
			"Search the web for a single query. Uses DuckDuckGo and SearXNG with automatic failover. "+
				"Best for: quick fact-checking, looking up a specific topic, finding a known site, answering \"what is X\" questions. "+
				"NOT for: multi-source deep research (use deep_research), multiple unrelated queries at once (use batch_search), "+
				"or reading full page content (use scrape_page after getting URLs). "+
				"Returns a numbered list with title, URL, and description snippet for each result.",
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query. Be specific — use keywords, phrases, or a natural-language question."),
		),
		mcp.WithNumber("num_results",
			mcp.Description("Number of results to return (default 10, max 25). Higher values may surface more diverse sources but take longer to scan."),
		),
	)

	s.AddTool(tool, h.handle)
}

type handler struct {
	backend Backend
	cfg     *config.Config
	limiter *ratelimit.Limiter
}

func (h *handler) handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := mcp.ParseString(request, "query", "")
	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	numResults := 10
	if n, ok := request.Params.Arguments["num_results"].(float64); ok {
		numResults = int(n)
		if numResults < 1 {
			numResults = 1
		}
		if numResults > 25 {
			numResults = 25
		}
	}

	// Rate limit before making the request.
	if err := h.limiter.Wait(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rate limiter: %v", err)), nil
	}

	var results []Result
	err := retry.Do(ctx,
		h.cfg.RetryMaxAttempts,
		h.cfg.RetryInitialDelay,
		h.cfg.RetryMaxDelay,
		h.cfg.RetryBackoffFactor,
		func() error {
			var err error
			results, err = h.backend.Search(ctx, query, numResults)
			return err
		},
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No results found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %s\n\n", query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		if r.Description != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Description))
		}
		sb.WriteString("\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}
