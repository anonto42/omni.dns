package dns

import (
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/sohidul/dns-server/internal/db"
	"github.com/sohidul/dns-server/internal/models"
)

const (
	blockedIPResp = "0.0.0.0"
)

type Config struct {
	BlockNXDOMAIN bool
	Upstreams     []Upstream
	CacheSize     int
}

type Handler struct {
	db        *db.DB
	cache     *Cache
	forwarder *PooledForwarder
	started   time.Time
	blockNX   bool

	rulesMu       sync.RWMutex
	rulesCache    []models.SteeringRule
	rulesLoadedAt time.Time
}

func NewHandler(database *db.DB, cfg *Config) *Handler {
	if cfg == nil {
		cfg = &Config{
			CacheSize: 1000,
			Upstreams: []Upstream{{Addr: "1.1.1.1:53", Timeout: 3 * time.Second}},
		}
	}

	return &Handler{
		db:        database,
		cache:     NewCache(cfg.CacheSize),
		forwarder: NewPooledForwarder(cfg.Upstreams),
		started:   time.Now(),
		blockNX:   cfg.BlockNXDOMAIN,
	}
}

func (h *Handler) HandleUDP(conn *net.UDPConn, client *net.UDPAddr, data []byte) {
	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		slog.Error("unpack failed", "error", err)
		return
	}

	respMsg := h.handle(msg, client.IP.String(), "UDP")
	if respMsg == nil {
		return
	}

	respData, err := respMsg.Pack()
	if err != nil {
		slog.Error("pack failed", "error", err)
		return
	}
	if _, err := conn.WriteToUDP(respData, client); err != nil {
		slog.Error("failed to write udp response", "error", err)
	}
}

func (h *Handler) HandleTCP(conn net.Conn, data []byte, clientIP string) {
	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		slog.Error("unpack failed", "error", err)
		return
	}

	respMsg := h.handle(msg, clientIP, "TCP")
	if respMsg == nil {
		return
	}

	respData, err := respMsg.Pack()
	if err != nil {
		slog.Error("pack failed", "error", err)
		return
	}

	tcpResp := make([]byte, 2+len(respData))
	tcpResp[0] = byte(len(respData) >> 8)
	tcpResp[1] = byte(len(respData))
	copy(tcpResp[2:], respData)
	if _, err := conn.Write(tcpResp); err != nil {
		slog.Error("failed to write tcp response", "error", err)
	}
}

// extractAnswerInfo pulls resolved IPs, answer count, TTL, and all-answers string from DNS response records.
func extractAnswerInfo(answers []dns.RR) (firstIP string, allAnswers string, count int, ttl uint32) {
	var ips []string
	for _, rr := range answers {
		switch r := rr.(type) {
		case *dns.A:
			ips = append(ips, r.A.String())
			if ttl == 0 {
				ttl = r.Hdr.Ttl
			}
		case *dns.AAAA:
			ips = append(ips, r.AAAA.String())
			if ttl == 0 {
				ttl = r.Hdr.Ttl
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

func (h *Handler) handle(msg *dns.Msg, clientIP, protocol string) *dns.Msg {
	if len(msg.Question) == 0 {
		return nil
	}

	question := msg.Question[0]
	domain := strings.ToLower(strings.TrimSuffix(question.Name, "."))
	qtype := question.Qtype
	qtypeStr := dns.TypeToString[qtype]

	slog.Debug("dns query", "domain", domain, "qtype", qtypeStr, "client", clientIP, "proto", protocol)

	base := models.QueryLog{
		Domain:    domain,
		ClientIP:  clientIP,
		Protocol:  protocol,
		QueryType: qtypeStr,
	}

	if h.db.IsBlocked(domain) {
		entry := base
		entry.Action = models.ActionBlocked
		entry.ResponseCode = "BLOCKED"
		entry.ResolvedIP = blockedIPResp
		entry.AllAnswers = blockedIPResp
		entry.AnswerCount = 1
		h.db.LogQuery(entry)
		return h.buildResponse(msg, blockedIPResp, 60, h.blockNX)
	}

	// Evaluate steering rules in priority order.
	for _, rule := range h.getSteeringRules() {
		if !rule.Enabled {
			continue
		}
		if !matchesCondition(rule, domain, clientIP, qtypeStr) {
			continue
		}
		slog.Debug("steering rule matched", "rule", rule.Name, "action", rule.ActionType)

		switch rule.ActionType {
		case "Block":
			entry := base
			entry.Action = models.ActionBlocked
			entry.ResponseCode = "BLOCKED"
			entry.ResolvedIP = blockedIPResp
			entry.AllAnswers = blockedIPResp
			entry.AnswerCount = 1
			h.db.LogQuery(entry)
			return h.buildResponse(msg, blockedIPResp, 60, h.blockNX)

		case "Redirect":
			// Return the target IP directly.
			if qtype == dns.TypeA && rule.ActionTarget != "" {
				entry := base
				entry.Action = models.ActionCustom
				entry.ResponseCode = "NOERROR"
				entry.ResolvedIP = rule.ActionTarget
				entry.AllAnswers = rule.ActionTarget
				entry.AnswerCount = 1
				entry.TTL = 60
				h.db.LogQuery(entry)
				return h.buildResponse(msg, rule.ActionTarget, 60, false)
			}

		case "Forward":
			// Send to a specific upstream instead of the default.
			target := rule.ActionTarget
			if target == "" {
				break
			}
			// Ensure port is present.
			if !strings.Contains(target, ":") {
				target = target + ":53"
			}
			start := time.Now()
			resp, err := forwardToUpstream(msg, target)
			latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
			if err != nil {
				slog.Warn("steering forward failed", "rule", rule.Name, "target", target, "error", err)
				break // fall through to default resolution
			}
			firstIP, allAnswers, answerCount, ttl := extractAnswerInfo(resp.Answer)
			entry := base
			entry.Action = models.ActionForwarded
			entry.ResponseCode = dns.RcodeToString[resp.Rcode]
			entry.ResolvedIP = firstIP
			entry.AllAnswers = allAnswers
			entry.AnswerCount = answerCount
			entry.TTL = ttl
			entry.UpstreamResolver = target
			entry.LatencyMs = latencyMs
			h.db.LogQuery(entry)
			return resp
		}
		break // first matching rule wins
	}

	if ip := h.db.GetCustomRecord(domain); ip != "" {
		if qtype == dns.TypeA {
			entry := base
			entry.Action = models.ActionCustom
			entry.ResponseCode = "NOERROR"
			entry.ResolvedIP = ip
			entry.AllAnswers = ip
			entry.AnswerCount = 1
			entry.TTL = 300
			h.db.LogQuery(entry)
			return h.buildResponse(msg, ip, 300, false)
		}
	}

	if cached := h.cache.Get(domain); cached != nil {
		if qtype == dns.TypeA {
			entry := base
			entry.Action = models.ActionCached
			entry.ResponseCode = "NOERROR"
			entry.ResolvedIP = cached.IP
			entry.AllAnswers = cached.IP
			entry.AnswerCount = 1
			entry.TTL = cached.TTL
			h.db.LogQuery(entry)
			return h.buildResponse(msg, cached.IP, cached.TTL, cached.NXDOMAIN)
		}
	}

	upstream := h.forwarder.CurrentUpstream()
	start := time.Now()
	response, err := h.forwarder.Forward(msg)
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		entry := base
		entry.Action = models.ActionError
		entry.ResponseCode = "SERVFAIL"
		entry.UpstreamResolver = upstream
		entry.LatencyMs = latencyMs
		h.db.LogQuery(entry)
		resp := new(dns.Msg)
		resp.SetRcode(msg, dns.RcodeServerFailure)
		return resp
	}

	firstIP, allAnswers, answerCount, ttl := extractAnswerInfo(response.Answer)
	rcodeStr := dns.RcodeToString[response.Rcode]

	if response.Rcode == dns.RcodeNameError {
		h.cache.SetNXDOMAIN(domain)
	}

	for _, rr := range response.Answer {
		if a, ok := rr.(*dns.A); ok {
			h.cache.Set(domain, a.A.String(), a.Hdr.Ttl)
			break
		}
	}

	entry := base
	entry.Action = models.ActionForwarded
	entry.ResponseCode = rcodeStr
	entry.ResolvedIP = firstIP
	entry.AllAnswers = allAnswers
	entry.AnswerCount = answerCount
	entry.TTL = ttl
	entry.UpstreamResolver = upstream
	entry.LatencyMs = latencyMs
	h.db.LogQuery(entry)
	return response
}

func (h *Handler) buildResponse(req *dns.Msg, ip string, ttl uint32, nxdomain bool) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)

	if nxdomain {
		resp.SetRcode(req, dns.RcodeNameError)
		return resp
	}

	if ip != "" && req.Question[0].Qtype == dns.TypeA {
		rr, err := dns.NewRR(req.Question[0].Name + " " + strconv.FormatUint(uint64(ttl), 10) + " IN A " + ip)
		if err == nil {
			resp.Answer = append(resp.Answer, rr)
		}
	}

	return resp
}

func (h *Handler) SetPrimaryUpstream(addr string, tls bool) {
	h.forwarder.SetPrimaryUpstream(addr, tls)
}

func (h *Handler) SetBlockNXDOMAIN(v bool) {
	h.blockNX = v
}

// getSteeringRules returns cached rules, refreshing every 10 seconds.
func (h *Handler) getSteeringRules() []models.SteeringRule {
	h.rulesMu.RLock()
	if time.Since(h.rulesLoadedAt) < 10*time.Second {
		rules := h.rulesCache
		h.rulesMu.RUnlock()
		return rules
	}
	h.rulesMu.RUnlock()

	rules := h.db.GetSteeringRules()
	h.rulesMu.Lock()
	h.rulesCache = rules
	h.rulesLoadedAt = time.Now()
	h.rulesMu.Unlock()
	return rules
}

// forwardToUpstream sends a query to a specific upstream address and returns the response.
func forwardToUpstream(msg *dns.Msg, addr string) (*dns.Msg, error) {
	tls := strings.HasSuffix(addr, ":853")
	netType := "udp"
	if tls {
		netType = "tcp-tls"
	}
	client := &dns.Client{Net: netType, Timeout: 4 * time.Second}
	resp, _, err := client.Exchange(msg, addr)
	return resp, err
}

func (h *Handler) UptimeSeconds() float64 {
	return time.Since(h.started).Seconds()
}

func (h *Handler) CacheSize() int {
	return h.cache.Size()
}

func (h *Handler) CacheHits() int64 {
	return h.cache.Hits()
}

func (h *Handler) CacheMisses() int64 {
	return h.cache.Misses()
}

func matchesCondition(rule models.SteeringRule, domain, clientIP, qtypeStr string) bool {
	switch rule.ConditionType {
	case "Domain":
		return matchSteeringDomain(rule.ConditionValue, domain)
	case "Client IP":
		return matchCIDR(rule.ConditionValue, clientIP)
	case "Query Type":
		for _, t := range strings.Split(rule.ConditionValue, ",") {
			if strings.EqualFold(strings.TrimSpace(t), qtypeStr) {
				return true
			}
		}
	case "Time Range":
		return matchTimeRange(rule.ConditionValue)
	}
	return false
}

func matchSteeringDomain(pattern, domain string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:]
		return domain == suffix || strings.HasSuffix(domain, "."+suffix)
	}
	return domain == pattern
}

func matchCIDR(cidr, ip string) bool {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		parsed := net.ParseIP(cidr)
		target := net.ParseIP(ip)
		return parsed != nil && target != nil && parsed.Equal(target)
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && network.Contains(parsed)
}

func matchTimeRange(r string) bool {
	parts := strings.SplitN(r, "-", 2)
	if len(parts) != 2 {
		return false
	}
	now := time.Now()
	cur := now.Hour()*60 + now.Minute()
	start := parseHHMM(parts[0])
	end := parseHHMM(parts[1])
	return start >= 0 && end >= 0 && cur >= start && cur <= end
}

func parseHHMM(s string) int {
	p := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(p) != 2 {
		return -1
	}
	h, err1 := strconv.Atoi(p[0])
	m, err2 := strconv.Atoi(p[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}
