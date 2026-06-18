package httpclient

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/dan/web-search-mcp/internal/config"
)

// Client wraps net/http.Client with stealth anti-blocking features:
//   - User-Agent rotation (pool of 8 realistic browser UAs)
//   - Browser-like headers (Accept, Accept-Language, Sec-*, etc.)
//   - Per-domain cookie jar (session persistence)
//   - Referrer tracking
//   - Configurable timeout and size cap
//
// Set STEALTH_MODE=false to disable browser-like behavior and use a static UA.
type Client struct {
	http        *http.Client
	cfg         *config.Config
	ua          string       // static UA, or empty to rotate
	cookieJar   *cookiejar.Jar
	lastURL     string       // for referrer tracking
}

// New creates a stealth Client from the given configuration.
func New(cfg *config.Config) *Client {
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	c := &Client{
		http: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: transport,
			Jar:       jar,
		},
		cfg:       cfg,
		cookieJar: jar,
	}

	// If a custom UA is set, use it. Otherwise rotate.
	if cfg.UserAgent != "" {
		c.ua = cfg.UserAgent
	}

	return c
}

// GetSimple performs a GET request with only User-Agent set (no stealth headers).
// Use this for search APIs and other endpoints that don't need browser emulation.
func (c *Client) GetSimple(targetURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	ua := c.ua
	if ua == "" {
		ua = rotateUA()
	}
	req.Header.Set("User-Agent", ua)
	c.lastURL = targetURL
	return c.http.Do(req)
}

// Get performs a GET request with stealth headers.
// The caller is responsible for closing resp.Body.
func (c *Client) Get(targetURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, targetURL)
	c.lastURL = targetURL
	return c.http.Do(req)
}

// GetBody performs a GET request and returns the full body, capped at MaxContentSize.
func (c *Client) GetBody(targetURL string) ([]byte, error) {
	resp, err := c.Get(targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, c.cfg.MaxContentSize)
	return io.ReadAll(limited)
}

// Do sends the given request after adding stealth headers.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.setHeaders(req, req.URL.String())
	c.lastURL = req.URL.String()
	return c.http.Do(req)
}

// Timeout returns the configured HTTP timeout.
func (c *Client) Timeout() time.Duration {
	return c.cfg.HTTPTimeout
}

// LastURL returns the most recently requested URL (for referrer tracking).
func (c *Client) LastURL() string {
	return c.lastURL
}

// Cookies returns the cookies stored for the given URL.
func (c *Client) Cookies(u *url.URL) []*http.Cookie {
	return c.cookieJar.Cookies(u)
}

// setHeaders applies stealth or static headers to the request.
func (c *Client) setHeaders(req *http.Request, targetURL string) {
	if c.cfg.StealthMode {
		ua := c.ua
		if ua == "" {
			ua = rotateUA()
		}
		for k, v := range browserHeaders(ua) {
			req.Header.Set(k, v)
		}
		// Set referrer from last URL.
		if c.lastURL != "" {
			req.Header.Set("Referer", c.lastURL)
		}
		// Set Sec-Fetch-Site based on whether this is same-origin.
		if c.lastURL != "" {
			lastParsed, _ := url.Parse(c.lastURL)
			thisParsed, _ := url.Parse(targetURL)
			if lastParsed != nil && thisParsed != nil && lastParsed.Host == thisParsed.Host {
				req.Header.Set("Sec-Fetch-Site", "same-origin")
			} else {
				req.Header.Set("Sec-Fetch-Site", "cross-site")
			}
		}
	} else {
		ua := c.ua
		if ua == "" {
			ua = "WebSearchMCP/1.0"
		}
		req.Header.Set("User-Agent", ua)
	}
}
