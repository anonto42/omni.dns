package app

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	appdns "github.com/sohidul/dns-server/internal/dns"
)

func (a *App) startUDP(done <-chan struct{}) error {
	udpConn, err := listenUDPWithRetry(a.cfg.DNSAddr, a.cfg.DNSPort)
	if err != nil {
		return fmt.Errorf("listen UDP on port %d: %w", a.cfg.DNSPort, err)
	}
	a.udpConn = udpConn

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		slog.Info("listening", "protocol", "UDP", "port", a.cfg.DNSPort)
		buf := make([]byte, 1500)
		for {
			select {
			case <-done:
				return
			default:
			}
			if err := udpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				slog.Error("failed to set udp read deadline", "error", err)
			}
			n, client, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			go a.dnsHandler.HandleUDP(udpConn, client, pkt)
		}
	}()

	return nil
}

func (a *App) startTCP(done <-chan struct{}) {
	tcpAddr := net.TCPAddr{IP: net.ParseIP(a.cfg.DNSAddr), Port: a.cfg.DNSPort}
	tcpListener, err := net.ListenTCP("tcp", &tcpAddr)
	if err != nil {
		slog.Warn("TCP listener unavailable (non-fatal)", "port", a.cfg.DNSPort, "error", err)
		return
	}
	a.tcpListener = tcpListener

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		slog.Info("listening", "protocol", "TCP", "port", a.cfg.DNSPort)
		for {
			select {
			case <-done:
				return
			default:
			}
			if err := tcpListener.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				slog.Error("failed to set tcp listener deadline", "error", err)
			}
			conn, err := tcpListener.AcceptTCP()
			if err != nil {
				continue
			}
			go handleTCPConn(conn, a.dnsHandler)
		}
	}()
}

func handleTCPConn(conn *net.TCPConn, handler *appdns.Handler) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		slog.Error("failed to set tcp conn deadline", "error", err)
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
