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

	if cfg.SearXNGURL != "" {
		t.Errorf("SearXNGURL = %q, want %q (empty — opt-in)", cfg.SearXNGURL, "")
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

func TestLoad_EnvParseFailures(t *testing.T) {
	// Set env vars to invalid values; verify they silently fall back to defaults.
	os.Setenv("RETRY_MAX_ATTEMPTS", "not-a-number")
	os.Setenv("RETRY_BACKOFF_FACTOR", "abc")
	os.Setenv("HTTP_TIMEOUT", "fast")
	os.Setenv("STEALTH_MODE", "maybe")
	os.Setenv("MAX_CONTENT_SIZE", "huge")
	defer func() {
		for _, k := range []string{
			"RETRY_MAX_ATTEMPTS", "RETRY_BACKOFF_FACTOR", "HTTP_TIMEOUT",
			"STEALTH_MODE", "MAX_CONTENT_SIZE",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg := Load()

	// Should fall back to defaults on parse failure.
	if cfg.RetryMaxAttempts != 3 {
		t.Errorf("RetryMaxAttempts = %d, want default 3", cfg.RetryMaxAttempts)
	}
	if cfg.RetryBackoffFactor != 2.0 {
		t.Errorf("RetryBackoffFactor = %f, want default 2.0", cfg.RetryBackoffFactor)
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout = %v, want default 30s", cfg.HTTPTimeout)
	}
	if !cfg.StealthMode {
		t.Error("StealthMode should default to true on invalid input")
	}
	if cfg.MaxContentSize != 10*1024*1024 {
		t.Errorf("MaxContentSize = %d, want default 10MB", cfg.MaxContentSize)
	}
}

func TestLoad_EnvBoolTruthy(t *testing.T) {
	// Test all truthy env bool values.
	for _, val := range []string{"1", "true", "yes", "on"} {
		os.Setenv("STEALTH_MODE", val)
		cfg := Load()
		if !cfg.StealthMode {
			t.Errorf("STEALTH_MODE=%q should set StealthMode=true", val)
		}
	}
	os.Unsetenv("STEALTH_MODE")
}

func TestLoad_EnvBoolFalsy(t *testing.T) {
	for _, val := range []string{"0", "false", "no", "off"} {
		os.Setenv("STEALTH_MODE", val)
		cfg := Load()
		if cfg.StealthMode {
			t.Errorf("STEALTH_MODE=%q should set StealthMode=false", val)
		}
	}
	os.Unsetenv("STEALTH_MODE")
}

func TestLoad_EnvInt64Parse(t *testing.T) {
	// Invalid parse falls back to default.
	os.Setenv("MAX_CONTENT_SIZE", "not-a-number")
	cfg := Load()
	if cfg.MaxContentSize != 10*1024*1024 {
		t.Errorf("MaxContentSize = %d, want default", cfg.MaxContentSize)
	}
	// Valid override.
	os.Setenv("MAX_CONTENT_SIZE", "5000")
	cfg2 := Load()
	if cfg2.MaxContentSize != 5000 {
		t.Errorf("MaxContentSize = %d, want 5000", cfg2.MaxContentSize)
	}
	os.Unsetenv("MAX_CONTENT_SIZE")
}

func TestLoad_EnvFloatParse(t *testing.T) {
	// Invalid parse falls back to default.
	os.Setenv("RETRY_BACKOFF_FACTOR", "xyz")
	cfg := Load()
	if cfg.RetryBackoffFactor != 2.0 {
		t.Errorf("RetryBackoffFactor = %f, want default 2.0", cfg.RetryBackoffFactor)
	}
	// Valid override.
	os.Setenv("RETRY_BACKOFF_FACTOR", "1.5")
	cfg2 := Load()
	if cfg2.RetryBackoffFactor != 1.5 {
		t.Errorf("RetryBackoffFactor = %f, want 1.5", cfg2.RetryBackoffFactor)
	}
	os.Unsetenv("RETRY_BACKOFF_FACTOR")
}

func TestLoad_EnvStrSliceAllEmpty(t *testing.T) {
	// When all parts are empty/whitespace, falls back to default.
	os.Setenv("SEARCH_BACKENDS", ", ,   ,")
	cfg := Load()
	if len(cfg.SearchBackends) != 2 || cfg.SearchBackends[0] != "duckduckgo" {
		t.Errorf("SearchBackends = %v, want default [duckduckgo searxng]", cfg.SearchBackends)
	}
	os.Unsetenv("SEARCH_BACKENDS")
}

func TestHasTool_CaseInsensitive(t *testing.T) {
	os.Setenv("TOOLS", "search,scrape")
	defer os.Unsetenv("TOOLS")

	cfg := Load()

	// Exact match.
	if !cfg.HasTool("search") {
		t.Error("HasTool(search) should be true")
	}
	// Case-insensitive.
	if !cfg.HasTool("Search") {
		t.Error("HasTool(Search) should be true (case-insensitive)")
	}
	if !cfg.HasTool("SEARCH") {
		t.Error("HasTool(SEARCH) should be true (case-insensitive)")
	}
	// Not configured.
	if cfg.HasTool("crawl") {
		t.Error("HasTool(crawl) should be false")
	}
	if cfg.HasTool("nonexistent") {
		t.Error("HasTool(nonexistent) should be false")
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
