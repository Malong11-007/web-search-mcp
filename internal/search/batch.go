package search

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterBatchTool adds the batch_search tool.
func RegisterBatchTool(s *server.MCPServer, cfg *config.Config) {
	backend := buildBackend(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	tool := mcp.NewTool("batch_search",
		mcp.WithDescription(
			"Run up to 10 web searches in parallel. Best for: fact-checking a claim across multiple sources, "+
				"comparing perspectives on a topic, searching for several related terms simultaneously, "+
				"or when the user asks about multiple distinct things at once. "+
				"NOT for: a single query (use web_search — it's faster and returns more results per query), "+
				"or deep reading of results (follow with batch_scrape on the returned URLs). "+
				"Returns results grouped under each query heading, with 5 results per query by default.",
		),
		mcp.WithArray("queries",
			mcp.Required(),
			mcp.Description("List of search query strings to run in parallel (max 10). Each query runs independently and may hit different backends."),
			mcp.Items(map[string]any{"type": "string"}),
			mcp.MaxItems(10),
		),
		mcp.WithNumber("num_results",
			mcp.Description("Results per query (default 5, max 10). Keep low for broad comparison, raise if each query needs deeper coverage."),
		),
	)

	s.AddTool(tool, batchHandler(backend, cfg, limiter))
}

func batchHandler(backend Backend, cfg *config.Config, limiter *ratelimit.Limiter) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		queries := parseStringArray(request, "queries")
		if len(queries) == 0 {
			return mcp.NewToolResultError("queries is required (array of strings)"), nil
		}
		if len(queries) > 10 {
			queries = queries[:10]
		}

		numResults := 5
		if n, ok := request.Params.Arguments["num_results"].(float64); ok {
			numResults = int(n)
			if numResults < 1 {
				numResults = 1
			}
			if numResults > 10 {
				numResults = 10
			}
		}

		type queryResult struct {
			Query   string   `json:"query"`
			Results []Result `json:"results"`
			Error   string   `json:"error,omitempty"`
		}

		var (
			mu      sync.Mutex
			results []queryResult
			wg      sync.WaitGroup
		)

		for _, q := range queries {
			wg.Add(1)
			go func(query string) {
				defer wg.Done()

				if err := limiter.Wait(ctx); err != nil {
					mu.Lock()
					results = append(results, queryResult{Query: query, Error: err.Error()})
					mu.Unlock()
					return
				}

				r, err := backend.Search(ctx, query, numResults)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					results = append(results, queryResult{Query: query, Error: err.Error()})
				} else {
					results = append(results, queryResult{Query: query, Results: r})
				}
			}(q)
		}
		wg.Wait()

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Batch search: %d queries\n\n", len(queries)))
		for _, r := range results {
			sb.WriteString(fmt.Sprintf("## %s\n", r.Query))
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf("Error: %s\n\n", r.Error))
				continue
			}
			for i, res := range r.Results {
				sb.WriteString(fmt.Sprintf("%d. **%s**\n   %s\n   %s\n",
					i+1, res.Title, res.URL, truncate(res.Description, 200)))
			}
			sb.WriteString("\n")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
