package dns

import (
	"log/slog"
	"net"
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

	respMsg, _ := h.handle(msg, client.IP.String())
	if respMsg == nil {
		return
	}

	respData, err := respMsg.Pack()
	if err != nil {
		slog.Error("pack failed", "error", err)
		return
	}
	conn.WriteToUDP(respData, client)
}

func (h *Handler) HandleTCP(conn net.Conn, data []byte, clientIP string) {
	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		slog.Error("unpack failed", "error", err)
		return
	}

	respMsg, _ := h.handle(msg, clientIP)
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
	conn.Write(tcpResp)
}

func (h *Handler) handle(msg *dns.Msg, clientIP string) (*dns.Msg, models.Action) {
	if len(msg.Question) == 0 {
		return nil, ""
	}

	question := msg.Question[0]
	domain := strings.ToLower(strings.TrimSuffix(question.Name, "."))
	qtype := question.Qtype

	slog.Debug("dns query", "domain", domain, "qtype", dns.TypeToString[qtype], "client", clientIP)

	if h.db.IsBlocked(domain) {
		h.db.LogQuery(domain, clientIP, models.ActionBlocked)
		return h.buildResponse(msg, blockedIPResp, 60, h.blockNX), models.ActionBlocked
	}

	if ip := h.db.GetCustomRecord(domain); ip != "" {
		if qtype == dns.TypeA {
			h.db.LogQuery(domain, clientIP, models.ActionCustom)
			return h.buildResponse(msg, ip, 300, false), models.ActionCustom
		}
	}

	if cached := h.cache.Get(domain); cached != nil {
		if qtype == dns.TypeA {
			h.db.LogQuery(domain, clientIP, models.ActionCached)
			return h.buildResponse(msg, cached.IP, cached.TTL, cached.NXDOMAIN), models.ActionCached
		}
	}

	response, err := h.forwarder.Forward(msg)
	if err != nil {
		h.db.LogQuery(domain, clientIP, models.ActionError)
		resp := new(dns.Msg)
		resp.SetRcode(msg, dns.RcodeServerFailure)
		return resp, models.ActionError
	}

	if response.Rcode == dns.RcodeNameError {
		h.cache.SetNXDOMAIN(domain)
	}

	for _, rr := range response.Answer {
		if a, ok := rr.(*dns.A); ok {
			h.cache.Set(domain, a.A.String(), a.Hdr.Ttl)
			break
		}
	}

	h.db.LogQuery(domain, clientIP, models.ActionForwarded)
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
