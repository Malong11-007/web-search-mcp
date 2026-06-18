package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Limiter is a simple token-bucket rate limiter.
type Limiter struct {
	interval time.Duration
	tokens   int
	burst    int
	ticker   *time.Ticker
	ch       chan struct{}
}

// New creates a Limiter from a rate string like "100/1m" (100 requests per minute).
// Supported units: s (second), m (minute), h (hour).
func New(rate string) (*Limiter, error) {
	tokens, interval, err := parseRate(rate)
	if err != nil {
		return nil, err
	}

	// Refill interval: divide the window evenly by token count.
	refillInterval := interval / time.Duration(tokens)
	if refillInterval < time.Millisecond {
		refillInterval = time.Millisecond
	}

	l := &Limiter{
		interval: interval,
		tokens:   tokens,
		burst:    tokens, // allow full burst at start
		ticker:   time.NewTicker(refillInterval),
		ch:       make(chan struct{}, tokens),
	}

	// Pre-fill the bucket.
	for i := 0; i < tokens; i++ {
		l.ch <- struct{}{}
	}

	// Refill loop.
	go func() {
		for range l.ticker.C {
			select {
			case l.ch <- struct{}{}:
			default:
				// bucket full
			}
		}
	}()

	return l, nil
}

// Wait blocks until a token is available or ctx is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.ch:
		return nil
	}
}

// NewNoop returns a Limiter that never blocks — useful for testing.
func NewNoop() *Limiter {
	l := &Limiter{
		ch: make(chan struct{}, 1),
	}
	l.ch <- struct{}{}
	return l
}

// Close stops the refill ticker. The Limiter should not be used after Close.
func (l *Limiter) Close() {
	if l.ticker != nil {
		l.ticker.Stop()
	}
}

// parseRate parses a rate string like "100/1m" into token count and interval.
func parseRate(rate string) (int, time.Duration, error) {
	parts := strings.SplitN(rate, "/", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid rate format %q: expected N/unit", rate)
	}

	n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid rate count %q: %w", parts[0], err)
	}
	if n <= 0 {
		return 0, 0, fmt.Errorf("rate count must be positive, got %d", n)
	}

	durStr := strings.TrimSpace(parts[1])

	// Support shorthand: "s", "m", "h" — prepend "1" if the string starts with a letter.
	if len(durStr) > 0 && durStr[0] >= 'a' && durStr[0] <= 'z' {
		durStr = "1" + durStr
	}

	if len(durStr) < 2 {
		return 0, 0, fmt.Errorf("invalid rate duration %q", parts[1])
	}

	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid rate duration %q: %w", parts[1], err)
	}

	return n, dur, nil
}
