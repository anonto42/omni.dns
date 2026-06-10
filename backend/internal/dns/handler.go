package dns

import (
	"log/slog"
	"net"
	"strconv"
	"strings"
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

	respMsg, _ := h.handle(msg, client.IP.String(), "UDP")
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

	respMsg, _ := h.handle(msg, clientIP, "TCP")
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

func (h *Handler) handle(msg *dns.Msg, clientIP, protocol string) (*dns.Msg, models.Action) {
	if len(msg.Question) == 0 {
		return nil, ""
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
		return h.buildResponse(msg, blockedIPResp, 60, h.blockNX), models.ActionBlocked
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
			return h.buildResponse(msg, ip, 300, false), models.ActionCustom
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
			return h.buildResponse(msg, cached.IP, cached.TTL, cached.NXDOMAIN), models.ActionCached
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
		return resp, models.ActionError
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
	return response, models.ActionForwarded
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
