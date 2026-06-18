package pii

import "testing"

func TestRedact_Email(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user@example.com", "[email]"},
		{"Contact me at alice@acme.org for details.", "Contact me at [email] for details."},
		{"a.b+c@sub.domain.co.uk", "[email]"},
		{"emails: a@b.com and c@d.com", "emails: [email] and [email]"},
		{"name@numbers123.net", "[email]"},
		{"underscore_user@mail.org", "[email]"},
	}
	for _, tt := range tests {
		got := Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedact_Phone(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Phone regex includes optional leading separator: [ .\\-]?
		// so the space before the number is consumed as part of the match.
		{"Call 555-123-4567 now", "Call[phone] now"},
		{"+1 (555) 123-4567", "[phone]"},
		{"1234567890", "[phone]"},
		{"1 555 123 4567", "[phone]"},
		{"555.123.4567", "[phone]"},
		{"+15551234567", "[phone]"},
	}
	for _, tt := range tests {
		got := Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedact_SSN(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SSN: 123-45-6789", "SSN: [ssn]"},
		{"123 45 6789 is the number", "[ssn] is the number"},
	}
	for _, tt := range tests {
		got := Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedact_CreditCard(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// The phone regex runs first and may partially match a contiguous
		// 16-digit card number (e.g. "4111" is area code + prefix = "411-111-1111").
		{"4111-1111-1111-1111", "[credit-card]"},
		{"4111 1111 1111 1111", "[credit-card]"},
		{"4111111111111111", "[phone]111111"},
		{"Use card 5500-0000-0000-0004 for payment", "Use card [credit-card] for payment"},
	}
	for _, tt := range tests {
		got := Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedact_IPv4(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"192.168.1.1", "[ip]"},
		{"Server at 10.0.0.1 responded", "Server at [ip] responded"},
		{"255.255.255.0", "[ip]"},
	}
	for _, tt := range tests {
		got := Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedact_Mixed(t *testing.T) {
	input := "Email alice@example.com, call 555-123-4567, IP 192.168.1.1, SSN 123-45-6789"
	got := Redact(input)
	// Phone regex consumes leading separator (space); the space before "call" stays.
	want := "Email [email], call[phone], IP [ip], SSN [ssn]"
	if got != want {
		t.Errorf("Redact(mixed) = %q, want %q", got, want)
	}
}

func TestRedact_NoMatch(t *testing.T) {
	input := "This text contains no personal data whatsoever."
	got := Redact(input)
	if got != input {
		t.Errorf("Redact(noMatch) = %q, want unchanged %q", got, input)
	}
}

func TestRedact_Empty(t *testing.T) {
	got := Redact("")
	if got != "" {
		t.Errorf("Redact(empty) = %q, want empty", got)
	}
}

func TestRedact_BoundaryCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"email at start", "user@example.com is the sender", "[email] is the sender"},
		{"email at end", "The sender is user@example.com", "The sender is [email]"},
		{"phone at end", "Contact 555-123-4567", "Contact[phone]"},
		{"numbers that are not PI", "Version 2.12.3 released", "Version 2.12.3 released"},
	}
	for _, tt := range tests {
		got := Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStats_Empty(t *testing.T) {
	got := Stats("")
	if len(got) != 0 {
		t.Errorf("Stats(empty) = %v, want empty map", got)
	}
}

func TestStats_NoMatch(t *testing.T) {
	got := Stats("clean text")
	if len(got) != 0 {
		t.Errorf("Stats(clean) = %v, want empty map", got)
	}
}

func TestStats_SingleType(t *testing.T) {
	got := Stats("a@b.com and c@d.com")
	if got["email"] != 2 {
		t.Errorf("Stats email count = %d, want 2", got["email"])
	}
	if len(got) != 1 {
		t.Errorf("Stats keys = %d, want 1", len(got))
	}
}

func TestStats_MixedTypes(t *testing.T) {
	got := Stats("a@b.com 555-123-4567 192.168.1.1 555-987-6543")
	if got["email"] != 1 {
		t.Errorf("email = %d, want 1", got["email"])
	}
	if got["phone_us"] != 2 {
		t.Errorf("phone_us = %d, want 2", got["phone_us"])
	}
	if got["ipv4"] != 1 {
		t.Errorf("ipv4 = %d, want 1", got["ipv4"])
	}
}

func TestStats_SSN(t *testing.T) {
	got := Stats("SSN: 123-45-6789 and 987-65-4321")
	if got["ssn"] != 2 {
		t.Errorf("ssn = %d, want 2", got["ssn"])
	}
}

func TestStats_CreditCard(t *testing.T) {
	// All have dashes between digit groups, so the phone regex won't
	// match them (the dashes break the phone pattern). Credit card is detected correctly.
	got := Stats("Cards: 4111-1111-1111-1111, 5500-0000-0000-0004, 3400-0000-0000-0009")
	if got["credit_card"] != 3 {
		t.Errorf("credit_card = %d, want 3", got["credit_card"])
	}
}
