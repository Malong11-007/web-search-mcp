package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	for _, k := range []string{
		"SEARXNG_URL", "SEARXNG_TIMEOUT", "HTTP_TIMEOUT", "USER_AGENT",
		"MAX_CONTENT_SIZE", "RATE_LIMIT", "RETRY_MAX_ATTEMPTS", "RETRY_INITIAL_DELAY",
		"RETRY_MAX_DELAY", "RETRY_BACKOFF_FACTOR", "TRANSPORT", "HTTP_PORT",
		"STEALTH_MODE", "RESPECT_ROBOTS_TXT",
	} {
		os.Unsetenv(k)
	}

	cfg := Load()

	if cfg.SearXNGURL != "https://searx.be" {
		t.Errorf("SearXNGURL = %q, want %q", cfg.SearXNGURL, "https://searx.be")
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout = %v, want 30s", cfg.HTTPTimeout)
	}
	if cfg.UserAgent != "" {
		t.Errorf("UserAgent = %q, want empty (use rotation pool)", cfg.UserAgent)
	}
	if !cfg.StealthMode {
		t.Error("StealthMode should default to true")
	}
	if !cfg.RespectRobotsTXT {
		t.Error("RespectRobotsTXT should default to true")
	}
	if cfg.MaxContentSize != 10*1024*1024 {
		t.Errorf("MaxContentSize = %d, want %d", cfg.MaxContentSize, 10*1024*1024)
	}
	if cfg.RateLimit != "100/1m" {
		t.Errorf("RateLimit = %q, want %q", cfg.RateLimit, "100/1m")
	}
	if cfg.RetryMaxAttempts != 3 {
		t.Errorf("RetryMaxAttempts = %d, want 3", cfg.RetryMaxAttempts)
	}
	if cfg.RetryInitialDelay != 1*time.Second {
		t.Errorf("RetryInitialDelay = %v, want 1s", cfg.RetryInitialDelay)
	}
	if cfg.RetryMaxDelay != 30*time.Second {
		t.Errorf("RetryMaxDelay = %v, want 30s", cfg.RetryMaxDelay)
	}
	if cfg.RetryBackoffFactor != 2.0 {
		t.Errorf("RetryBackoffFactor = %f, want 2.0", cfg.RetryBackoffFactor)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport = %q, want %q", cfg.Transport, "stdio")
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("RATE_LIMIT", "50/30s")
	os.Setenv("RETRY_MAX_ATTEMPTS", "5")
	os.Setenv("STEALTH_MODE", "false")
	defer func() {
		os.Unsetenv("RATE_LIMIT")
		os.Unsetenv("RETRY_MAX_ATTEMPTS")
		os.Unsetenv("STEALTH_MODE")
	}()

	cfg := Load()

	if cfg.RateLimit != "50/30s" {
		t.Errorf("RateLimit = %q, want %q", cfg.RateLimit, "50/30s")
	}
	if cfg.RetryMaxAttempts != 5 {
		t.Errorf("RetryMaxAttempts = %d, want 5", cfg.RetryMaxAttempts)
	}
	if cfg.StealthMode {
		t.Error("StealthMode should be false when set to 'false'")
	}
}

func TestLoad_DurationParsing(t *testing.T) {
	os.Setenv("HTTP_TIMEOUT", "5s")
	os.Setenv("RETRY_INITIAL_DELAY", "500ms")
	defer func() {
		os.Unsetenv("HTTP_TIMEOUT")
		os.Unsetenv("RETRY_INITIAL_DELAY")
	}()

	cfg := Load()

	if cfg.HTTPTimeout != 5*time.Second {
		t.Errorf("HTTPTimeout = %v, want 5s", cfg.HTTPTimeout)
	}
	if cfg.RetryInitialDelay != 500*time.Millisecond {
		t.Errorf("RetryInitialDelay = %v, want 500ms", cfg.RetryInitialDelay)
	}
}

func TestLoad_ToolGroups(t *testing.T) {
	os.Setenv("TOOLS", "search,scrape")
	defer os.Unsetenv("TOOLS")

	cfg := Load()

	if !cfg.HasTool("search") {
		t.Error("expected search to be enabled")
	}
	if !cfg.HasTool("scrape") {
		t.Error("expected scrape to be enabled")
	}
	if cfg.HasTool("crawl") {
		t.Error("crawl should not be enabled")
	}
}

func TestLoad_DefaultToolGroups(t *testing.T) {
	os.Unsetenv("TOOLS")
	cfg := Load()

	for _, name := range []string{"search", "scrape", "batch", "crawl", "map", "research"} {
		if !cfg.HasTool(name) {
			t.Errorf("expected %q to be enabled by default", name)
		}
	}
}
