package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	md "github.com/JohannesKaufmann/html-to-markdown"

	"github.com/dan/web-search-mcp/internal/config"
	"github.com/dan/web-search-mcp/internal/httpclient"
	"github.com/dan/web-search-mcp/internal/pii"
	"github.com/dan/web-search-mcp/internal/ratelimit"
	"github.com/dan/web-search-mcp/internal/retry"
	"github.com/dan/web-search-mcp/internal/robots"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTool adds the scrape_page tool and its handler to the MCP server.
func RegisterTool(s *server.MCPServer, cfg *config.Config) {
	client := httpclient.New(cfg)
	limiter, _ := ratelimit.New(cfg.RateLimit)

	h := &handler{
		client:  client,
		cfg:     cfg,
		limiter: limiter,
	}

	tool := mcp.NewTool("scrape_page",
		mcp.WithDescription(
			"Fetch a URL and extract the main content as clean Markdown (default), JSON, or plain text. "+
				"Strips navigation, ads, headers, footers, and sidebars automatically. "+
				"Best for: reading an article, extracting documentation, getting page content after finding a URL via search. "+
				"NOT for: quick page assessment (use page_metadata first — it's faster and shows reading time/paywall status), "+
				"multiple URLs (use batch_scrape), or discovering which URLs exist on a site (use site_map). "+
				"Set format to \"text\" for minimal output (raw text only) or \"json\" for structured {content: ...} output.",
		),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The full URL to scrape, including https://. Must be a valid, reachable URL."),
		),
		mcp.WithString("format",
			mcp.Description("Output format. \"markdown\" gives formatted content with links and structure, \"text\" gives raw plain text, \"json\" gives {content: ...}."),
			mcp.Enum("markdown", "json", "text"),
		),
	)

	s.AddTool(tool, h.handle)
}

type handler struct {
	client  *httpclient.Client
	cfg     *config.Config
	limiter *ratelimit.Limiter
}

func (h *handler) handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url := mcp.ParseString(request, "url", "")
	if url == "" {
		return mcp.NewToolResultError("url is required"), nil
	}

	format := "markdown"
	if f, ok := request.Params.Arguments["format"].(string); ok && f != "" {
		format = f
	}


		// Check robots.txt if enabled.
		if h.cfg.RespectRobotsTXT {
			if !robots.CheckBeforeScrape(h.client, url) {
				return mcp.NewToolResultError(fmt.Sprintf("blocked by robots.txt: %s", url)), nil
			}
		}

	// Rate limit.
	if err := h.limiter.Wait(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rate limiter: %v", err)), nil
	}

	var body []byte
	err := retry.Do(ctx,
		h.cfg.RetryMaxAttempts,
		h.cfg.RetryInitialDelay,
		h.cfg.RetryMaxDelay,
		h.cfg.RetryBackoffFactor,
		func() error {
			var err error
			body, err = h.client.GetBody(url)
			return err
		},
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scrape failed: %v", err)), nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("parse HTML: %v", err)), nil
	}

	// Extract main content: prefer <article>, then <main>, then <body>.
	contentSel := extractMainContent(doc)

	contentHTML, err := contentSel.Html()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("extract content: %v", err)), nil
	}

	var result string
	switch format {
	case "json":
		result, err = formatJSON(contentHTML, contentSel)
	case "text":
		result = contentSel.Text()
	case "markdown":
		fallthrough
	default:
		result, err = formatMarkdown(contentHTML, url)
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("format output: %v", err)), nil
	}

	if result == "" {
		result = "(empty page)"
	}

	// Truncate if needed.
	if len(result) > int(h.cfg.MaxContentSize) {
		result = result[:h.cfg.MaxContentSize] + "\n\n[truncated]"
	}

	// PII redaction if enabled.
	if h.cfg.RedactPII {
		result = pii.Redact(result)
	}

	return mcp.NewToolResultText(result), nil
}

// extractMainContent returns a goquery selection for the main content area.
// Prefers <article>, falls back to <main>, then <body>.
func extractMainContent(doc *goquery.Document) *goquery.Selection {
	if sel := doc.Find("article"); sel.Length() > 0 {
		return sel.First()
	}
	if sel := doc.Find("main"); sel.Length() > 0 {
		return sel.First()
	}
	// Remove nav, header, footer, script, style before using body.
	doc.Find("nav, header, footer, script, style, noscript, iframe").Remove()
	return doc.Find("body")
}

func formatMarkdown(html, pageURL string) (string, error) {
	converter := md.NewConverter(pageURL, true, nil)
	markdown, err := converter.ConvertString(html)
	if err != nil {
		return "", fmt.Errorf("convert to markdown: %w", err)
	}
	// Collapse excessive blank lines.
	for strings.Contains(markdown, "\n\n\n") {
		markdown = strings.ReplaceAll(markdown, "\n\n\n", "\n\n")
	}
	return markdown, nil
}

type jsonOutput struct {
	Content string `json:"content"`
}

func formatJSON(html string, sel *goquery.Selection) (string, error) {
	text := strings.TrimSpace(sel.Text())
	out := jsonOutput{Content: text}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}


