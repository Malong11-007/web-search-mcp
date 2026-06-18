package pii

import "regexp"

// Patterns for common PII. These are deliberately conservative to avoid false positives.
var patterns = []struct {
	name    string
	pattern *regexp.Regexp
	replace string
}{
	{name: "email", pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), replace: "[email]"},
	{name: "phone_us", pattern: regexp.MustCompile(`\+?1?[ .\-]?\(?\d{3}\)?[ .\-]?\d{3}[ .\-]?\d{4}`), replace: "[phone]"},
	{name: "ssn", pattern: regexp.MustCompile(`\b\d{3}[ -]\d{2}[ -]\d{4}\b`), replace: "[ssn]"},
	{name: "credit_card", pattern: regexp.MustCompile(`\b(?:\d{4}[ -]?){3}\d{4}\b`), replace: "[credit-card]"},
	{name: "ipv4", pattern: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), replace: "[ip]"},
}

// Redact replaces common PII patterns in text with placeholders.
func Redact(text string) string {
	for _, p := range patterns {
		text = p.pattern.ReplaceAllString(text, p.replace)
	}
	return text
}

// Stats returns counts of each PII type found.
func Stats(text string) map[string]int {
	counts := make(map[string]int)
	for _, p := range patterns {
		matches := p.pattern.FindAllString(text, -1)
		if len(matches) > 0 {
			counts[p.name] = len(matches)
		}
	}
	return counts
}
