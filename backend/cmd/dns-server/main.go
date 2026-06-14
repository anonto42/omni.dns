package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/sohidul/dns-server/internal/api"
	"github.com/sohidul/dns-server/internal/db"
	"github.com/sohidul/dns-server/internal/dns"
)

var (
	dnsPort     = flag.Int("dns-port", 53, "DNS server port")
	dnsAddr     = flag.String("dns-addr", "", "DNS bind address (default: all interfaces)")
	httpPort    = flag.Int("http-port", 8080, "HTTP API port")
	dbPath      = flag.String("db", "data/dns.db", "SQLite database path")
	blockNX     = flag.Bool("block-nxdomain", false, "Return NXDOMAIN for blocked domains")
	cacheSize   = flag.Int("cache-size", 1000, "DNS cache size")
	upstreamDNS = flag.String("upstream", "1.1.1.1:853", "Upstream DNS server (host:port)")
	upstreamTLS = flag.Bool("upstream-tls", true, "Use DNS-over-TLS for upstream queries")
	staticDir   = flag.String("static", "", "Directory with static files to serve at /")
	logPrune    = flag.Duration("log-prune", 0, "Auto-prune logs older than this (e.g. 72h)")
	logFormat   = flag.String("log-format", "text", "Log format: text or json")
	logLevel    = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	adminEmail  = flag.String("admin-email", "", "Initial admin email (or OMNIDNS_ADMIN_EMAIL)")
	adminPass   = flag.String("admin-password", "", "Initial admin password (or OMNIDNS_ADMIN_PASSWORD)")
)

// @title           ESP32 DNS Server API
// @version         1.0
// @description     API for managing custom DNS records and blocklists.
// @host            localhost:8080
// @BasePath        /api

func main() {
	flag.Parse()

	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var logHandler slog.Handler
	if *logFormat == "json" {
		logHandler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		logHandler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(logHandler))

	if err := os.MkdirAll("data", 0755); err != nil {
		slog.Error("create data directory", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	email, password := initialAdminCredentials()
	if err := database.InitAdmin(email, password); err != nil {
		slog.Error("init admin", "error", err)
	}
	defer database.Close()

	// Load persisted settings — override CLI defaults if user has saved preferences.
	savedSettings := database.GetSettings()
	if v, ok := savedSettings["upstream_dns"]; ok && v != "" {
		*upstreamDNS = v
		*upstreamTLS = strings.HasSuffix(v, ":853")
	}
	if v, ok := savedSettings["block_nxdomain"]; ok {
		*blockNX = v == "true"
	}

	dnsCfg := &dns.Config{
		BlockNXDOMAIN: *blockNX,
		CacheSize:     *cacheSize,
		Upstreams: []dns.Upstream{
			{Addr: *upstreamDNS, Timeout: 4 * time.Second, TLS: *upstreamTLS},
			{Addr: "8.8.8.8:853", Timeout: 6 * time.Second, TLS: *upstreamTLS},
		},
	}
	dnsHandler := dns.NewHandler(database, dnsCfg)

	var wg sync.WaitGroup
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// --- UDP DNS ---
	udpConn, err := listenUDPWithRetry(*dnsAddr, *dnsPort)
	if err != nil {
		slog.Error("listen UDP", "port", *dnsPort, "error", err)
		os.Exit(1)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("listening", "protocol", "UDP", "port", *dnsPort)
		buf := make([]byte, 1500)
		for {
			select {
			case <-quit:
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
			go dnsHandler.HandleUDP(udpConn, client, pkt)
		}
	}()

	// --- TCP DNS ---
	tcpAddr := net.TCPAddr{IP: net.ParseIP(*dnsAddr), Port: *dnsPort}
	tcpListener, err := net.ListenTCP("tcp", &tcpAddr)
	if err != nil {
		slog.Warn("TCP listener unavailable (non-fatal)", "port", *dnsPort, "error", err)
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slog.Info("listening", "protocol", "TCP", "port", *dnsPort)
			for {
				select {
				case <-quit:
					tcpListener.Close()
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
				go handleTCPConn(conn, dnsHandler)
			}
		}()
	}

	// --- HTTP API ---
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(api.CORS)

	api.RegisterRoutes(r, database, dnsHandler)

	if isEmbedded() {
		slog.Info("serving embedded static files")
		r.Handle("/*", spaFileServer(getFileSystem()))
	} else if *staticDir != "" {
		slog.Info("serving static files", "dir", *staticDir)
		r.Handle("/*", spaFileServer(http.Dir(*staticDir)))
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", *httpPort),
		Handler: r,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("listening", "protocol", "HTTP", "port", *httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
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
	slog.Info("shutting down")

	httpServer.Close()
	udpConn.Close()
	if tcpListener != nil {
		tcpListener.Close()
	}

	wg.Wait()
	slog.Info("shutdown complete")
}

func initialAdminCredentials() (string, string) {
	email := strings.TrimSpace(firstNonEmpty(*adminEmail, os.Getenv("OMNIDNS_ADMIN_EMAIL"), "admin@omnidns.local"))
	password := firstNonEmpty(*adminPass, os.Getenv("OMNIDNS_ADMIN_PASSWORD"))

	if strings.TrimSpace(password) == "" {
		slog.Error("initial admin password is required; set OMNIDNS_ADMIN_PASSWORD or --admin-password")
		os.Exit(1)
	}
	if len(password) < 8 {
		slog.Error("initial admin password must be at least 8 characters")
		os.Exit(1)
	}

	return email, password
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func handleTCPConn(conn *net.TCPConn, handler *dns.Handler) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		slog.Error("failed to set tcp conn deadline", "error", err)
	}

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

// fileServer is kept for reference but no longer used in main.
func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}

// spaFileServer serves static assets normally; for any path that is not a
// real file it falls back to index.html so that React Router can handle the
// route client-side (fixes hard-reload 404 on paths like /steering).
func spaFileServer(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested path
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			// Path does not exist as a file — serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		defer f.Close()

		// Path exists. If it's a directory without index.html, also fall back.
		stat, err := f.Stat()
		if err == nil && stat.IsDir() {
			// Check for index.html inside the directory
			idx, err2 := fs.Open(r.URL.Path + "/index.html")
			if err2 != nil {
				// No index.html in directory — serve root index.html
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			idx.Close()
		}

		fileServer.ServeHTTP(w, r)
	})
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
