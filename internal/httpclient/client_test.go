package httpclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Malong11-007/web-search-mcp/internal/config"
)

func newTestConfig() *config.Config {
	return &config.Config{
		HTTPTimeout:    10 * time.Second,
		UserAgent:      "TestBot/1.0",
		MaxContentSize: 10 * 1024 * 1024,
		StealthMode:    false,
	}
}

func newStealthConfig() *config.Config {
	cfg := newTestConfig()
	cfg.StealthMode = true
	cfg.UserAgent = "" // Use rotation pool.
	return cfg
}

func TestNew_Default(t *testing.T) {
	cfg := newTestConfig()
	c := New(cfg)
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.cfg != cfg {
		t.Error("config not stored")
	}
	if c.ua != "TestBot/1.0" {
		t.Errorf("ua = %q, want TestBot/1.0", c.ua)
	}
}

func TestNew_CustomUA(t *testing.T) {
	cfg := newTestConfig()
	cfg.UserAgent = "MyBot/2.0"
	c := New(cfg)
	if c.ua != "MyBot/2.0" {
		t.Errorf("ua = %q, want MyBot/2.0", c.ua)
	}
}

func TestNew_RotatingUA(t *testing.T) {
	cfg := newStealthConfig() // UA empty → rotate.
	c := New(cfg)
	if c.ua != "" {
		t.Errorf("ua = %q, want empty (rotate)", c.ua)
	}
}

func TestGetSimple(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("User-Agent"), "TestBot") {
			t.Errorf("unexpected UA: %s", r.Header.Get("User-Agent"))
		}
		fmt.Fprintln(w, "hello")
	}))
	defer ts.Close()

	cfg := newTestConfig()
	c := New(cfg)
	resp, err := c.GetSimple(ts.URL)
	if err != nil {
		t.Fatalf("GetSimple: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestGet_StealthHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stealth mode should send browser-like headers.
		if r.Header.Get("Accept") == "" {
			t.Error("Accept header missing in stealth mode")
		}
		if r.Header.Get("Accept-Language") == "" {
			t.Error("Accept-Language header missing")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header missing")
		}
		fmt.Fprintln(w, "ok")
	}))
	defer ts.Close()

	cfg := newStealthConfig()
	c := New(cfg)
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
}

func TestGet_NonStealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Non-stealth should only have User-Agent.
		if r.Header.Get("Accept") != "" {
			t.Error("Accept header should be absent in non-stealth mode")
		}
		if r.Header.Get("Sec-Fetch-Dest") != "" {
			t.Error("Sec-Fetch-Dest should be absent")
		}
		fmt.Fprintln(w, "ok")
	}))
	defer ts.Close()

	cfg := newTestConfig()
	cfg.StealthMode = false
	c := New(cfg)
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
}

func TestGetBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "exact content here")
	}))
	defer ts.Close()

	cfg := newTestConfig()
	c := New(cfg)
	body, err := c.GetBody(ts.URL)
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if string(body) != "exact content here" {
		t.Errorf("body = %q, want %q", string(body), "exact content here")
	}
}

func TestGetBody_SizeLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 1KB of data.
		fmt.Fprint(w, strings.Repeat("x", 1024))
	}))
	defer ts.Close()

	cfg := newTestConfig()
	cfg.MaxContentSize = 10 // Only read first 10 bytes.
	c := New(cfg)
	body, err := c.GetBody(ts.URL)
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}
	if len(body) != 10 {
		t.Errorf("body len = %d, want 10 (size limited)", len(body))
	}
}

func TestGetSimple_Error(t *testing.T) {
	cfg := newTestConfig()
	c := New(cfg)
	// Connect to a port that nothing is listening on.
	_, err := c.GetSimple("http://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Error("expected error for refused connection, got nil")
	}
}

func TestLastURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	}))
	defer ts.Close()

	cfg := newTestConfig()
	c := New(cfg)

	if c.LastURL() != "" {
		t.Errorf("initial LastURL = %q, want empty", c.LastURL())
	}

	c.GetSimple(ts.URL)
	if c.LastURL() != ts.URL {
		t.Errorf("LastURL = %q, want %q", c.LastURL(), ts.URL)
	}
}

func TestTimeout(t *testing.T) {
	cfg := newTestConfig()
	cfg.HTTPTimeout = 5 * time.Second
	c := New(cfg)
	if c.Timeout() != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", c.Timeout())
	}
}

func TestCookies(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123"})
		fmt.Fprintln(w, "ok")
	}))
	defer ts.Close()

	cfg := newTestConfig()
	c := New(cfg)

	// Make a request that sets a cookie.
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	cookies := c.Cookies(u)
	if len(cookies) == 0 {
		t.Error("expected cookies, got none")
	}
}

func TestDo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	}))
	defer ts.Close()

	cfg := newTestConfig()
	c := New(cfg)

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRotateUA(t *testing.T) {
	ua := rotateUA()
	if ua == "" {
		t.Error("rotateUA returned empty string")
	}
	// Check that the returned UA is from the pool.
	found := false
	for _, known := range userAgents {
		if ua == known {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("rotateUA returned unknown UA: %q", ua)
	}
}
