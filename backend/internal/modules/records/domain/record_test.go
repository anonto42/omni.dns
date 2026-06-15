package records

import (
	"errors"
	"testing"
)

func TestNewNormalizesAndValidatesRecord(t *testing.T) {
	record, err := New(" Example.LOCAL ", "192.168.1.10")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if got := record.Domain().String(); got != "example.local" {
		t.Fatalf("domain = %q, want %q", got, "example.local")
	}
	if got := record.IP().String(); got != "192.168.1.10" {
		t.Fatalf("ip = %q, want %q", got, "192.168.1.10")
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		ip      string
		wantErr error
	}{
		{name: "empty domain", domain: "", ip: "192.168.1.10", wantErr: ErrInvalidDomain},
		{name: "bad domain character", domain: "bad/domain", ip: "192.168.1.10", wantErr: ErrInvalidDomain},
		{name: "bad ip", domain: "example.local", ip: "not-an-ip", wantErr: ErrInvalidIP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.domain, tt.ip)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewDomainValueObject(t *testing.T) {
	d, err := NewDomain(" *.Example.COM ")
	if err != nil {
		t.Fatalf("NewDomain returned error: %v", err)
	}
	if d.String() != "*.example.com" {
		t.Fatalf("domain = %q, want %q", d.String(), "*.example.com")
	}
}
