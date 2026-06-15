package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	appdns "github.com/sohidul/dns-server/internal/dns"
)

const readDeadline = 500 * time.Millisecond

func (s *Server) startUDP(ctx context.Context) error {
	conn, err := listenUDPWithRetry(s.cfg.DNSAddr, s.cfg.DNSPort)
	if err != nil {
		return fmt.Errorf("listen UDP on port %d: %w", s.cfg.DNSPort, err)
	}
	s.udpConn = conn

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		slog.Info("listening", "protocol", "UDP", "port", s.cfg.DNSPort)
		buf := make([]byte, 1500)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
				slog.Error("set udp read deadline failed", "error", err)
			}
			n, client, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			go s.handler.HandleUDP(conn, client, pkt)
		}
	}()
	return nil
}

func (s *Server) startTCP(ctx context.Context) {
	addr := net.TCPAddr{IP: net.ParseIP(s.cfg.DNSAddr), Port: s.cfg.DNSPort}
	listener, err := net.ListenTCP("tcp", &addr)
	if err != nil {
		slog.Warn("TCP listener unavailable (non-fatal)", "port", s.cfg.DNSPort, "error", err)
		return
	}
	s.tcpListener = listener

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		slog.Info("listening", "protocol", "TCP", "port", s.cfg.DNSPort)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := listener.SetDeadline(time.Now().Add(readDeadline)); err != nil {
				slog.Error("set tcp listener deadline failed", "error", err)
			}
			conn, err := listener.AcceptTCP()
			if err != nil {
				continue
			}
			go handleTCPConn(conn, s.handler)
		}
	}()
}

func handleTCPConn(conn *net.TCPConn, handler *appdns.Handler) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		slog.Error("set tcp conn deadline failed", "error", err)
	}

	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return
	}
	length := int(lenBuf[0])<<8 | int(lenBuf[1])
	if length < 12 || length > 4096 {
		return
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return
	}
	clientIP := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	handler.HandleTCP(conn, data, clientIP)
}

func (s *Server) closeListeners() {
	if s.httpServer != nil {
		if err := s.httpServer.Close(); err != nil {
			slog.Error("close http server", "error", err)
		}
	}
	if s.udpConn != nil {
		if err := s.udpConn.Close(); err != nil {
			slog.Error("close udp listener", "error", err)
		}
	}
	if s.tcpListener != nil {
		if err := s.tcpListener.Close(); err != nil {
			slog.Error("close tcp listener", "error", err)
		}
	}
}

func listenUDPWithRetry(addr string, port int) (*net.UDPConn, error) {
	for i := 0; i < 10; i++ {
		udpAddr := net.UDPAddr{IP: net.ParseIP(addr), Port: port}
		conn, err := net.ListenUDP("udp", &udpAddr)
		if err == nil {
			return conn, nil
		}
		slog.Warn("failed to listen UDP, retrying...", "port", port, "attempt", i+1)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not listen on UDP port %d after retries", port)
}
