package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/sohidul/esp32-dns-server/internal/api"
	"github.com/sohidul/esp32-dns-server/internal/db"
	"github.com/sohidul/esp32-dns-server/internal/dns"
)

var (
	dnsPort     = flag.Int("dns-port", 53, "DNS server port")
	httpPort    = flag.Int("http-port", 8080, "HTTP API port")
	dbPath      = flag.String("db", "data/dns.db", "SQLite database path")
	blockNX     = flag.Bool("block-nxdomain", false, "Return NXDOMAIN for blocked domains")
	cacheSize   = flag.Int("cache-size", 1000, "DNS cache size")
	upstreamDNS = flag.String("upstream", "1.1.1.1:53", "Upstream DNS server")
	staticDir   = flag.String("static", "", "Directory with static files to serve at /")
	logPrune    = flag.Duration("log-prune", 0, "Auto-prune logs older than this (e.g. 72h)")
)

func main() {
	flag.Parse()

	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	dnsCfg := &dns.Config{
		BlockNXDOMAIN: *blockNX,
		CacheSize:     *cacheSize,
		Upstreams: []dns.Upstream{
			{Addr: *upstreamDNS, Timeout: 3 * time.Second},
			{Addr: "8.8.8.8:53", Timeout: 5 * time.Second},
		},
	}
	handler := dns.NewHandler(database, dnsCfg)

	var wg sync.WaitGroup
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// --- UDP DNS ---
	udpAddr := net.UDPAddr{Port: *dnsPort}
	udpConn, err := net.ListenUDP("udp", &udpAddr)
	if err != nil {
		log.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("DNS (UDP) listening on :%d", *dnsPort)
		buf := make([]byte, 1500)
		for {
			select {
			case <-quit:
				return
			default:
			}
			udpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, client, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			go handler.HandleUDP(udpConn, client, pkt)
		}
	}()

	// --- TCP DNS ---
	tcpAddr := net.TCPAddr{Port: *dnsPort}
	tcpListener, err := net.ListenTCP("tcp", &tcpAddr)
	if err != nil {
		log.Printf("Warning: TCP DNS listener on :%d failed (non-fatal): %v", *dnsPort, err)
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("DNS (TCP) listening on :%d", *dnsPort)
			for {
				select {
				case <-quit:
					tcpListener.Close()
					return
				default:
				}
				tcpListener.SetDeadline(time.Now().Add(500 * time.Millisecond))
				conn, err := tcpListener.AcceptTCP()
				if err != nil {
					continue
				}
				go handleTCPConn(conn, handler)
			}
		}()
	}

	// --- HTTP API ---
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(api.CORS)

	api.RegisterRoutes(r, database, handler)

	if *staticDir != "" {
		r.Handle("/*", http.FileServer(http.Dir(*staticDir)))
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", *httpPort),
		Handler: r,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("HTTP API listening on :%d", *httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// --- Log pruning ---
	if *logPrune > 0 {
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			for range ticker.C {
				database.PruneLogs(time.Now().Add(-*logPrune))
			}
		}()
	}

	<-quit
	log.Println("Shutting down...")

	httpServer.Close()
	udpConn.Close()
	if tcpListener != nil {
		tcpListener.Close()
	}

	wg.Wait()
	log.Println("Shutdown complete")
}

func handleTCPConn(conn *net.TCPConn, handler *dns.Handler) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	lenBuf := make([]byte, 2)
	if _, err := conn.Read(lenBuf); err != nil {
		return
	}
	length := int(lenBuf[0])<<8 | int(lenBuf[1])
	if length < 12 || length > 4096 {
		return
	}

	data := make([]byte, length)
	if _, err := conn.Read(data); err != nil {
		return
	}

	clientIP := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	handler.HandleTCP(conn, data, clientIP)
}
