// Package dns adapts raw UDP/TCP DNS packets to the resolver pipeline.
package dns

import (
	"log/slog"
	"net"

	"github.com/miekg/dns"

	resolver "github.com/sohidul/dns-server/internal/modules/resolver/engine"
)

// Handler decodes wire-format DNS messages, runs them through the resolver, and
// writes the encoded response back to the client.
type Handler struct {
	resolver *resolver.Resolver
}

// NewHandler wraps a resolver for packet-level handling.
func NewHandler(r *resolver.Resolver) *Handler {
	return &Handler{resolver: r}
}

// HandleUDP processes a single UDP query and writes the response datagram.
func (h *Handler) HandleUDP(conn *net.UDPConn, client *net.UDPAddr, data []byte) {
	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		slog.Error("unpack udp query failed", "error", err)
		return
	}
	resp := h.resolver.Resolve(msg, client.IP.String(), "UDP")
	if resp == nil {
		return
	}
	out, err := resp.Pack()
	if err != nil {
		slog.Error("pack udp response failed", "error", err)
		return
	}
	if _, err := conn.WriteToUDP(out, client); err != nil {
		slog.Error("write udp response failed", "error", err)
	}
}

// HandleTCP processes a single TCP query and writes the length-prefixed response.
func (h *Handler) HandleTCP(conn net.Conn, data []byte, clientIP string) {
	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		slog.Error("unpack tcp query failed", "error", err)
		return
	}
	resp := h.resolver.Resolve(msg, clientIP, "TCP")
	if resp == nil {
		return
	}
	out, err := resp.Pack()
	if err != nil {
		slog.Error("pack tcp response failed", "error", err)
		return
	}
	framed := make([]byte, 2+len(out))
	framed[0] = byte(len(out) >> 8)
	framed[1] = byte(len(out))
	copy(framed[2:], out)
	if _, err := conn.Write(framed); err != nil {
		slog.Error("write tcp response failed", "error", err)
	}
}
