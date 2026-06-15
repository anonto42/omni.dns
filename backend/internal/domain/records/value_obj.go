package records

import (
	"errors"
	"net"
	"strings"
)

const maxDomainLen = 253

// DNS query type numbers used by custom records.
const (
	TypeA    uint16 = 1
	TypeAAAA uint16 = 28
)

var (
	ErrInvalidDomain = errors.New("invalid domain name")
	ErrInvalidIP     = errors.New("invalid IP address")
)

// Domain is an immutable value object representing a validated, normalized
// DNS domain name. A leading "*." wildcard prefix is permitted.
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

// IP is an immutable value object representing a validated IP address. It
// remembers whether the address is IPv4 (A) or IPv6 (AAAA).
type IP struct {
	value string
	isV4  bool
}

// NewIP validates a raw IP string (IPv4 or IPv6).
func NewIP(raw string) (IP, error) {
	v := strings.TrimSpace(raw)
	parsed := net.ParseIP(v)
	if parsed == nil {
		return IP{}, ErrInvalidIP
	}
	return IP{value: v, isV4: parsed.To4() != nil}, nil
}

// String returns the IP value.
func (ip IP) String() string { return ip.value }

// QType returns the DNS record type implied by the address family.
func (ip IP) QType() uint16 {
	if ip.isV4 {
		return TypeA
	}
	return TypeAAAA
}
