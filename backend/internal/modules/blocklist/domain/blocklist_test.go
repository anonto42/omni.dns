package blocklist

import (
	"errors"
	"testing"
)

func TestNewNormalizesEntry(t *testing.T) {
	entry, err := New(" Ads.Example.COM ", true)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if got := entry.Domain().String(); got != "ads.example.com" {
		t.Fatalf("domain = %q, want %q", got, "ads.example.com")
	}
	if !entry.Wildcard() {
		t.Fatalf("wildcard = false, want true")
	}
}

func TestNewAcceptsWildcardPrefix(t *testing.T) {
	entry, err := New("*.tracker.io", false)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if got := entry.Domain().String(); got != "*.tracker.io" {
		t.Fatalf("domain = %q, want %q", got, "*.tracker.io")
	}
}

func TestNewRejectsInvalidDomain(t *testing.T) {
	tests := []struct {
		name   string
		domain string
	}{
		{name: "empty", domain: ""},
		{name: "bare wildcard", domain: "*."},
		{name: "bad character", domain: "bad/domain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.domain, false)
			if !errors.Is(err, ErrInvalidDomain) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidDomain)
			}
		})
	}
}
