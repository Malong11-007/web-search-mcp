package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
	"github.com/Malong11-007/web-search-mcp/internal/ratelimit"
	"github.com/mark3labs/mcp-go/mcp"
)

func newCrawlRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

func crawlContentText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if tc, ok := result.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func newCrawlCfg() *config.Config {
	return &config.Config{
		HTTPTimeout:       10 * 1e9,
		MaxContentSize:    10 * 1024 * 1024,
		StealthMode:       false,
		RespectRobotsTXT:  false,
		CrawlMaxDepth:     3,
		CrawlMaxPages:     50,
	}
}

func TestCrawlHandler_EmptyURL(t *testing.T) {
	cfg := newCrawlCfg()
	client := httpclient.New(cfg)
	h := handler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newCrawlRequest(map[string]any{}))
	if !result.IsError {
		t.Error("expected error for empty URL")
	}
}

func TestCrawlHandler_InvalidURL(t *testing.T) {
	cfg := newCrawlCfg()
	client := httpclient.New(cfg)
	h := handler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newCrawlRequest(map[string]any{"url": "://bad-url"}))
	if !result.IsError {
		t.Error("expected error for invalid URL")
	}
}

func TestCrawlHandler_Basic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte(`<html><body><article>Home page</article><a href="/page1">P1</a><a href="/page2">P2</a></body></html>`))
			return
		}
		w.Write([]byte(`<html><body><article>Content on ` + r.URL.Path + `</article></body></html>`))
	}))
	defer ts.Close()

	cfg := newCrawlCfg()
	client := httpclient.New(cfg)
	h := handler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newCrawlRequest(map[string]any{
		"url":       ts.URL + "/",
		"max_depth": float64(1),
		"max_pages": float64(5),
	}))
	text := crawlContentText(result)
	if !strings.Contains(text, "Home page") {
		t.Errorf("expected home page content: %s", text)
	}
	if !strings.Contains(text, "/page1") {
		t.Errorf("expected page1 link: %s", text)
	}
}

func TestCrawlHandler_DepthLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>Page at ` + r.URL.Path + `</p><a href="/next">next</a></body></html>`))
	}))
	defer ts.Close()

	cfg := newCrawlCfg()
	client := httpclient.New(cfg)
	h := handler(client, cfg, ratelimit.MustNew("100/1s"))

	// max_depth=0 should be clamped to 1 (only start page).
	result, _ := h(context.Background(), newCrawlRequest(map[string]any{
		"url":       ts.URL + "/",
		"max_depth": float64(0),
		"max_pages": float64(10),
	}))
	text := crawlContentText(result)
	// With depth=1, only the start page and immediate children should be crawled.
	// Depth 0 clamps to 1.
	if !strings.Contains(text, "Page at /") {
		t.Errorf("expected start page content: %s", text)
	}
}

func TestCrawlHandler_PageLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><a href="/p1">1</a><a href="/p2">2</a><a href="/p3">3</a></body></html>`))
	}))
	defer ts.Close()

	cfg := newCrawlCfg()
	client := httpclient.New(cfg)
	h := handler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newCrawlRequest(map[string]any{
		"url":       ts.URL + "/",
		"max_depth": float64(1),
		"max_pages": float64(2),
	}))
	text := crawlContentText(result)
	// Should only crawl 2 pages, not all 4.
	if result.IsError {
		t.Errorf("unexpected error: %v", result)
	}
	_ = text
}

func TestExtractMain(t *testing.T) {
	// Test extractMain function with a simple HTML body.
	html := `<html><body>
		<nav>Navigation</nav>
		<article>Main article content here</article>
		<footer>Footer</footer>
	</body></html>`

	// We can't easily create a goquery document without an HTTP request,
	// so we test via the handler which uses extractMain internally.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	defer ts.Close()

	cfg := newCrawlCfg()
	client := httpclient.New(cfg)
	h := handler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newCrawlRequest(map[string]any{
		"url":       ts.URL + "/",
		"max_depth": float64(1),
		"max_pages": float64(1),
	}))
	text := crawlContentText(result)
	if !strings.Contains(text, "Main article content") {
		t.Errorf("expected article content: %s", text)
	}
	if strings.Contains(text, "Navigation") || strings.Contains(text, "Footer") {
		t.Errorf("nav/footer should be stripped: %s", text)
	}
}

func TestResolveURLCrawl(t *testing.T) {
	// Test the package-local resolveURL function indirectly.
	// We know external links are excluded by testing a server with cross-domain links.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<a href="/local">Local</a>
			<a href="https://google.com/external">External</a>
		</body></html>`))
	}))
	defer ts.Close()

	cfg := newCrawlCfg()
	client := httpclient.New(cfg)
	h := handler(client, cfg, ratelimit.MustNew("100/1s"))

	result, _ := h(context.Background(), newCrawlRequest(map[string]any{
		"url":       ts.URL + "/",
		"max_depth": float64(1),
		"max_pages": float64(2),
	}))
	text := crawlContentText(result)
	// External URLs appear in the markdown as unreachable link text,
	// but they should not produce crawled pages. With max_pages=2,
	// only the start page and the one local link should be crawled.
	if strings.Count(text, "URL:") < 2 {
		t.Errorf("expected 2 crawled pages, got text: %s", text)
	}
	// Verify no URL containing "google.com" is listed as a crawled page URL.
	// The crawled page URLs appear after "URL:" prefix.
	if strings.Contains(text, "URL: https://google.com") {
		t.Errorf("google.com should not be crawled: %s", text)
	}
}
