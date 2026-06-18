package httpclient

import "math/rand"

// userAgents is a pool of realistic browser User-Agent strings.
// Updated June 2026 — Chrome 130+, Firefox 130+, Safari 18+ on macOS/Windows/Linux.
var userAgents = []string{
	// Chrome 133 on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	// Chrome 133 on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	// Chrome 133 on Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	// Firefox 134 on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:134.0) Gecko/20100101 Firefox/134.0",
	// Firefox 134 on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:134.0) Gecko/20100101 Firefox/134.0",
	// Firefox 134 on Linux
	"Mozilla/5.0 (X11; Linux i686; rv:134.0) Gecko/20100101 Firefox/134.0",
	// Safari 18 on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Safari/605.1.15",
	// Edge 133 on Windows (Chromium-based)
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0",
}

// rotateUA returns a random User-Agent from the pool.
func rotateUA() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// browserHeaders returns headers that a real browser would send.
// These are headers Chrome sends on a typical GET request.
func browserHeaders(ua string) map[string]string {
	return map[string]string{
		"User-Agent":      ua,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
		"Sec-Fetch-Dest":  "document",
		"Sec-Fetch-Mode":  "navigate",
		"Sec-Fetch-Site":  "none",
		"Sec-Fetch-User":  "?1",
		"Upgrade-Insecure-Requests": "1",
		"Cache-Control":   "max-age=0",
		"Connection":      "keep-alive",
	}
}

// referrerFor returns a plausible referrer for a given URL, or empty string.
func referrerFor(targetURL string) string {
	// No referrer by default — callers set it based on their navigation context.
	return ""
}
