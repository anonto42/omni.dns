package dns

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Upstream struct {
	Addr    string
	Timeout time.Duration
	TLS     bool
}

type PooledForwarder struct {
	upstreams []Upstream
	mu        sync.RWMutex
	healthy   map[int]bool
}

func NewPooledForwarder(upstreams []Upstream) *PooledForwarder {
	f := &PooledForwarder{
		upstreams: upstreams,
		healthy:   make(map[int]bool),
	}

	for i := range upstreams {
		f.healthy[i] = true
	}

	if len(f.upstreams) == 0 {
		f.upstreams = []Upstream{{Addr: "1.1.1.1:53", Timeout: 3 * time.Second}}
		f.healthy[0] = true
	}

	go f.healthLoop()
	return f
}

// CurrentUpstream returns the address of the first healthy upstream, for logging purposes.
func (f *PooledForwarder) CurrentUpstream() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for i, up := range f.upstreams {
		if f.healthy[i] {
			return up.Addr
		}
	}
	if len(f.upstreams) > 0 {
		return f.upstreams[0].Addr
	}
	return ""
}

func (f *PooledForwarder) Forward(req *dns.Msg) (*dns.Msg, error) {
	var lastErr error
	for i := range f.upstreams {
		f.mu.RLock()
		ok := f.healthy[i]
		f.mu.RUnlock()
		if !ok {
			continue
		}

		up := f.upstreams[i]
		netType := "udp"
		if up.TLS {
			netType = "tcp-tls"
		}
		client := &dns.Client{Net: netType, Timeout: up.Timeout}
		resp, _, err := client.Exchange(req, up.Addr)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		slog.Warn("upstream failure", "addr", up.Addr, "error", err)
		f.mu.Lock()
		f.healthy[i] = false
		f.mu.Unlock()
	}

	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}

func (f *PooledForwarder) healthLoop() {
	ticker := time.NewTicker(30 * time.Second)
	probeClient := &dns.Client{Timeout: 2 * time.Second}
	msg := new(dns.Msg)
	msg.SetQuestion("google.com.", dns.TypeA)

	for range ticker.C {
		for i, up := range f.upstreams {
			_, _, err := probeClient.Exchange(msg, up.Addr)
			f.mu.Lock()
			wasHealthy := f.healthy[i]
			f.healthy[i] = (err == nil)
			f.mu.Unlock()

			if err != nil {
				slog.Warn("upstream unhealthy", "addr", up.Addr, "error", err)
			} else if !wasHealthy {
				slog.Info("upstream recovered", "addr", up.Addr)
			}
		}
	}
}
