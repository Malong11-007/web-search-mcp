package robots

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Malong11-007/web-search-mcp/internal/httpclient"
)

// Rule represents a single robots.txt rule.
type Rule struct {
	UserAgents []string
	Allow      []string
	Disallow   []string
	CrawlDelay time.Duration
}

// Cache stores parsed robots.txt rules per domain.
type Cache struct {
	mu    sync.RWMutex
	rules map[string]*cachedRules
}

type cachedRules struct {
	rules     *Rule
	expiresAt time.Time
}

// NewCache creates a robots.txt cache with the given TTL.
func NewCache() *Cache {
	return &Cache{
		rules: make(map[string]*cachedRules),
	}
}

// Allowed checks whether the given URL is allowed by robots.txt rules for the
// provided user-agent. Returns true if:
//   - The domain has no cached robots.txt (not yet fetched)
//   - The robots.txt has no matching rules
//   - The path is explicitly allowed or not disallowed
func (c *Cache) Allowed(pageURL, userAgent string) bool {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return true // can't parse, allow
	}
	host := parsed.Host

	c.mu.RLock()
	cached, ok := c.rules[host]
	c.mu.RUnlock()

	if !ok || cached == nil || cached.rules == nil {
		return true // no rules cached, allow (caller should fetch)
	}

	if time.Now().After(cached.expiresAt) {
		return true // expired, allow (caller should re-fetch)
	}

	rule := cached.rules
	path := parsed.Path
	if path == "" {
		path = "/"
	}

	// Check if this rule applies to our user-agent.
	if !matchesUserAgent(userAgent, rule.UserAgents) {
		return true // rules don't apply to us
	}

	// Check explicit allows first (they override disallows).
	for _, allow := range rule.Allow {
		if pathMatches(path, allow) {
			return true
		}
	}

	// Check disallows.
	for _, disallow := range rule.Disallow {
		if pathMatches(path, disallow) {
			return false
		}
	}

	return true
}

// CrawlDelay returns the Crawl-delay for the domain, or 0 if not set.
func (c *Cache) CrawlDelay(pageURL string) time.Duration {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return 0
	}
	host := parsed.Host

	c.mu.RLock()
	cached, ok := c.rules[host]
	c.mu.RUnlock()

	if !ok || cached == nil || cached.rules == nil {
		return 0
	}
	return cached.rules.CrawlDelay
}

// Store parses and caches robots.txt content for a domain.
func (c *Cache) Store(domain, content string, ttl time.Duration) {
	rule := parseRobotsTXT(content)

	c.mu.Lock()
	c.rules[domain] = &cachedRules{
		rules:     rule,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// IsKnown returns true if we have cached rules for the domain.
func (c *Cache) IsKnown(host string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cached, ok := c.rules[host]
	return ok && cached != nil && cached.rules != nil && time.Now().Before(cached.expiresAt)
}

// parseRobotsTXT parses a robots.txt file into a Rule.
// This is a simplified parser that handles the most common directives.
func parseRobotsTXT(content string) *Rule {
	rule := &Rule{}
	var currentUA string
	var collecting bool

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Remove comments.
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		directive := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch strings.ToLower(directive) {
		case "user-agent":
			currentUA = strings.ToLower(value)
			if currentUA == "*" || strings.Contains(currentUA, "websearch") || strings.Contains(currentUA, "bot") {
				collecting = true
			} else {
				collecting = false
			}
			rule.UserAgents = append(rule.UserAgents, strings.ToLower(value))

		case "allow":
			if collecting {
				rule.Allow = append(rule.Allow, value)
			}

		case "disallow":
			if collecting {
				rule.Disallow = append(rule.Disallow, value)
			}

		case "crawl-delay":
			if collecting {
				if d, err := time.ParseDuration(fmt.Sprintf("%ss", value)); err == nil {
					rule.CrawlDelay = d
				}
			}

		case "sitemap":
			// Not parsed for now, but recognized.
		}
	}

	return rule
}

// matchesUserAgent checks if our UA matches any of the rule's user-agents.
func matchesUserAgent(ourUA string, ruleUAs []string) bool {
	if len(ruleUAs) == 0 {
		return true // empty = applies to all
	}
	ourLower := strings.ToLower(ourUA)
	for _, ua := range ruleUAs {
		ua = strings.ToLower(ua)
		if ua == "*" {
			return true
		}
		// Check if our UA contains the rule's UA (e.g., "bot" matches "mybot/1.0").
		if strings.Contains(ourLower, ua) || strings.Contains(ua, ourLower) {
			return true
		}
	}
	return false
}

// pathMatches checks if a URL path matches a robots.txt pattern.
// The pattern can contain * (any sequence) and $ (end of path).
func pathMatches(path, pattern string) bool {
	if pattern == "" {
		return false
	}

	// Escape regex special chars except * and $.
	regex := "^"
	for _, ch := range pattern {
		switch ch {
		case '*':
			regex += ".*"
		case '$':
			regex += "$"
		case '.', '?', '+', '(', ')', '[', ']', '{', '}', '\\', '|', '^':
			regex += "\\" + string(ch)
		default:
			regex += string(ch)
		}
	}
	// If no $ at end, allow any suffix.
	if !strings.HasSuffix(pattern, "$") {
		regex += ".*"
	}
	regex += "$"

	// Simple prefix matching for common patterns (avoiding regex import).
	if pattern == "/" {
		return path == "/"
	}

	// For simple prefix patterns (no wildcards), use strings.HasPrefix.
	if !strings.ContainsAny(pattern, "*$") {
		return strings.HasPrefix(path, pattern)
	}

	// Fallback: use a simple glob match.
	return globMatch(path, pattern)
}

// globMatch implements simplified glob matching for robots.txt patterns.
func globMatch(s, pattern string) bool {
	// Simple recursive glob match.
	if pattern == "" {
		return s == ""
	}
	if pattern == "*" {
		return true
	}

	// End anchor.
	endAnchor := strings.HasSuffix(pattern, "$")
	if endAnchor {
		pattern = pattern[:len(pattern)-1]
	}

	if !strings.Contains(pattern, "*") {
		if endAnchor {
			return s == pattern
		}
		return strings.HasPrefix(s, pattern)
	}

	parts := strings.Split(pattern, "*")
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(s, part)
		if idx < 0 {
			return false
		}
		if i == 0 && !strings.HasPrefix(s, part) {
			return false
		}
		if i == len(parts)-1 && endAnchor && !strings.HasSuffix(s, part) {
			return false
		}
		s = s[idx+len(part):]
	}
	return true
}

// DefaultCache is a package-level cache for robots.txt rules with a 1-hour TTL.
var DefaultCache = NewCache()
var robotsTTL = 1 * time.Hour

// FetchAndCache fetches robots.txt for the given domain, parses it, and caches the rules.
// Returns an error only if the fetch itself fails (not if robots.txt doesn't exist — 404 is treated as "allow all").
func FetchAndCache(client *httpclient.Client, pageURL string) error {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return err
	}
	host := parsed.Host

	// Already cached and fresh?
	if DefaultCache.IsKnown(host) {
		return nil
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsed.Scheme, host)
	body, err := client.GetBody(robotsURL)
	if err != nil {
		// Can't fetch — allow all.
		DefaultCache.Store(host, "", robotsTTL)
		return nil
	}

	DefaultCache.Store(host, string(body), robotsTTL)
	return nil
}

// CheckBeforeScrape checks robots.txt for a URL. Fetches if not cached.
// Returns true if scraping is permitted for the client's user-agent.
func CheckBeforeScrape(client *httpclient.Client, pageURL string) bool {
	FetchAndCache(client, pageURL)
	return DefaultCache.Allowed(pageURL, "")
}

// GetCrawlDelay returns the cached Crawl-delay for a domain, or 0.
func GetCrawlDelay(pageURL string) time.Duration {
	return DefaultCache.CrawlDelay(pageURL)
}
