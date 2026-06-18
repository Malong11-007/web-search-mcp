package search

import (
	"context"
	"errors"
	"testing"
)

func TestFallback_FirstSucceeds(t *testing.T) {
	m1 := &mockBackend{results: []Result{{Title: "From first", URL: "https://a.com"}}}
	m2 := &mockBackend{results: []Result{{Title: "From second", URL: "https://b.com"}}}
	fb := NewFallback(m1, m2)

	results, err := fb.Search(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results[0].Title != "From first" {
		t.Errorf("expected 'From first', got %q", results[0].Title)
	}
}

func TestFallback_FirstFailsSecondSucceeds(t *testing.T) {
	m1 := &mockBackend{err: errors.New("backend down")}
	m2 := &mockBackend{results: []Result{{Title: "Fallback winner", URL: "https://b.com"}}}
	fb := NewFallback(m1, m2)

	results, err := fb.Search(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results[0].Title != "Fallback winner" {
		t.Errorf("expected 'Fallback winner', got %q", results[0].Title)
	}
}

func TestFallback_FirstEmptySecondSucceeds(t *testing.T) {
	m1 := &mockBackend{results: []Result{}} // empty, no error
	m2 := &mockBackend{results: []Result{{Title: "Second", URL: "https://b.com"}}}
	fb := NewFallback(m1, m2)

	results, err := fb.Search(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results[0].Title != "Second" {
		t.Errorf("expected 'Second', got %q", results[0].Title)
	}
}

func TestFallback_BothFail(t *testing.T) {
	m1 := &mockBackend{err: errors.New("err1")}
	m2 := &mockBackend{err: errors.New("err2")}
	fb := NewFallback(m1, m2)

	_, err := fb.Search(context.Background(), "q", 5)
	if err == nil {
		t.Error("expected error when both backends fail")
	}
}

func TestFallback_AllEmpty(t *testing.T) {
	m1 := &mockBackend{results: []Result{}}
	m2 := &mockBackend{results: []Result{}}
	fb := NewFallback(m1, m2)

	_, err := fb.Search(context.Background(), "q", 5)
	if err == nil {
		t.Error("expected error when all backends return empty")
	}
}

func TestFallback_SingleBackend(t *testing.T) {
	m1 := &mockBackend{results: []Result{{Title: "Only", URL: "https://a.com"}}}
	fb := NewFallback(m1)

	results, err := fb.Search(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}
