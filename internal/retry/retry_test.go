package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	err := Do(ctx, 3, 1*time.Millisecond, 10*time.Millisecond, 2.0, func() error {
		attempts++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_RetryAndSucceed(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	err := Do(ctx, 3, 1*time.Millisecond, 10*time.Millisecond, 2.0, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_AllFail(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("fatal")
	err := Do(ctx, 3, 1*time.Millisecond, 10*time.Millisecond, 2.0, func() error {
		return expectedErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestDo_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	err := Do(ctx, 5, 100*time.Millisecond, 1*time.Second, 2.0, func() error {
		cancel()
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDo_MaxDelay(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	_ = Do(ctx, 3, 50*time.Millisecond, 10*time.Millisecond, 2.0, func() error {
		return errors.New("fail")
	})
	elapsed := time.Since(start)
	// With maxDelay 10ms and 3 attempts, max sleep is 2*10ms = 20ms. Should be well under 100ms.
	if elapsed > 200*time.Millisecond {
		t.Errorf("took too long (%v); max delay should have been capped", elapsed)
	}
}
