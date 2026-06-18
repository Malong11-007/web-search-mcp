package sitemap

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTool adds the site_map tool.
func RegisterTool(s *server.MCPServer, cfg *config.Config) {
	client := httpclient.New(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	tool := mcp.NewTool("site_map",
		mcp.WithDescription(
			"Discover all URLs on a website by extracting links from the page and optionally checking sitemap.xml. "+
				"Returns a deduplicated list of same-domain URLs grouped by path. "+
				"Best for: understanding a site's structure, finding specific pages within a domain, "+
				"getting a URL list to feed into batch_scrape or crawl. "+
				"Same-domain only — external links are excluded. Results truncated at 200 URLs in output. "+
				"NOT for: reading page content (follow up with scrape_page or batch_scrape on discovered URLs), "+
				"or deep crawling (use crawl which maps AND extracts content). "+
				"Tip: use this when you know the site but not the exact page path, e.g., \"find the pricing page on example.com\".",
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("Any page URL on the target site. All discovered links are resolved relative to this domain."),
		),
		mcp.WithBoolean("check_sitemap",
			mcp.Description("Also fetch and parse sitemap.xml for the domain (default: true). Disable if the site has no sitemap or you only want links from the page itself."),
		),
	)

	s.AddTool(tool, handler(client, cfg, limiter))
}

func handler(client *httpclient.Client, cfg *config.Config, limiter *ratelimit.Limiter) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		startURL := mcp.ParseString(request, "url", "")
		if startURL == "" {
			return mcp.NewToolResultError("url is required"), nil
		}

		checkSitemap := true
		if b, ok := request.Params.Arguments["check_sitemap"].(bool); ok {
			checkSitemap = b
		}

		if err := limiter.Wait(ctx); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		base, err := url.Parse(startURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid URL: %v", err)), nil
		}

		seen := make(map[string]bool)
		var allURLs []string

		// 1. Extract links from the page.
		body, err := client.GetBody(startURL)
		if err == nil {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
			if err == nil {
				doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
					href, _ := s.Attr("href")
					abs := resolveURL(base, href)
					if abs != "" && !seen[abs] {
						seen[abs] = true
						allURLs = append(allURLs, abs)
					}
				})
			}
		}

		// 2. Check sitemap.xml if requested.
		if checkSitemap {
			sitemapURL := fmt.Sprintf("%s://%s/sitemap.xml", base.Scheme, base.Host)
			sitemapBody, err := client.GetBody(sitemapURL)
			if err == nil {
				doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(sitemapBody)))
				if err == nil {
					doc.Find("loc").Each(func(_ int, s *goquery.Selection) {
						loc := strings.TrimSpace(s.Text())
						if loc != "" && !seen[loc] {
							seen[loc] = true
							allURLs = append(allURLs, loc)
						}
					})
				}
			}
		}

		// Build output.
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Site map for: %s\n", startURL))
		sb.WriteString(fmt.Sprintf("Found %d unique URLs\n\n", len(allURLs)))

		// Group by path for readability.
		for i, u := range allURLs {
			if i >= 200 {
				sb.WriteString(fmt.Sprintf("\n... and %d more URLs (truncated)", len(allURLs)-200))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", u))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
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
	// Only return URLs from the same domain.
	if abs.Host != base.Host {
		return ""
	}
	abs.Fragment = ""
	return abs.String()
}
