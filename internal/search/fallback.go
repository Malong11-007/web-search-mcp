package search

import (
	"context"
	"fmt"
)

// FallbackBackend tries each backend in order and returns the first successful result.
type FallbackBackend struct {
	backends []Backend
}

// NewFallback creates a Backend that tries each backend in order.
func NewFallback(backends ...Backend) *FallbackBackend {
	return &FallbackBackend{backends: backends}
}

// Search tries each backend in order, returning the first successful result set.
func (f *FallbackBackend) Search(ctx context.Context, query string, numResults int) ([]Result, error) {
	var lastErr error
	for i, b := range f.backends {
		results, err := b.Search(ctx, query, numResults)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		lastErr = fmt.Errorf("backend %d: %w", i, err)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all backends returned empty results")
}

// Ensure FallbackBackend implements Backend.
var _ Backend = (*FallbackBackend)(nil)
