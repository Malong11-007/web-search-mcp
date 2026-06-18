# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] — 2026-06-18

### Fixed

- **Scrape tools returned garbled binary content.** The HTTP client's stealth headers
  advertised `Accept-Encoding: gzip, deflate, br`. Go's `net/http` auto-decompresses
  gzip and deflate but not brotli, so brotli-compressed responses arrived as raw binary.
  Removing the `Accept-Encoding` header entirely lets Go's `Transport` handle compression
  transparently. Affected: `scrape_page`, `batch_scrape`, `crawl`, `deep_research`.

- **SearXNG 403 burned all retry attempts.** The default `SEARXNG_URL` pointed to
  `https://searx.be`, which returns 403. When DuckDuckGo also had transient failures,
  SearXNG was the fallback and its 403 consumed all retries. Now `SEARXNG_URL` defaults
  to empty (opt-in), and the backend is skipped when no URL is configured.

- **DuckDuckGo 202 rate-limiting.** DuckDuckGo intermittently returns HTTP 202 when
  rate-limited. Previously treated as a fatal error, the DDG backend now waits 2 seconds
  and retries once when it receives 202.

- **Version reporting.** The server now reports its version at startup (was hardcoded
  to `"0.2.0"`). A `--version` / `-v` flag prints build version, commit SHA, and date.
  Build with `-ldflags` to bake in release metadata.

## [0.2.0] — 2026-06-17

### Added

- Initial release: 10 MCP tools across 6 tool groups.
- `web_search` — search via DuckDuckGo and SearXNG with automatic failover.
- `batch_search` — run up to 10 searches in parallel.
- `search_feedback` — rate search result relevance.
- `scrape_page` — extract clean Markdown/JSON/text from any URL.
- `batch_scrape` — scrape up to 20 URLs in parallel.
- `page_metadata` — quick page assessment (OG tags, reading time, paywall detection).
- `site_map` — discover all URLs on a domain via link extraction and sitemap.xml.
- `crawl` — BFS crawl with depth, page count, and path-prefix controls.
- `deep_research` — search → scrape top N results → synthesized report.
- Stealth anti-blocking: UA rotation (8 browser UAs), browser headers, cookie jar,
  referrer tracking, retry with full jitter, robots.txt compliance.
- Configurable via 16 environment variables.
- stdio and HTTP/SSE transports.
- Privacy: optional PII redaction.

[Unreleased]: https://github.com/Malong11-007/web-search-mcp/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/Malong11-007/web-search-mcp/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Malong11-007/web-search-mcp/releases/tag/v0.2.0
