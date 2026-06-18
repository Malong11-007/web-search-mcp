# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go MCP (Model Context Protocol) server that provides web search, content extraction, and web scraping tools. Designed to be free, local, unblocked — combining the best features of `brightdata-mcp`, `firecrawl-mcp-server`, and `mcp-server-brave-search`.

## Build & Development Commands

```bash
# Build the server binary
go build -o web-search-mcp ./cmd/web-search-mcp/

# Run the server (stdio transport, for Claude Desktop / MCP clients)
go run ./cmd/web-search-mcp/

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/search/

# Run a single test
go test ./internal/search/ -run TestFunctionName

# Run tests with verbose output and race detection
go test -v -race ./...

# Lint (requires golangci-lint)
golangci-lint run ./...

# Generate mocks (requires mockgen)
go generate ./...
```

## Architecture

### Transport Layer
- The server supports **stdio** (default, for local MCP clients like Claude Desktop) and **HTTP/SSE** (for remote client connections).
- stdio transport is handled by the `mcp-go` library's `server.ServeStdio` or equivalent.
- The main entrypoint is in `cmd/web-search-mcp/main.go` — it wires together all tool registrations and starts the transport.

### Tool Registration Pattern
Each tool group (search, scrape, crawl, etc.) lives in its own package under `internal/` and exposes a registration function:

```go
// internal/search/tool.go
func RegisterTool(s *mcp.Server) {
    s.AddTool(mcp.Tool{...}, handler)
}
```

All tool groups are registered in `cmd/web-search-mcp/main.go` by calling each package's `Register*` function. This keeps tool definitions co-located with their handlers and dependencies.

### Key Packages

| Package | Purpose |
|---------|---------|
| `internal/search` | Web search tool — queries multiple free backends (SearXNG, DuckDuckGo), aggregates results |
| `internal/scrape` | Single-page content extraction to clean Markdown, JSON, or plain text |
| `internal/crawl` | Multi-page crawling with depth and limit controls |
| `internal/map` | Site URL discovery |
| `internal/extract` | Structured data extraction using JSON schemas |
| `internal/httpclient` | Shared HTTP client with retry, rate limiting, and user-agent rotation |
| `internal/retry` | Exponential backoff retry logic (configurable max attempts, initial/max delay, backoff factor) |
| `internal/ratelimit` | Configurable rate limiter (e.g., `100/1h` format) |
| `internal/monitor` | Recurring page monitoring with change detection |
| `internal/config` | Environment variable and CLI flag parsing |

### Data Flow
1. MCP client sends a tool call (e.g., `web_search`) over stdio/HTTP.
2. `mcp-go` server routes to the registered handler.
3. Handler validates inputs, applies rate limiting, and calls the appropriate backend (SearXNG, DuckDuckGo, direct HTTP scrape).
4. Response is cleaned/formatted (main content extraction, HTML-to-Markdown conversion).
5. Result is returned through the MCP response channel.

### Dependencies
- **[mcp-go](https://github.com/mark3labs/mcp-go)** — MCP protocol implementation for Go (server types, tool registration, transport).
- **[goquery](https://github.com/PuerkitoBio/goquery)** — HTML parsing and main content extraction (jQuery-like API).
- **[html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown)** — Converts extracted HTML to clean Markdown.
- **[shurcooL/github_flavored_markdown](https://github.com/shurcooL/github_flavored_markdown)** — Markdown rendering if needed.

### Error Handling Conventions
- All tool handlers return structured errors via `mcp.CallToolResult` with `IsError: true`.
- Transient network errors are retried with exponential backoff before surfacing to the client.
- Timeout errors are surfaced immediately (no retry).
- HTTP 429 responses are retried after the `Retry-After` duration.

### Configuration
All configuration is via environment variables:

| Variable | Purpose | Default |
|----------|---------|---------|
| `SEARXNG_URL` | SearXNG instance URL | `https://searx.be` |
| `SEARXNG_TIMEOUT` | SearXNG request timeout | `10s` |
| `HTTP_TIMEOUT` | General HTTP timeout | `30s` |
| `RATE_LIMIT` | Global rate limit (format: `N/1s`) | `100/1m` |
| `RETRY_MAX_ATTEMPTS` | Max retry attempts | `3` |
| `RETRY_INITIAL_DELAY` | Initial retry delay | `1s` |
| `RETRY_MAX_DELAY` | Max retry delay | `30s` |
| `RETRY_BACKOFF_FACTOR` | Exponential backoff multiplier | `2` |
| `USER_AGENT` | HTTP User-Agent header | `WebSearchMCP/1.0` |
| `MAX_CONTENT_SIZE` | Max response body size (bytes) | `10485760` (10MB) |
| `TRANSPORT` | Transport type: `stdio` or `http` | `stdio` |
| `HTTP_PORT` | Port for HTTP transport | `8080` |

### Testing Strategy
- Unit tests for each tool handler using mocked HTTP backends (`httptest.Server`).
- Integration tests require a running SearXNG instance; skipped when `SEARXNG_URL` is not set.
- Use `go generate` with `mockgen` for generating mock interfaces of HTTP clients and backends.
