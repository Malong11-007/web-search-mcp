package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/crawl"
	sitemap "github.com/Malong11-007/web-search-mcp/internal/map"
	"github.com/Malong11-007/web-search-mcp/internal/research"
	"github.com/Malong11-007/web-search-mcp/internal/scrape"
	"github.com/Malong11-007/web-search-mcp/internal/search"
	"github.com/Malong11-007/web-search-mcp/internal/version"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// osExit is a test hook that can be overridden in tests.
var osExit = os.Exit

func main() {
	// Handle --version / -v before starting the server.
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.String())
		osExit(0)
	}

	cfg := config.Load()

	s := server.NewMCPServer(
		"web-search-mcp",
		version.Short(),
		server.WithToolCapabilities(true),
	)

	// Register tools based on the TOOLS env var.
	registerTools(s, cfg)

	// Add a search_feedback tool if search is enabled.
	if cfg.HasTool("search") {
		registerFeedbackTool(s)
	}

	switch cfg.Transport {
	case "http", "sse":
		runHTTP(s, cfg)
	default:
		runStdio(s)
	}
}

func registerTools(s *server.MCPServer, cfg *config.Config) {
	if cfg.HasTool("search") {
		search.RegisterTool(s, cfg)
		search.RegisterBatchTool(s, cfg)
	}
	if cfg.HasTool("scrape") {
		scrape.RegisterTool(s, cfg)
		scrape.RegisterBatchTool(s, cfg)
		scrape.RegisterMetadataTool(s, cfg)
	}
	if cfg.HasTool("crawl") {
		crawl.RegisterTool(s, cfg)
	}
	if cfg.HasTool("map") {
		sitemap.RegisterTool(s, cfg)
	}
	if cfg.HasTool("research") {
		research.RegisterTool(s, cfg)
	}
}

func registerFeedbackTool(s *server.MCPServer) {
	tool := mcp.NewTool("search_feedback",
		mcp.WithDescription(
			"Submit feedback on a search result's relevance. This helps tune future search rankings. "+
				"Use after web_search or deep_research when a result was particularly helpful or irrelevant. "+
				"Ratings: \"relevant\" = exactly what was needed, \"somewhat_relevant\" = related but not quite right, "+
				"\"not_relevant\" = unrelated to the query, \"spam\" = low-quality or spam content.",
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The original search query that produced this result."),
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL of the search result you're rating."),
		),
		mcp.WithString("rating",
			mcp.Required(),
			mcp.Description("Relevance rating for this result."),
			mcp.Enum("relevant", "somewhat_relevant", "not_relevant", "spam"),
		),
		mcp.WithString("comment",
			mcp.Description("Optional: any additional context about why this result was or wasn't helpful."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := mcp.ParseString(request, "query", "")
		ratedURL := mcp.ParseString(request, "url", "")
		rating := mcp.ParseString(request, "rating", "")
		comment := mcp.ParseString(request, "comment", "")

		if query == "" || ratedURL == "" || rating == "" {
			return mcp.NewToolResultError("query, url, and rating are required"), nil
		}

		// In the future, this could feed into a learning-to-rank system.
		// For now, log and acknowledge.
		log.Printf("[feedback] query=%q url=%q rating=%s comment=%q", query, ratedURL, rating, comment)
		return mcp.NewToolResultText(fmt.Sprintf("Thanks! Feedback recorded: %s rated as '%s'", ratedURL, rating)), nil
	})
}

func runStdio(s *server.MCPServer) {
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		osExit(1)
	}
}

func runHTTP(s *server.MCPServer, cfg *config.Config) {
	addr := fmt.Sprintf(":%d", cfg.HTTPPort)

	sseServer := server.NewSSEServer(s)

	mux := http.NewServeMux()
	mux.Handle("/sse", sseServer)
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		srv.Shutdown(context.Background())
	}()

	log.Printf("MCP server listening on http://localhost%s/sse", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		osExit(1)
	}
}
