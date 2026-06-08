package dns

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sohidul/esp32-dns-server/internal/db"
	"github.com/sohidul/esp32-dns-server/internal/models"
)

const (
	udpBufferSize  = 1500
	dnsHeaderSize  = 12
	blockedIPResp  = "0.0.0.0"
)

var blockedIPBytes = []byte{0, 0, 0, 0}

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
	resp, _ := h.handle(data, client.IP.String())
	if resp == nil {
		return
	}
	conn.WriteToUDP(resp, client)
}

func (h *Handler) HandleTCP(conn net.Conn, data []byte, clientIP string) {
	resp, _ := h.handle(data, clientIP)
	if resp == nil {
		return
	}

	tcpResp := make([]byte, 2+len(resp))
	tcpResp[0] = byte(len(resp) >> 8)
	tcpResp[1] = byte(len(resp))
	copy(tcpResp[2:], resp)
	conn.Write(tcpResp)
}

func (h *Handler) handle(data []byte, clientIP string) ([]byte, models.Action) {
	domain, qtype := parseQuery(data)
	if domain == "" {
		return nil, ""
	}

	log.Printf("Query: %s (type %d) from %s", domain, qtype, clientIP)

	if h.db.IsBlocked(domain) {
		h.db.LogQuery(domain, clientIP, models.ActionBlocked)
		if h.blockNX {
			return buildNXResponse(data), models.ActionBlocked
		}
		return buildAResponse(data, blockedIPResp, 60, 0), models.ActionBlocked
	}

	if ip := h.db.GetCustomRecord(domain); ip != "" {
		h.db.LogQuery(domain, clientIP, models.ActionCustom)
		return buildAResponse(data, ip, 300, 0), models.ActionCustom
	}

	if cached := h.cache.Get(domain); cached != nil {
		h.db.LogQuery(domain, clientIP, models.ActionCached)
		if cached.NXDOMAIN {
			return buildNXResponse(data), models.ActionCached
		}
		return buildAResponse(data, cached.IP, cached.TTL, 0), models.ActionCached
	}

	response, err := h.forwarder.Forward(data)
	if err != nil {
		h.db.LogQuery(domain, clientIP, models.ActionError)
		return buildNXResponse(data), models.ActionError
	}

	if len(response) >= dnsHeaderSize {
		rcode := response[3] & 0x0F
		if rcode == 3 {
			h.cache.SetNXDOMAIN(domain)
		}
	}

	if ip := ExtractAnswerIP(response); ip != "" {
		ttl := ExtractTTL(response)
		h.cache.Set(domain, ip, ttl)
	}

	h.db.LogQuery(domain, clientIP, models.ActionForwarded)
	return response, models.ActionForwarded
}

func parseQuery(data []byte) (domain string, qtype uint16) {
	if len(data) < dnsHeaderSize {
		return "", 0
	}
	var labels []string
	i := dnsHeaderSize
	for {
		if i >= len(data) {
			return "", 0
		}
		length := int(data[i])
		if length == 0 {
			i++
			break
		}
		if i+1+length > len(data) {
			return "", 0
		}
		labels = append(labels, string(data[i+1:i+1+length]))
		i += 1 + length
	}
	if i+4 > len(data) {
		return "", 0
	}
	qtype = binary.BigEndian.Uint16(data[i : i+2])
	return strings.ToLower(strings.Join(labels, ".")), qtype
}

func findQEnd(data []byte, start int) int {
	i := start
	for i < len(data) && data[i] != 0 {
		i++
	}
	i++ // null terminator
	if i+4 <= len(data) {
		i += 4 // QTYPE + QCLASS
	}
	return i
}

func buildAResponse(query []byte, ip string, ttl uint32, rcode uint8) []byte {
	resp := make([]byte, len(query)+16)
	copy(resp, query[:2])

	var flags uint16 = 0x8000 | uint16(rcode)
	binary.BigEndian.PutUint16(resp[2:4], flags)
	copy(resp[4:6], query[4:6])

	ancount := uint16(0)
	if ip != "" {
		ancount = 1
	}
	binary.BigEndian.PutUint16(resp[6:8], ancount)
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)

	qstart := findQEnd(resp, dnsHeaderSize)
	copy(resp[dnsHeaderSize:qstart], query[dnsHeaderSize:qstart])

	if ip != "" {
		off := qstart
		resp[off] = 0xC0
		resp[off+1] = 0x0C
		binary.BigEndian.PutUint16(resp[off+2:off+4], 1)
		binary.BigEndian.PutUint16(resp[off+4:off+6], 1)
		binary.BigEndian.PutUint32(resp[off+6:off+10], ttl)
		binary.BigEndian.PutUint16(resp[off+10:off+12], 4)

		if ip == blockedIPResp {
			copy(resp[off+12:off+16], blockedIPBytes)
		} else {
			parts := strings.Split(ip, ".")
			if len(parts) == 4 {
				for j, p := range parts {
					n, err := strconv.Atoi(p)
					if err == nil {
						resp[off+12+j] = byte(n)
					}
				}
			}
		}
		resp = resp[:off+16]
	} else {
		resp = resp[:qstart]
	}
	return resp
}

func buildNXResponse(query []byte) []byte {
	resp := make([]byte, len(query))
	copy(resp, query[:2])

	var flags uint16 = 0x8000 | 0x0003
	binary.BigEndian.PutUint16(resp[2:4], flags)
	copy(resp[4:6], query[4:6])
	binary.BigEndian.PutUint16(resp[6:8], 0)
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)

	qend := findQEnd(resp, dnsHeaderSize)
	if qend > len(resp) {
		qend = len(resp)
	}
	return resp[:qend]
}

func ExtractAnswerIP(data []byte) string {
	if len(data) < dnsHeaderSize {
		return ""
	}

	qend := findQEnd(data, dnsHeaderSize)
	if qend+12 > len(data) {
		return ""
	}

	if data[qend] == 0xC0 && data[qend+1] == 0x0C {
		rtype := binary.BigEndian.Uint16(data[qend+2 : qend+4])
		rdlength := binary.BigEndian.Uint16(data[qend+10 : qend+12])

		if rtype == 1 && rdlength == 4 && qend+16 <= len(data) {
			return fmt.Sprintf("%d.%d.%d.%d",
				data[qend+12], data[qend+13], data[qend+14], data[qend+15])
		}
	}
	return ""
}

func ExtractTTL(data []byte) uint32 {
	if len(data) < dnsHeaderSize {
		return 300
	}

	qend := findQEnd(data, dnsHeaderSize)
	if qend+12 <= len(data) && data[qend] == 0xC0 && data[qend+1] == 0x0C {
		return binary.BigEndian.Uint32(data[qend+6 : qend+10])
	}
	return 300
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
