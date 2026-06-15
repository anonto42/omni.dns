package forwarder

import (
	"log/slog"
	"time"

	"github.com/miekg/dns"
)

const healthProbeInterval = 30 * time.Second

// healthLoop periodically probes every upstream and updates its healthy flag,
// logging transitions. It runs until the pool is closed.
func (p *Pool) healthLoop() {
	ticker := time.NewTicker(healthProbeInterval)
	defer ticker.Stop()

	probe := new(dns.Msg)
	probe.SetQuestion("google.com.", dns.TypeA)

	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.probeAll(probe)
		}
	}
}

func (p *Pool) probeAll(probe *dns.Msg) {
	for i, up := range p.snapshot() {
		client := &dns.Client{Net: up.net(), Timeout: 3 * time.Second}
		_, _, err := client.Exchange(probe, up.Addr)

		p.mu.Lock()
		wasHealthy := p.healthy[i]
		p.healthy[i] = err == nil
		p.mu.Unlock()

		switch {
		case err != nil:
			slog.Warn("upstream unhealthy", "addr", up.Addr, "error", err)
		case !wasHealthy:
			slog.Info("upstream recovered", "addr", up.Addr)
		}
	}
}
