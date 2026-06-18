package robots

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Malong11-007/web-search-mcp/internal/config"
	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
)

// ── parseRobotsTXT ──────────────────────────────────────────

func TestParseRobotsTXT_AllowAll(t *testing.T) {
	rule := parseRobotsTXT("")
	if rule == nil {
		t.Fatal("expected non-nil rule")
	}
	if len(rule.Disallow) != 0 {
		t.Errorf("expected no disallow rules, got %v", rule.Disallow)
	}
}

func TestParseRobotsTXT_DisallowAll(t *testing.T) {
	content := "User-agent: *\nDisallow: /\n"
	rule := parseRobotsTXT(content)
	if len(rule.Disallow) != 1 || rule.Disallow[0] != "/" {
		t.Errorf("disallow = %v, want [\"/\"]", rule.Disallow)
	}
}

func TestParseRobotsTXT_AllowOverride(t *testing.T) {
	content := "User-agent: *\nDisallow: /\nAllow: /public/\n"
	rule := parseRobotsTXT(content)
	if len(rule.Allow) != 1 || rule.Allow[0] != "/public/" {
		t.Errorf("allow = %v, want [\"/public/\"]", rule.Allow)
	}
}

func TestParseRobotsTXT_CrawlDelay(t *testing.T) {
	content := "User-agent: *\nCrawl-delay: 10\n"
	rule := parseRobotsTXT(content)
	if rule.CrawlDelay != 10*time.Second {
		t.Errorf("CrawlDelay = %v, want 10s", rule.CrawlDelay)
	}
}

func TestParseRobotsTXT_MultipleUserAgents(t *testing.T) {
	content := "User-agent: googlebot\nDisallow: /no-google/\nUser-agent: *\nDisallow: /no-all/\n"
	rule := parseRobotsTXT(content)
	// The parser collects all rules for all matching UAs; "*" matches everything.
	// After parsing all directives, "*" rules accumulate.
	if len(rule.Disallow) < 1 {
		t.Error("expected at least one disallow rule")
	}
}

func TestParseRobotsTXT_Comments(t *testing.T) {
	content := "# This is a comment\nUser-agent: *  # end-of-line\nDisallow: /private/\n"
	rule := parseRobotsTXT(content)
	if len(rule.Disallow) != 1 || rule.Disallow[0] != "/private/" {
		t.Errorf("disallow = %v, want [\"/private/\"]", rule.Disallow)
	}
}

func TestParseRobotsTXT_EmptyLines(t *testing.T) {
	content := "\n\nUser-agent: *\n\nDisallow: /tmp/\n\n"
	rule := parseRobotsTXT(content)
	if len(rule.Disallow) != 1 || rule.Disallow[0] != "/tmp/" {
		t.Errorf("disallow = %v, want [\"/tmp/\"]", rule.Disallow)
	}
}

func TestParseRobotsTXT_SitemapIgnored(t *testing.T) {
	content := "Sitemap: https://example.com/sitemap.xml\nUser-agent: *\nDisallow: /private/\n"
	rule := parseRobotsTXT(content)
	if len(rule.Disallow) != 1 {
		t.Errorf("expected sitemap line to be ignored, got disallow count %d", len(rule.Disallow))
	}
}

// ── Cache / IsKnown ─────────────────────────────────────────

func TestCache_StoreAndIsKnown(t *testing.T) {
	c := NewCache()
	c.Store("example.com", "User-agent: *\nDisallow: /\n", 1*time.Hour)

	if !c.IsKnown("example.com") {
		t.Error("expected IsKnown true after Store")
	}
	if c.IsKnown("other.com") {
		t.Error("expected IsKnown false for unknown domain")
	}
}

func TestCache_StoreExpired(t *testing.T) {
	c := NewCache()
	c.Store("example.com", "User-agent: *\nDisallow: /\n", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if c.IsKnown("example.com") {
		t.Error("expected IsKnown false after TTL expires")
	}
}

// ── Allowed ──────────────────────────────────────────────────

func TestCache_Allowed_NoCache(t *testing.T) {
	c := NewCache()
	if !c.Allowed("https://example.com/page", "testbot") {
		t.Error("expected allowed when no cached rule")
	}
}

func TestCache_Allowed_Disallowed(t *testing.T) {
	c := NewCache()
	c.Store("example.com", "User-agent: *\nDisallow: /private/\n", 1*time.Hour)
	if c.Allowed("https://example.com/private/data", "") {
		t.Error("expected disallowed for /private/")
	}
	if !c.Allowed("https://example.com/public/data", "") {
		t.Error("expected allowed for /public/")
	}
}

func TestCache_Allowed_Expired(t *testing.T) {
	c := NewCache()
	c.Store("example.com", "User-agent: *\nDisallow: /\n", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if !c.Allowed("https://example.com/page", "") {
		t.Error("expected allowed when cache expired (allow all)")
	}
}

func TestCache_Allowed_InvalidURL(t *testing.T) {
	c := NewCache()
	// Unparseable URL falls back to allowed.
	if !c.Allowed("://bad-url", "") {
		t.Error("expected allowed for invalid URL")
	}
}

// ── CrawlDelay ──────────────────────────────────────────────

func TestCache_CrawlDelay(t *testing.T) {
	c := NewCache()
	c.Store("example.com", "User-agent: *\nCrawl-delay: 5\n", 1*time.Hour)

	d := c.CrawlDelay("https://example.com/page")
	if d != 5*time.Second {
		t.Errorf("CrawlDelay = %v, want 5s", d)
	}
}

func TestCache_CrawlDelay_Unknown(t *testing.T) {
	c := NewCache()
	d := c.CrawlDelay("https://unknown.example/page")
	if d != 0 {
		t.Errorf("CrawlDelay = %v, want 0", d)
	}
}

// ── matchesUserAgent ────────────────────────────────────────

func TestMatchesUserAgent(t *testing.T) {
	// Empty rule UAs → match all.
	if !matchesUserAgent("mybot/1.0", nil) {
		t.Error("expected match when rule UAs is nil")
	}
	// Wildcard.
	if !matchesUserAgent("mybot/1.0", []string{"*"}) {
		t.Error("expected match for *")
	}
	// Substring match.
	if !matchesUserAgent("mybot/1.0", []string{"bot"}) {
		t.Error("expected match for 'bot' in 'mybot/1.0'")
	}
	if !matchesUserAgent("websearch-mcp/1.0", []string{"websearch"}) {
		t.Error("expected match for 'websearch' in 'websearch-mcp/1.0'")
	}
	// No match.
	if matchesUserAgent("mybot/1.0", []string{"googlebot"}) {
		t.Error("expected no match for 'googlebot' vs 'mybot/1.0'")
	}
}

// ── pathMatches ─────────────────────────────────────────────

func TestPathMatches(t *testing.T) {
	tests := []struct {
		path    string
		pattern string
		want    bool
	}{
		// Empty pattern → no match.
		{"anything", "", false},
		// Exact root.
		{"/", "/", true},
		{"/page", "/", false}, // "/" only matches "/" exactly
		// Prefix match.
		{"/private/data", "/private/", true},
		{"/public/data", "/private/", false},
		// Wildcard.
		{"/blog/post-1", "/blog/*", true},
		{"/other/page", "/blog/*", false},
		// End anchor.
		{"/page.html", "/*.html$", true},
		{"/page.htm", "/*.html$", false},
		// Complex glob.
		{"/docs/api/v1/users", "/docs/*/v1/*", true},
	}

	for _, tt := range tests {
		got := pathMatches(tt.path, tt.pattern)
		if got != tt.want {
			t.Errorf("pathMatches(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		want    bool
	}{
		{"", "", true},
		{"anything", "*", true},
		{"/page.html", "*.html", true},
		{"/page.htm", "*.html", false},
		{"/a/b/c", "*/b/*", true},
	}
	for _, tt := range tests {
		got := globMatch(tt.s, tt.pattern)
		if got != tt.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
		}
	}
}

// ── FetchAndCache / CheckBeforeScrape ──────────────────────

func newRobotsServer(content string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/robots.txt") {
			w.WriteHeader(status)
			w.Write([]byte(content))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("<html>ok</html>"))
	}))
}

func testClient() *httpclient.Client {
	return httpclient.New(&config.Config{
		HTTPTimeout:    10 * time.Second,
		UserAgent:      "TestBot/1.0",
		MaxContentSize: 10 * 1024 * 1024,
	})
}

func newRobotsServerPortion(hostport string) string {
	rest := hostport
	if idx := strings.Index(rest, "://"); idx >= 0 {
		rest = rest[idx+3:]
	}
	return rest
}

func TestFetchAndCache_OK(t *testing.T) {
	ts := newRobotsServer("User-agent: *\nDisallow: /\n", 200)
	defer ts.Close()

	client := testClient()
	err := FetchAndCache(client, ts.URL+"/some-page")
	if err != nil {
		t.Fatalf("FetchAndCache: %v", err)
	}
	if !DefaultCache.IsKnown(newRobotsServerPortion(ts.URL)) {
		t.Error("expected IsKnown true after FetchAndCache")
	}
	// Reset DefaultCache for other tests.
	DefaultCache = NewCache()
}

func TestFetchAndCache_404(t *testing.T) {
	ts := newRobotsServer("", 404)
	defer ts.Close()

	client := testClient()
	err := FetchAndCache(client, ts.URL+"/some-page")
	if err != nil {
		t.Fatalf("FetchAndCache: %v", err)
	}
	DefaultCache = NewCache()
}

func TestCheckBeforeScrape_Allowed(t *testing.T) {
	ts := newRobotsServer("User-agent: *\nDisallow: /private/\n", 200)
	defer ts.Close()

	DefaultCache = NewCache()
	client := testClient()

	FetchAndCache(client, ts.URL+"/some-page")

	if !CheckBeforeScrape(client, ts.URL+"/public/page") {
		t.Error("expected allowed for /public/")
	}
	if CheckBeforeScrape(client, ts.URL+"/private/data") {
		t.Error("expected blocked for /private/")
	}
	DefaultCache = NewCache()
}

func TestCheckBeforeScrape_NoCache(t *testing.T) {
	DefaultCache = NewCache()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/robots.txt") {
			w.Write([]byte("User-agent: *\nDisallow:\n"))
			return
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	client := testClient()
	if !CheckBeforeScrape(client, ts.URL+"/page") {
		t.Error("expected allowed when no cached rules (fresh fetch)")
	}
	DefaultCache = NewCache()
}
