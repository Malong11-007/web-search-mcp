package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestParseRate_Valid(t *testing.T) {
	tests := []struct {
		rate     string
		wantN    int
		wantDur  time.Duration
	}{
		{"100/1m", 100, time.Minute},
		{"10/1s", 10, time.Second},
		{"5/1h", 5, time.Hour},
		{"1/m", 1, time.Minute},
		{"50/s", 50, time.Second},
	}

	for _, tt := range tests {
		n, dur, err := parseRate(tt.rate)
		if err != nil {
			t.Errorf("parseRate(%q) unexpected error: %v", tt.rate, err)
			continue
		}
		if n != tt.wantN {
			t.Errorf("parseRate(%q) n = %d, want %d", tt.rate, n, tt.wantN)
		}
		if dur != tt.wantDur {
			t.Errorf("parseRate(%q) dur = %v, want %v", tt.rate, dur, tt.wantDur)
		}
	}
}

func TestParseRate_Invalid(t *testing.T) {
	invalid := []string{"", "abc", "100", "/1m", "0/1m", "-1/1m"}
	for _, rate := range invalid {
		_, _, err := parseRate(rate)
		if err == nil {
			t.Errorf("parseRate(%q) expected error, got nil", rate)
		}
	}
}

func TestLimiter_Wait(t *testing.T) {
	l, err := New("10/1s")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	ctx := context.Background()

	// Should be able to immediately acquire 10 tokens.
	for i := 0; i < 10; i++ {
		if err := l.Wait(ctx); err != nil {
			t.Fatalf("Wait #%d: %v", i, err)
		}
	}

	// The 11th should block. Use a short context to verify timeout.
	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := l.Wait(ctx2); err == nil {
		t.Error("expected timeout on 11th token, got nil")
	}
}

func TestNewNoop(t *testing.T) {
	l := NewNoop()
	ctx := context.Background()

	// Noop provides exactly one token — it never blocks on the first call.
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait on Noop limiter: %v", err)
	}
	// Close should not panic.
	l.Close()
}

func TestClose(t *testing.T) {
	l, err := New("1/1h")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Close()
	// Second Close should not panic (idempotent).
	l.Close()
	// ticker should be nil or stopped; no goroutine leaks.
}

func TestParseRate_DurationError(t *testing.T) {
	// Rate with valid count but unparseable duration.
	_, _, err := parseRate("10/xyz")
	if err == nil {
		t.Error("parseRate('10/xyz') expected error for bad duration, got nil")
	}
}

func TestParseRate_EmptyDurationPart(t *testing.T) {
	// Slash present but nothing after it.
	_, _, err := parseRate("5/")
	if err == nil {
		t.Error("parseRate('5/') expected error for empty duration, got nil")
	}
}

func TestNew_FastRate(t *testing.T) {
	// Extremely fast rate causes refill interval to be clamped at 1ms.
	l, err := New("1000/1ms") // 1000 tokens per ms → interval = 1µs, clamped to 1ms
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()
	// Should not block on first token.
	if err := l.Wait(context.Background()); err != nil {
		t.Errorf("Wait: %v", err)
	}
}

func TestParseRate_BareNumber(t *testing.T) {
	// A bare number without a slash or unit should fail.
	_, _, err := parseRate("100")
	if err == nil {
		t.Error("parseRate('100') expected error for bare number, got nil")
	}
}

func TestLimiter_ContextCancel(t *testing.T) {
	l, err := New("1/1h") // Very slow refill.
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	// Exhaust the single token.
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("first Wait: %v", err)
	}

	// Cancel immediately.
	ctx2, cancel := context.WithCancel(ctx)
	cancel()
	if err := l.Wait(ctx2); err == nil {
		t.Error("expected context.Canceled, got nil")
	}
}
