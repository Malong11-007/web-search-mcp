package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// Do calls fn up to maxAttempts times with exponential backoff + jitter.
// Jitter randomizes the delay (0 to calculated_delay) to avoid synchronized
// retry patterns that trigger rate-limiters and bot detection.
//
// Returns immediately if ctx is cancelled, or if fn returns nil.
// On the final attempt, fn's error is returned directly (no wrapping).
func Do(
	ctx context.Context,
	maxAttempts int,
	initialDelay time.Duration,
	maxDelay time.Duration,
	backoffFactor float64,
	fn func() error,
) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Don't sleep after the last attempt.
		if attempt == maxAttempts-1 {
			break
		}

		// Calculate exponential backoff.
		delay := time.Duration(float64(initialDelay) * math.Pow(backoffFactor, float64(attempt)))
		if delay > maxDelay {
			delay = maxDelay
		}

		// Full jitter: randomize between 0 and calculated delay.
		// This prevents thundering herd and makes retry patterns look human.
		if delay > 0 {
			delay = time.Duration(rand.Int64N(int64(delay)))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("retry: all %d attempts failed: %w", maxAttempts, lastErr)
}
