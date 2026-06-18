package scrape

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterMetadataTool adds the page_metadata tool.
func RegisterMetadataTool(s *server.MCPServer, cfg *config.Config) {
	client := httpclient.New(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	tool := mcp.NewTool("page_metadata",
		mcp.WithDescription(
			"Inspect a URL's metadata WITHOUT downloading full content (fetches the page but extracts only metadata). "+
				"Returns: title, meta description, Open Graph tags (og:title, og:description, og:image), Twitter card, "+
				"canonical URL, author, publish date, language, word count, estimated reading time, page size, and paywall detection. "+
				"Best for: quickly assessing a page before committing to a full scrape, checking if content is accessible, "+
				"verifying a page's reading time is reasonable, or extracting OG images for previews. "+
				"NOT for: reading the actual page content (use scrape_page after confirming the page is worth it). "+
				"Tip: use this BEFORE scrape_page on unfamiliar URLs — it can save you from scraping a 30-minute read or a paywalled article.",
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to inspect. Must include https://."),
		),
	)

	s.AddTool(tool, metadataHandler(client, cfg, limiter))
}

func metadataHandler(client *httpclient.Client, cfg *config.Config, limiter *ratelimit.Limiter) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pageURL := mcp.ParseString(request, "url", "")
		if pageURL == "" {
			return mcp.NewToolResultError("url is required"), nil
		}

		if err := limiter.Wait(ctx); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body, err := client.GetBody(pageURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetch failed: %v", err)), nil
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse HTML: %v", err)), nil
		}

		meta := extractMetadata(doc, pageURL, len(body))

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Page metadata for: %s\n\n", pageURL))
		sb.WriteString(fmt.Sprintf("- **Title:** %s\n", meta.Title))
		sb.WriteString(fmt.Sprintf("- **Description:** %s\n", meta.Description))
		if meta.OGTitle != "" {
			sb.WriteString(fmt.Sprintf("- **OG Title:** %s\n", meta.OGTitle))
		}
		if meta.OGDescription != "" {
			sb.WriteString(fmt.Sprintf("- **OG Description:** %s\n", meta.OGDescription))
		}
		if meta.OGImage != "" {
			sb.WriteString(fmt.Sprintf("- **OG Image:** %s\n", meta.OGImage))
		}
		if meta.TwitterCard != "" {
			sb.WriteString(fmt.Sprintf("- **Twitter Card:** %s\n", meta.TwitterCard))
		}
		if meta.CanonicalURL != "" {
			sb.WriteString(fmt.Sprintf("- **Canonical URL:** %s\n", meta.CanonicalURL))
		}
		if meta.Author != "" {
			sb.WriteString(fmt.Sprintf("- **Author:** %s\n", meta.Author))
		}
		if meta.PublishDate != "" {
			sb.WriteString(fmt.Sprintf("- **Publish Date:** %s\n", meta.PublishDate))
		}
		if meta.Language != "" {
			sb.WriteString(fmt.Sprintf("- **Language:** %s\n", meta.Language))
		}
		sb.WriteString(fmt.Sprintf("- **Word Count:** %d\n", meta.WordCount))
		sb.WriteString(fmt.Sprintf("- **Reading Time:** %s\n", meta.ReadingTime))
		sb.WriteString(fmt.Sprintf("- **Page Size:** %s\n", formatBytes(len(body))))
		sb.WriteString(fmt.Sprintf("- **Has Paywall:** %v\n", meta.HasPaywall))
		sb.WriteString(fmt.Sprintf("- **StatusCode:** %d\n", meta.StatusCode))

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// PageMetadata holds extracted page information.
type PageMetadata struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	OGTitle      string `json:"og_title,omitempty"`
	OGDescription string `json:"og_description,omitempty"`
	OGImage      string `json:"og_image,omitempty"`
	TwitterCard  string `json:"twitter_card,omitempty"`
	CanonicalURL string `json:"canonical_url,omitempty"`
	Author       string `json:"author,omitempty"`
	PublishDate  string `json:"publish_date,omitempty"`
	Language     string `json:"language,omitempty"`
	WordCount    int    `json:"word_count"`
	ReadingTime  string `json:"reading_time"`
	HasPaywall   bool   `json:"has_paywall"`
	StatusCode   int    `json:"status_code"`
}

func extractMetadata(doc *goquery.Document, pageURL string, bodyLen int) PageMetadata {
	sel := doc.Selection

	meta := PageMetadata{
		Title:       strings.TrimSpace(sel.Find("title").First().Text()),
		Description: attr(sel, "meta[name=description]", "content"),
		OGTitle:     attr(sel, "meta[property='og:title']", "content"),
		OGDescription: attr(sel, "meta[property='og:description']", "content"),
		OGImage:     attr(sel, "meta[property='og:image']", "content"),
		TwitterCard: attr(sel, "meta[name='twitter:card']", "content"),
		CanonicalURL: attr(sel, "link[rel=canonical]", "href"),
		Author:      attr(sel, "meta[name=author]", "content"),
		PublishDate: attr(sel, "meta[property='article:published_time']", "content"),
		Language:    attr(sel, "html", "lang"),
		StatusCode:  200,
	}

	// Word count from main content.
	contentSel := extractMainContent(doc)
	text := contentSel.Text()
	meta.WordCount = len(strings.Fields(text))

	// Reading time: average 200 words per minute.
	minutes := int(math.Ceil(float64(meta.WordCount) / 200.0))
	if minutes < 1 {
		minutes = 1
	}
	meta.ReadingTime = fmt.Sprintf("%d min", minutes)

	// Heuristic paywall detection.
	bodyLower := strings.ToLower(doc.Text())
	paywallTerms := []string{"subscribe to read", "premium article", "paywall", "sign up to continue"}
	for _, term := range paywallTerms {
		if strings.Contains(bodyLower, term) {
			meta.HasPaywall = true
			break
		}
	}

	// Fallback: try <meta name="description"> then first <p> text.
	if meta.Description == "" {
		meta.Description = truncateText(contentSel.Find("p").First().Text(), 300)
	}

	return meta
}

func attr(sel *goquery.Selection, selector, attrName string) string {
	s := sel.Find(selector).First()
	if s.Length() == 0 {
		return ""
	}
	val, _ := s.Attr(attrName)
	return strings.TrimSpace(val)
}

func truncateText(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatBytes(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// ensure time is used
var _ = time.Now
