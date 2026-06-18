package crawl

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/pii"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/Malong11-007/web-search-mcp/internal/robots"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTool adds the crawl tool.
func RegisterTool(s *server.MCPServer, cfg *config.Config) {
	client := httpclient.New(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	tool := mcp.NewTool("crawl",
		mcp.WithDescription(
			"Crawl a website starting from a URL, following same-domain links up to max_depth. "+
				"Extracts clean content from each page (strips nav, ads, footers). "+
				"Best for: extracting all documentation pages under /docs/, downloading a blog series, mapping a knowledge base section. "+
				"⚠️ SYNCHRONOUS — each page is fetched sequentially with rate limiting. A crawl of 50 pages may take 30+ seconds. "+
				"Start with max_pages=5 and max_depth=1 for a first pass, then increase if needed. "+
				"Uses path_prefix to scope the crawl (e.g., \"/docs/\" only crawls URLs containing /docs/). "+
				"Same-domain only — will not follow links to other websites. "+
				"Each page's content is truncated to 3000 chars in the output. "+
				"NOT for: a single page (use scrape_page), discovering URLs without content (use site_map), "+
				"or cross-domain research (use deep_research instead).",
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("Starting URL. Crawl stays on this domain and follows links found on each page."),
		),
		mcp.WithNumber("max_depth",
			mcp.Description(fmt.Sprintf("How many link-hops from the start URL (default %d, max 5). Depth 1 = start page + its direct links. Depth 2 = those pages + their links.", cfg.CrawlMaxDepth)),
		),
		mcp.WithNumber("max_pages",
			mcp.Description(fmt.Sprintf("Stop after crawling this many pages (default %d, max 100). Protects against accidentally crawling hundreds of pages.", cfg.CrawlMaxPages)),
		),
		mcp.WithString("path_prefix",
			mcp.Description("Only crawl URLs containing this path prefix. Example: \"/docs/api/\" restricts to API docs. Leave empty to crawl the entire site (within depth/pages limits)."),
		),
	)

	s.AddTool(tool, handler(client, cfg, limiter))
}

type crawlResult struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Depth   int    `json:"depth"`
}

func handler(client *httpclient.Client, cfg *config.Config, limiter *ratelimit.Limiter) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		startURL := mcp.ParseString(request, "url", "")
		if startURL == "" {
			return mcp.NewToolResultError("url is required"), nil
		}

		maxDepth := cfg.CrawlMaxDepth
		if n, ok := request.Params.Arguments["max_depth"].(float64); ok {
			maxDepth = int(n)
			if maxDepth < 1 {
				maxDepth = 1
			}
			if maxDepth > 5 {
				maxDepth = 5
			}
		}

		maxPages := cfg.CrawlMaxPages
		if n, ok := request.Params.Arguments["max_pages"].(float64); ok {
			maxPages = int(n)
			if maxPages < 1 {
				maxPages = 1
			}
			if maxPages > 100 {
				maxPages = 100
			}
		}

		pathPrefix := mcp.ParseString(request, "path_prefix", "")

		base, err := url.Parse(startURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid URL: %v", err)), nil
		}

		converter := md.NewConverter(base.String(), true, nil)

		var (
			mu       sync.Mutex
			results  []crawlResult
			seen     = make(map[string]bool)
			pages    int
		)

		// BFS crawl.
		type job struct {
			url   string
			depth int
		}
		queue := []job{{url: startURL, depth: 0}}

		for len(queue) > 0 && pages < maxPages {
			j := queue[0]
			queue = queue[1:]

			// Normalize.
			parsed, err := url.Parse(j.url)
			if err != nil {
				continue
			}
			abs := base.ResolveReference(parsed)
			abs.Fragment = ""
			normURL := abs.String()

			if seen[normURL] {
				continue
			}
			// Only same domain.
			if abs.Host != base.Host {
				continue
			}
			// Path prefix filter.
			if pathPrefix != "" && !strings.HasPrefix(abs.Path, pathPrefix) {
				continue
			}

			seen[normURL] = true

			// Check robots.txt if enabled.
			if cfg.RespectRobotsTXT {
				if !robots.CheckBeforeScrape(client, normURL) {
					continue
				}
			}

			if err := limiter.Wait(ctx); err != nil {
				break
			}

			body, err := client.GetBody(normURL)
			if err != nil {
				continue
			}

			doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
			if err != nil {
				continue
			}

			// Extract content.
			contentSel := extractMain(doc)
			html, _ := contentSel.Html()
			markdown, _ := converter.ConvertString(html)
			if cfg.RedactPII {
				markdown = pii.Redact(markdown)
			}

			title := strings.TrimSpace(doc.Find("title").First().Text())

			mu.Lock()
			results = append(results, crawlResult{
				URL:     normURL,
				Title:   title,
				Content: markdown,
				Depth:   j.depth,
			})
			pages++
			mu.Unlock()

			// Enqueue links if not at max depth.
			if j.depth < maxDepth {
				doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
					href, _ := s.Attr("href")
					abs := resolveURL(base, href)
					if abs != "" && !seen[abs] {
						queue = append(queue, job{url: abs, depth: j.depth + 1})
					}
				})
			}
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Crawl: %s\n", startURL))
		sb.WriteString(fmt.Sprintf("Pages: %d, Max depth: %d\n\n", len(results), maxDepth))

		for _, r := range results {
			sb.WriteString(fmt.Sprintf("## %s\n", r.Title))
			sb.WriteString(fmt.Sprintf("URL: %s | Depth: %d\n\n", r.URL, r.Depth))
			sb.WriteString(truncate(r.Content, 3000))
			sb.WriteString("\n\n---\n\n")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func extractMain(doc *goquery.Document) *goquery.Selection {
	if sel := doc.Find("article"); sel.Length() > 0 {
		return sel.First()
	}
	if sel := doc.Find("main"); sel.Length() > 0 {
		return sel.First()
	}
	doc.Find("nav, header, footer, script, style, noscript, iframe").Remove()
	return doc.Find("body")
}

func resolveURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	abs := base.ResolveReference(parsed)
	if abs.Host != base.Host {
		return ""
	}
	abs.Fragment = ""
	return abs.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
