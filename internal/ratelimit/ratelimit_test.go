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
