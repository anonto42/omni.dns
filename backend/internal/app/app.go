package app

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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

// App owns process-level resources and keeps main.go focused on orchestration.
type App struct {
	cfg        Config
	static     StaticFiles
	database   *db.DB
	dnsHandler *dns.Handler

	httpServer  *http.Server
	udpConn     *net.UDPConn
	tcpListener *net.TCPListener

	wg sync.WaitGroup
}

func New(cfg Config, static StaticFiles) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := ensureDBDir(cfg.DBPath); err != nil {
		return nil, err
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := database.InitAdmin(cfg.AdminEmail, cfg.AdminPass); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("init admin: %w", err)
	}

	cfg = applySavedSettings(cfg, database.GetSettings())
	dnsHandler := dns.NewHandler(database, &dns.Config{
		BlockNXDOMAIN: cfg.BlockNX,
		CacheSize:     cfg.CacheSize,
		Upstreams: []dns.Upstream{
			{Addr: cfg.UpstreamDNS, Timeout: 4 * time.Second, TLS: cfg.UpstreamTLS},
			{Addr: "8.8.8.8:853", Timeout: 6 * time.Second, TLS: cfg.UpstreamTLS},
		},
	})

	return &App{
		cfg:        cfg,
		static:     static,
		database:   database,
		dnsHandler: dnsHandler,
	}, nil
}

func (a *App) Run() error {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	if err := a.startUDP(done); err != nil {
		return err
	}
	a.startTCP(done)
	a.startHTTP()
	a.startLogPruner(done)

	<-signals
	slog.Info("shutting down")
	close(done)

	a.closeListeners()
	a.wg.Wait()
	slog.Info("shutdown complete")
	return nil
}

func (a *App) Close() {
	if a.database == nil {
		return
	}
	if err := a.database.Close(); err != nil {
		slog.Error("close database", "error", err)
	}
	a.database = nil
}

func (a *App) startHTTP() {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(api.CORS)

	api.RegisterRoutes(r, a.database, a.dnsHandler)
	a.registerStaticRoutes(r)

	a.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.cfg.HTTPPort),
		Handler: r,
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		slog.Info("listening", "protocol", "HTTP", "port", a.cfg.HTTPPort)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()
}

func (a *App) registerStaticRoutes(r chi.Router) {
	if a.static.Embedded {
		slog.Info("serving embedded static files")
		r.Handle("/*", spaFileServer(a.static.FileSystem))
		return
	}
	if a.cfg.StaticDir != "" {
		slog.Info("serving static files", "dir", a.cfg.StaticDir)
		r.Handle("/*", spaFileServer(http.Dir(a.cfg.StaticDir)))
	}
}

func (a *App) startLogPruner(done <-chan struct{}) {
	if a.cfg.LogPrune <= 0 {
		return
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				a.database.PruneLogs(time.Now().Add(-a.cfg.LogPrune))
			}
		}
	}()
}

func (a *App) closeListeners() {
	if a.httpServer != nil {
		if err := a.httpServer.Close(); err != nil {
			slog.Error("close http server", "error", err)
		}
	}
	if a.udpConn != nil {
		if err := a.udpConn.Close(); err != nil {
			slog.Error("close udp listener", "error", err)
		}
	}
	if a.tcpListener != nil {
		if err := a.tcpListener.Close(); err != nil {
			slog.Error("close tcp listener", "error", err)
		}
	}
}

func ensureDBDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func applySavedSettings(cfg Config, saved map[string]string) Config {
	if v, ok := saved["upstream_dns"]; ok && v != "" {
		cfg.UpstreamDNS = v
		cfg.UpstreamTLS = strings.HasSuffix(v, ":853")
	}
	if v, ok := saved["block_nxdomain"]; ok {
		cfg.BlockNX = v == "true"
	}
	return cfg
}
