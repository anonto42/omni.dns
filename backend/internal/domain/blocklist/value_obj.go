package blocklist

import (
	"errors"
	"strings"
)

const maxDomainLen = 253

var ErrInvalidDomain = errors.New("invalid domain name")

// Domain is an immutable value object representing a validated, normalized
// domain name for a blocklist rule. A leading "*." wildcard prefix is permitted.
type Domain struct {
	value string
}

// NewDomain normalizes and validates a raw domain string.
func NewDomain(raw string) (Domain, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if !validDomain(v) {
		return Domain{}, ErrInvalidDomain
	}
	return Domain{value: v}, nil
}

// String returns the normalized domain value.
func (d Domain) String() string { return d.value }

func validDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > maxDomainLen {
		return false
	}

	d := strings.TrimPrefix(domain, "*.")
	if len(d) == 0 {
		return false
	}

	for _, c := range d {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
