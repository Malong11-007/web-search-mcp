# web-search-mcp

A Model Context Protocol (MCP) server that turns the web from a search index into a readable, explorable document store. Built for Claude Code, GitHub Copilot, and any MCP-compatible client.

**10 tools. Zero API keys. Works out of the box.**

## Why this exists

Claude Code and Copilot have built-in web search — but they only return titles and 150-character snippets. You can't actually **read** the pages they find. This server gives your AI:

- **Content extraction** — scrape any URL into clean Markdown (not raw HTML)
- **Site exploration** — map all URLs on a domain, crawl entire doc sections
- **Deep research** — search + scrape + synthesize in a single call
- **Stealth anti-blocking** — UA rotation, browser headers, cookie jar, retry jitter
- **Privacy** — self-hosted, no API keys, DuckDuckGo works immediately

## Quick Install

### Option 1: Go install (if you have Go)

```bash
go install github.com/Malong11-007/web-search-mcp/cmd/web-search-mcp@latest
```

### Option 2: One-liner (curl)

```bash
curl -fsSL https://raw.githubusercontent.com/Malong11-007/web-search-mcp/main/install.sh | bash
```

### Option 3: Build from source

```bash
git clone git@github.com:Malong11-007/web-search-mcp.git
cd web-search-mcp
go build -o web-search-mcp ./cmd/web-search-mcp/
```

## Configure Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "web-search": {
      "command": "web-search-mcp",
      "args": []
    }
  }
}
```

Or point to the binary directly:

```json
{
  "mcpServers": {
    "web-search": {
      "command": "/path/to/web-search-mcp",
      "args": []
    }
  }
}
```

## Configure GitHub Copilot

Add to `.github/copilot-instructions.md` in your project, or configure in VS Code / JetBrains Copilot settings as an MCP server:

```json
{
  "mcpServers": {
    "web-search": {
      "command": "web-search-mcp",
      "args": []
    }
  }
}
```

## Quick Start

After configuring, restart Claude Code / Copilot. The tools appear automatically. Try:

```
> Search the web for "Go generics tutorial 2026" and read the top 2 results
```

Claude will call `web_search` → `batch_scrape` on the top URLs.

Or shorter:

```
> Research the current state of WebAssembly GC and summarize the top 3 sources
```

Claude will call `deep_research` — search + scrape in one call.

## Tools

### Search

| Tool | What it does |
|------|-------------|
| `web_search` | Search the web (DuckDuckGo → SearXNG fallback). Returns titles, URLs, descriptions. |
| `batch_search` | Run up to 10 searches in parallel. Returns grouped results per query. |
| `search_feedback` | Rate a result's relevance. Helps tune future rankings. |

### Content Extraction

| Tool | What it does |
|------|-------------|
| `scrape_page` | Fetch a URL, extract main content as clean Markdown/JSON/text. Strips nav, ads, footers. |
| `batch_scrape` | Scrape up to 20 URLs in parallel. |
| `page_metadata` | Quick page assessment: title, OG tags, reading time, paywall detection, word count. Use before scraping to avoid wasting tokens. |

### Site Exploration

| Tool | What it does |
|------|-------------|
| `site_map` | Discover all URLs on a domain (page links + sitemap.xml). Same-domain only. |
| `crawl` | BFS crawl with depth/pages/path-prefix controls. Extracts clean content from each page. |

### Research

| Tool | What it does |
|------|-------------|
| `deep_research` | Search → scrape top N results → synthesized report. One call for "research X" tasks. |

## Tool Selection Guide

```
Task                                    Best tool
────                                    ─────────
"What is X?"                            web_search
"Is claim Y true? Check multiple srcs"  batch_search
"Read this article" (have URL)          scrape_page
"Is this page worth reading?" (URL)     page_metadata → then scrape_page
"Read these 5 pages" (have URLs)        batch_scrape
"What pages exist on example.com?"      site_map
"Extract all docs from /docs/"          crawl (path_prefix="/docs/")
"Research X comprehensively"            deep_research
```

## Configuration

All settings via environment variables:

```bash
# Search backends (tried in order)
SEARCH_BACKENDS=duckduckgo,searxng

# Which tool groups to enable
TOOLS=search,scrape,batch,crawl,map,research

# Anti-blocking (both default true)
STEALTH_MODE=true          # UA rotation, browser headers, cookie jar
RESPECT_ROBOTS_TXT=true    # Check robots.txt before scraping

# Custom user-agent (empty = auto-rotate 8 realistic UAs)
USER_AGENT=""

# Rate limiting
RATE_LIMIT=100/1m          # 100 requests per minute

# Retry with jitter
RETRY_MAX_ATTEMPTS=3
RETRY_INITIAL_DELAY=1s
RETRY_MAX_DELAY=30s
RETRY_BACKOFF_FACTOR=2.0

# Crawl defaults
CRAWL_MAX_DEPTH=3
CRAWL_MAX_PAGES=50

# Content limits
MAX_CONTENT_SIZE=10485760  # 10MB

# Privacy
REDACT_PII=false           # Auto-redact emails, phones, SSNs, IPs

# Transport
TRANSPORT=stdio            # or "http" for remote access via SSE
HTTP_PORT=8080
```

## Anti-Blocking

When `STEALTH_MODE=true` (default), every HTTP request mimics a real browser:

- **UA rotation** — 8 Chrome/Firefox/Safari/Edge user-agents, rotated per request
- **Browser headers** — Accept, Accept-Language, Sec-Fetch-*, Cache-Control
- **Cookie jar** — per-domain session persistence
- **Referrer chain** — tracks navigation context across requests
- **Retry jitter** — randomized backoff to avoid bot-like patterns
- **robots.txt** — fetches/parses/caches per domain, checks before every scrape/crawl

This won't beat IP-based blocking (you need a proxy network for that), but it defeats header-pattern and request-pattern detection used by most WAFs.

## Architecture

```
cmd/web-search-mcp/main.go          Entrypoint: tool registration, stdio/HTTP+SSE
internal/
  ├── async/job.go          Background job manager (create/poll/reap)
  ├── config/config.go      16 env vars, tool group filtering
  ├── crawl/tool.go         BFS crawler: depth, pages, path-prefix, same-domain
  ├── httpclient/
  │   ├── client.go         Stealth HTTP client (UA rotation, headers, cookies)
  │   └── ua.go             UA pool + browser header generation
  ├── map/tool.go           Link extraction + sitemap.xml parsing
  ├── pii/redact.go         Regex-based PII scrubbing (email, phone, SSN, IP, CC)
  ├── ratelimit/            Token-bucket rate limiter ("100/1m" format)
  ├── research/tool.go      Search → scrape → synthesize (deep_research)
  ├── retry/                Exponential backoff with full jitter
  ├── robots/robots.go      robots.txt fetch/parse/cache + Allow/Disallow checks
  ├── scrape/
  │   ├── tool.go           Single-page scrape (Markdown/JSON/text)
  │   ├── batch.go          Parallel batch scrape (up to 20 URLs)
  │   └── metadata.go       Page metadata extractor (OG tags, reading time, paywall)
  └── search/
      ├── backend.go        Search backend interface
      ├── duckduckgo.go     DuckDuckGo HTML scraper
      ├── searxng.go        SearXNG JSON API client
      ├── fallback.go       Multi-backend with automatic failover
      ├── tool.go           web_search handler
      └── batch.go          batch_search handler
```

## Requirements

- **Go 1.23+** (to build from source)
- **Nothing else** — DuckDuckGo works immediately with no configuration
- Optional: SearXNG instance URL for a second search backend (`SEARXNG_URL`)

## License

MIT
