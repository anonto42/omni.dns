// Package resolver implements the DNS resolution pipeline: blocklist, steering
// rules, custom records, cache, and upstream forwarding. Each step is a small
// method on Resolver so the pipeline reads top-to-bottom.
package resolver

import (
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/sohidul/dns-server/internal/db/models"
	"github.com/sohidul/dns-server/internal/dns/cache"
	"github.com/sohidul/dns-server/internal/dns/forwarder"
)

const blockedIPv4 = "0.0.0.0"

// Resolver answers DNS queries by walking the resolution pipeline.
type Resolver struct {
	blocklist Blocklist
	records   CustomRecords
	steering  SteeringRules
	logger    QueryLogger
	mac       MACResolver

	cache   *cache.Cache
	pool    *forwarder.Pool
	started time.Time
	blockNX bool

	rulesMu       sync.RWMutex
	rulesCache    []models.SteeringRule
	rulesLoadedAt time.Time
}

// Deps bundles the collaborators a Resolver needs.
type Deps struct {
	Blocklist Blocklist
	Records   CustomRecords
	Steering  SteeringRules
	Logger    QueryLogger
	MAC       MACResolver
	Cache     *cache.Cache
	Pool      *forwarder.Pool
	BlockNX   bool
}

// New builds a Resolver from its dependencies.
func New(d Deps) *Resolver {
	return &Resolver{
		blocklist: d.Blocklist,
		records:   d.Records,
		steering:  d.Steering,
		logger:    d.Logger,
		mac:       d.MAC,
		cache:     d.Cache,
		pool:      d.Pool,
		started:   time.Now(),
		blockNX:   d.BlockNX,
	}
}

// Resolve runs the full pipeline for a parsed query and returns the response
// message (nil if the query was malformed and should be dropped).
func (r *Resolver) Resolve(msg *dns.Msg, clientIP, protocol string) *dns.Msg {
	if len(msg.Question) == 0 {
		return nil
	}
	q := msg.Question[0]
	domain := strings.ToLower(strings.TrimSuffix(q.Name, "."))
	qtypeStr := dns.TypeToString[q.Qtype]

	slog.Debug("dns query", "domain", domain, "qtype", qtypeStr, "client", clientIP, "proto", protocol)

	base := models.QueryLog{
		Domain:     domain,
		ClientIP:   clientIP,
		MACAddress: r.mac.Lookup(clientIP),
		Protocol:   protocol,
		QueryType:  qtypeStr,
	}

	if resp := r.stepBlocklist(msg, domain, base); resp != nil {
		return resp
	}
	if resp := r.stepSteering(msg, domain, clientIP, qtypeStr, base); resp != nil {
		return resp
	}
	if resp := r.stepCustomRecords(msg, domain, base); resp != nil {
		return resp
	}
	if resp := r.stepCache(msg, domain, base); resp != nil {
		return resp
	}
	return r.stepUpstream(msg, domain, base)
}

// --- pipeline steps -------------------------------------------------------

func (r *Resolver) stepBlocklist(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	if !r.blocklist.IsBlocked(domain) {
		return nil
	}
	entry := base
	entry.Action = models.ActionBlocked
	entry.ResponseCode = "BLOCKED"
	entry.ResolvedIP = blockedIPv4
	entry.AllAnswers = blockedIPv4
	entry.AnswerCount = 1
	r.logger.LogQuery(entry)
	return r.answerBlocked(msg)
}

func (r *Resolver) stepSteering(msg *dns.Msg, domain, clientIP, qtypeStr string, base models.QueryLog) *dns.Msg {
	qtype := msg.Question[0].Qtype
	for _, rule := range r.activeRules() {
		if !rule.Enabled || !matchesCondition(rule, domain, clientIP, qtypeStr) {
			continue
		}
		slog.Debug("steering rule matched", "rule", rule.Name, "action", rule.ActionType)

		switch rule.ActionType {
		case "Block":
			entry := base
			entry.Action = models.ActionBlocked
			entry.ResponseCode = "BLOCKED"
			entry.ResolvedIP = blockedIPv4
			entry.AllAnswers = blockedIPv4
			entry.AnswerCount = 1
			r.logger.LogQuery(entry)
			return r.answerBlocked(msg)

		case "Redirect":
			return r.steeringRedirect(msg, rule, qtype, base)

		case "Forward":
			if resp := r.steeringForward(msg, rule, base); resp != nil {
				return resp
			}
			// Forward failed: fall through to default resolution.
		}
		break // first matching rule wins
	}
	return nil
}

// steeringRedirect returns the rule's target IP for matching-family queries and
// a proper NODATA response when the family does not match (e.g. AAAA query, IPv4
// target), so the client is not left without an answer.
func (r *Resolver) steeringRedirect(msg *dns.Msg, rule models.SteeringRule, qtype uint16, base models.QueryLog) *dns.Msg {
	if rule.ActionTarget == "" {
		return nil
	}
	target := net.ParseIP(rule.ActionTarget)
	if target == nil {
		return nil
	}
	isV4 := target.To4() != nil

	if (qtype == dns.TypeA && isV4) || (qtype == dns.TypeAAAA && !isV4) {
		entry := base
		entry.Action = models.ActionCustom
		entry.ResponseCode = "NOERROR"
		entry.ResolvedIP = rule.ActionTarget
		entry.AllAnswers = rule.ActionTarget
		entry.AnswerCount = 1
		entry.TTL = 60
		r.logger.LogQuery(entry)
		return r.answerAddress(msg, rule.ActionTarget, 60)
	}

	// Family mismatch (e.g. AAAA asked, only an IPv4 redirect exists): NODATA.
	if qtype == dns.TypeA || qtype == dns.TypeAAAA {
		entry := base
		entry.Action = models.ActionCustom
		entry.ResponseCode = "NOERROR"
		r.logger.LogQuery(entry)
		return r.answerNoData(msg)
	}
	return nil
}

func (r *Resolver) steeringForward(msg *dns.Msg, rule models.SteeringRule, base models.QueryLog) *dns.Msg {
	target := rule.ActionTarget
	if target == "" {
		return nil
	}
	if !strings.Contains(target, ":") {
		target += ":53"
	}
	start := time.Now()
	resp, err := exchange(msg, target)
	latency := millisSince(start)
	if err != nil {
		slog.Warn("steering forward failed", "rule", rule.Name, "target", target, "error", err)
		return nil
	}
	firstIP, allAnswers, count, ttl := extractAnswers(resp.Answer)
	entry := base
	entry.Action = models.ActionForwarded
	entry.ResponseCode = dns.RcodeToString[resp.Rcode]
	entry.ResolvedIP = firstIP
	entry.AllAnswers = allAnswers
	entry.AnswerCount = count
	entry.TTL = ttl
	entry.UpstreamResolver = target
	entry.LatencyMs = latency
	r.logger.LogQuery(entry)
	return resp
}

func (r *Resolver) stepCustomRecords(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	qtype := msg.Question[0].Qtype
	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return nil
	}
	ip, found, existsOtherType := r.records.Lookup(domain, qtype)
	if found {
		entry := base
		entry.Action = models.ActionCustom
		entry.ResponseCode = "NOERROR"
		entry.ResolvedIP = ip
		entry.AllAnswers = ip
		entry.AnswerCount = 1
		entry.TTL = 300
		r.logger.LogQuery(entry)
		return r.answerAddress(msg, ip, 300)
	}
	if existsOtherType {
		// The name has a record of the other family only: answer NODATA so the
		// client stops waiting instead of falling through to upstream.
		entry := base
		entry.Action = models.ActionCustom
		entry.ResponseCode = "NOERROR"
		r.logger.LogQuery(entry)
		return r.answerNoData(msg)
	}
	return nil
}

func (r *Resolver) stepCache(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	qtype := msg.Question[0].Qtype
	cached := r.cache.Get(domain, qtype)
	if cached == nil {
		return nil
	}
	entry := base
	entry.Action = models.ActionCached
	switch {
	case cached.NXDOMAIN:
		entry.ResponseCode = "NXDOMAIN"
		r.logger.LogQuery(entry)
		return r.answerNXDOMAIN(msg)
	case cached.NoData:
		entry.ResponseCode = "NOERROR"
		r.logger.LogQuery(entry)
		return r.answerNoData(msg)
	default:
		ip := cached.First()
		entry.ResponseCode = "NOERROR"
		entry.ResolvedIP = ip
		entry.AllAnswers = strings.Join(cached.IPs, ",")
		entry.AnswerCount = len(cached.IPs)
		entry.TTL = cached.TTL
		r.logger.LogQuery(entry)
		return r.answerAddress(msg, ip, cached.TTL)
	}
}

func (r *Resolver) stepUpstream(msg *dns.Msg, domain string, base models.QueryLog) *dns.Msg {
	qtype := msg.Question[0].Qtype
	upstream := r.pool.Current()
	start := time.Now()
	resp, err := r.pool.Forward(msg)
	latency := millisSince(start)

	if err != nil {
		entry := base
		entry.Action = models.ActionError
		entry.ResponseCode = "SERVFAIL"
		entry.UpstreamResolver = upstream
		entry.LatencyMs = latency
		r.logger.LogQuery(entry)
		fail := new(dns.Msg)
		fail.SetRcode(msg, dns.RcodeServerFailure)
		return fail
	}

	r.cacheUpstream(domain, qtype, resp)

	firstIP, allAnswers, count, ttl := extractAnswers(resp.Answer)
	entry := base
	entry.Action = models.ActionForwarded
	entry.ResponseCode = dns.RcodeToString[resp.Rcode]
	entry.ResolvedIP = firstIP
	entry.AllAnswers = allAnswers
	entry.AnswerCount = count
	entry.TTL = ttl
	entry.UpstreamResolver = upstream
	entry.LatencyMs = latency
	r.logger.LogQuery(entry)
	return resp
}

// cacheUpstream stores the upstream answer keyed by query type, including
// negative (NXDOMAIN) and empty (NODATA) responses for A/AAAA queries.
func (r *Resolver) cacheUpstream(domain string, qtype uint16, resp *dns.Msg) {
	if resp.Rcode == dns.RcodeNameError {
		r.cache.SetNXDOMAIN(domain, qtype)
		return
	}
	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return
	}

	var ips []string
	var ttl uint32
	for _, rr := range resp.Answer {
		switch v := rr.(type) {
		case *dns.A:
			if qtype == dns.TypeA {
				ips = append(ips, v.A.String())
				if ttl == 0 {
					ttl = v.Hdr.Ttl
				}
			}
		case *dns.AAAA:
			if qtype == dns.TypeAAAA {
				ips = append(ips, v.AAAA.String())
				if ttl == 0 {
					ttl = v.Hdr.Ttl
				}
			}
		}
	}
	if len(ips) > 0 {
		r.cache.Set(domain, qtype, ips, ttl)
	} else if resp.Rcode == dns.RcodeSuccess {
		r.cache.SetNoData(domain, qtype)
	}
}

// --- runtime controls -----------------------------------------------------

// SetPrimaryUpstream swaps the primary upstream at runtime.
func (r *Resolver) SetPrimaryUpstream(addr string, tls bool) { r.pool.SetPrimary(addr, tls) }

// SetBlockNXDOMAIN toggles whether blocked domains return NXDOMAIN vs 0.0.0.0.
func (r *Resolver) SetBlockNXDOMAIN(v bool) { r.blockNX = v }

// UptimeSeconds reports how long the resolver has been running.
func (r *Resolver) UptimeSeconds() float64 { return time.Since(r.started).Seconds() }

// CacheSize/Hits/Misses expose cache metrics for the status endpoint.
func (r *Resolver) CacheSize() int     { return r.cache.Size() }
func (r *Resolver) CacheHits() int64   { return r.cache.Hits() }
func (r *Resolver) CacheMisses() int64 { return r.cache.Misses() }

// activeRules returns steering rules, refreshing the local snapshot every 10s.
func (r *Resolver) activeRules() []models.SteeringRule {
	r.rulesMu.RLock()
	if time.Since(r.rulesLoadedAt) < 10*time.Second {
		rules := r.rulesCache
		r.rulesMu.RUnlock()
		return rules
	}
	r.rulesMu.RUnlock()

	rules := r.steering.Rules()
	r.rulesMu.Lock()
	r.rulesCache = rules
	r.rulesLoadedAt = time.Now()
	r.rulesMu.Unlock()
	return rules
}

func millisSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

// extractAnswers pulls resolved IPs, an "a,b,c" string, the count, and the
// first TTL from A/AAAA answer records.
func extractAnswers(answers []dns.RR) (firstIP, allAnswers string, count int, ttl uint32) {
	var ips []string
	for _, rr := range answers {
		switch v := rr.(type) {
		case *dns.A:
			ips = append(ips, v.A.String())
			if ttl == 0 {
				ttl = v.Hdr.Ttl
			}
		case *dns.AAAA:
			ips = append(ips, v.AAAA.String())
			if ttl == 0 {
				ttl = v.Hdr.Ttl
			}
		}
	}
	if len(ips) > 0 {
		firstIP = ips[0]
		allAnswers = strings.Join(ips, ",")
		count = len(ips)
	}
	return
}

func exchange(msg *dns.Msg, addr string) (*dns.Msg, error) {
	netType := "udp"
	if strings.HasSuffix(addr, ":853") {
		netType = "tcp-tls"
	}
	client := &dns.Client{Net: netType, Timeout: 4 * time.Second}
	resp, _, err := client.Exchange(msg, addr)
	return resp, err
}
