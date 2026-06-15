// Package forwarder sends DNS queries to upstream resolvers with health-aware
// failover. The primary upstream can be swapped at runtime; fallback upstreams
// are preserved.
package forwarder

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Upstream describes a single upstream resolver.
type Upstream struct {
	Addr    string
	Timeout time.Duration
	TLS     bool
}

func (u Upstream) net() string {
	if u.TLS {
		return "tcp-tls"
	}
	return "udp"
}

// Pool forwards queries across a set of upstreams, skipping unhealthy ones.
type Pool struct {
	mu        sync.RWMutex
	upstreams []Upstream
	healthy   map[int]bool

	stop chan struct{}
}

// NewPool builds a forwarder pool. An empty upstream list falls back to a
// sane default. A background health loop is started; call Close to stop it.
func NewPool(upstreams []Upstream) *Pool {
	if len(upstreams) == 0 {
		upstreams = []Upstream{{Addr: "1.1.1.1:53", Timeout: 3 * time.Second}}
	}
	p := &Pool{
		upstreams: upstreams,
		healthy:   make(map[int]bool, len(upstreams)),
		stop:      make(chan struct{}),
	}
	for i := range upstreams {
		p.healthy[i] = true
	}
	go p.healthLoop()
	return p
}

// Close stops the background health loop.
func (p *Pool) Close() {
	close(p.stop)
}

// SetPrimary replaces the first upstream (the user-selected provider) at
// runtime, preserving any configured fallback upstreams.
func (p *Pool) SetPrimary(addr string, tls bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	up := Upstream{Addr: addr, Timeout: 4 * time.Second, TLS: tls}
	if len(p.upstreams) == 0 {
		p.upstreams = []Upstream{up}
	} else {
		p.upstreams[0] = up
	}
	p.healthy[0] = true
	slog.Info("upstream changed", "addr", addr, "tls", tls)
}

// Current returns the address of the first healthy upstream, for logging.
func (p *Pool) Current() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i, up := range p.upstreams {
		if p.healthy[i] {
			return up.Addr
		}
	}
	if len(p.upstreams) > 0 {
		return p.upstreams[0].Addr
	}
	return ""
}

// Forward tries each healthy upstream in order, returning the first success.
// An upstream that errors is marked unhealthy until the next health probe.
func (p *Pool) Forward(req *dns.Msg) (*dns.Msg, error) {
	var lastErr error
	for i := range p.snapshot() {
		p.mu.RLock()
		up, ok := p.upstreams[i], p.healthy[i]
		p.mu.RUnlock()
		if !ok {
			continue
		}

		client := &dns.Client{Net: up.net(), Timeout: up.Timeout}
		resp, _, err := client.Exchange(req, up.Addr)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		slog.Warn("upstream failure", "addr", up.Addr, "error", err)

		p.mu.Lock()
		p.healthy[i] = false
		p.mu.Unlock()
	}
	if lastErr == nil {
		return nil, fmt.Errorf("no upstreams configured")
	}
	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

// snapshot returns the current upstream slice length-equivalent for ranging.
func (p *Pool) snapshot() []Upstream {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Upstream, len(p.upstreams))
	copy(out, p.upstreams)
	return out
}
