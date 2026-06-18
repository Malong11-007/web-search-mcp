package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the server, loaded from environment variables.
type Config struct {
	// SearXNG
	SearXNGURL     string
	SearXNGTimeout time.Duration

	// Search backends (comma-separated, tried in order: duckduckgo,searxng)
	SearchBackends []string

	// Tool groups to enable (comma-separated: search,scrape,batch,crawl,map,research)
	Tools []string

	// HTTP client
	HTTPTimeout    time.Duration
	UserAgent      string
	MaxContentSize int64

	// Stealth / anti-blocking
	StealthMode    bool
	RespectRobotsTXT bool

	// Rate limiting
	RateLimit string

	// Retry
	RetryMaxAttempts   int
	RetryInitialDelay  time.Duration
	RetryMaxDelay      time.Duration
	RetryBackoffFactor float64

	// Crawl defaults
	CrawlMaxDepth int
	CrawlMaxPages int

	// Privacy
	RedactPII bool

	// Transport
	Transport string
	HTTPPort  int
}

// Load reads configuration from environment variables, applying defaults.
func Load() *Config {
	return &Config{
		SearXNGURL:     envStr("SEARXNG_URL", "https://searx.be"),
		SearXNGTimeout: envDuration("SEARXNG_TIMEOUT", 10*time.Second),

		SearchBackends: envStrSlice("SEARCH_BACKENDS", []string{"duckduckgo", "searxng"}),
		Tools:          envStrSlice("TOOLS", []string{"search", "scrape", "batch", "crawl", "map", "research"}),

		HTTPTimeout:    envDuration("HTTP_TIMEOUT", 30*time.Second),
		UserAgent:      envStr("USER_AGENT", ""), // empty = use rotation pool
		MaxContentSize: envInt64("MAX_CONTENT_SIZE", 10*1024*1024), // 10MB

		// Stealth / anti-blocking.
		StealthMode:      envBool("STEALTH_MODE", true),
		RespectRobotsTXT: envBool("RESPECT_ROBOTS_TXT", true),

		RateLimit: envStr("RATE_LIMIT", "100/1m"),

		RetryMaxAttempts:   envInt("RETRY_MAX_ATTEMPTS", 3),
		RetryInitialDelay:  envDuration("RETRY_INITIAL_DELAY", 1*time.Second),
		RetryMaxDelay:      envDuration("RETRY_MAX_DELAY", 30*time.Second),
		RetryBackoffFactor: envFloat("RETRY_BACKOFF_FACTOR", 2.0),

		CrawlMaxDepth: envInt("CRAWL_MAX_DEPTH", 3),
		CrawlMaxPages: envInt("CRAWL_MAX_PAGES", 50),

		RedactPII: envBool("REDACT_PII", false),

		Transport: envStr("TRANSPORT", "stdio"),
		HTTPPort:  envInt("HTTP_PORT", 8080),
	}
}

// HasTool returns true if the named tool group is enabled.
func (c *Config) HasTool(name string) bool {
	for _, t := range c.Tools {
		if strings.EqualFold(t, name) {
			return true
		}
	}
	return false
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envStrSlice(key string, def []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return def
	}
	return result
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
