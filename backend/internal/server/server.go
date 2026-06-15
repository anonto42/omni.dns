// Package server wires together configuration, storage, the DNS resolver, and
// the HTTP API into a runnable process, and owns their lifecycle.
package server

import (
	"context"
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

	db "github.com/sohidul/dns-server/internal/infrastructure/persistence"
	appdns "github.com/sohidul/dns-server/internal/interfaces/dns"
	httpapi "github.com/sohidul/dns-server/internal/interfaces/http"
	"github.com/sohidul/dns-server/internal/interfaces/http/handlers"
	apimw "github.com/sohidul/dns-server/internal/interfaces/http/middleware"
	blocklistapp "github.com/sohidul/dns-server/internal/modules/blocklist/application"
	blocklistinfra "github.com/sohidul/dns-server/internal/modules/blocklist/infrastructure"
	notificationinfra "github.com/sohidul/dns-server/internal/modules/notification/infrastructure"
	recordsapp "github.com/sohidul/dns-server/internal/modules/records/application"
	recordsinfra "github.com/sohidul/dns-server/internal/modules/records/infrastructure"
	resolver "github.com/sohidul/dns-server/internal/modules/resolver/engine"
	"github.com/sohidul/dns-server/internal/modules/resolver/engine/arp"
	"github.com/sohidul/dns-server/internal/modules/resolver/engine/cache"
	"github.com/sohidul/dns-server/internal/modules/resolver/engine/forwarder"
	steeringapp "github.com/sohidul/dns-server/internal/modules/steering/application"
	steeringinfra "github.com/sohidul/dns-server/internal/modules/steering/infrastructure"
	"github.com/sohidul/dns-server/internal/shared/config"
)

// StaticFiles describes the UI assets to serve at "/".
type StaticFiles struct {
	Embedded   bool
	FileSystem http.FileSystem
}

// Server owns process-level resources.
type Server struct {
	cfg      config.Config
	static   StaticFiles
	database *db.DB
	resolver *resolver.Resolver
	handler  *appdns.Handler
	arp      *arp.Cache
	cache    *cache.Cache
	pool     *forwarder.Pool
	api      *handlers.Handler

	httpServer  *http.Server
	udpConn     *net.UDPConn
	tcpListener *net.TCPListener

	wg sync.WaitGroup
}

// New builds a fully wired Server. It opens the database, bootstraps the admin
// user, applies saved settings, and assembles the resolver pipeline.
func New(cfg config.Config, static StaticFiles) (*Server, error) {
	if err := ensureDBDir(cfg.DBPath); err != nil {
		return nil, err
	}

	database, err := db.Open(cfg.DBPath, db.Options{
		LogFlushInterval: cfg.LogFlushInterval,
		LogFlushSize:     cfg.LogFlushSize,
		SessionTTL:       cfg.SessionTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := database.InitAdmin(cfg.AdminEmail, cfg.AdminPass); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("init admin: %w", err)
	}

	cfg = applySavedSettings(cfg, database.GetSettings())

	arpCache := arp.NewCache(cfg.EnableMACLookup, cfg.ARPRefresh)
	dnsCache := cache.New(cfg.CacheSize)
	pool := forwarder.NewPool([]forwarder.Upstream{
		{Addr: cfg.UpstreamDNS, Timeout: 4 * time.Second, TLS: cfg.UpstreamTLS},
		{Addr: "8.8.8.8:853", Timeout: 6 * time.Second, TLS: cfg.UpstreamTLS},
	})

	res := resolver.New(resolver.Deps{
		Blocklist: database,
		Records:   database,
		Steering:  database,
		Logger:    database,
		MAC:       arpCache,
		Cache:     dnsCache,
		Pool:      pool,
		BlockNX:   cfg.BlockNX,
	})

	notifier := notificationinfra.NewNotifications(database)
	apiHandler := handlers.New(
		database,
		res,
		recordsapp.NewService(recordsinfra.NewRecords(database), notifier),
		blocklistapp.NewService(blocklistinfra.NewBlocklist(database), notifier),
		steeringapp.NewService(steeringinfra.NewSteering(database), notifier),
	)

	return &Server{
		cfg:      cfg,
		static:   static,
		database: database,
		resolver: res,
		handler:  appdns.NewHandler(res),
		arp:      arpCache,
		cache:    dnsCache,
		pool:     pool,
		api:      apiHandler,
	}, nil
}

// Run starts all listeners and background workers, then blocks until a SIGINT
// or SIGTERM triggers a graceful shutdown.
func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.arp.Start(ctx)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	if err := s.startUDP(ctx); err != nil {
		return err
	}
	s.startTCP(ctx)
	s.startHTTP()
	s.startLogPruner(ctx)
	s.startSessionJanitor(ctx)

	<-signals
	slog.Info("shutting down")
	cancel()

	s.closeListeners()
	s.cache.Close()
	s.pool.Close()
	s.wg.Wait()
	slog.Info("shutdown complete")
	return nil
}

// Close releases the database. Safe to call once after Run returns.
func (s *Server) Close() {
	if s.database == nil {
		return
	}
	if err := s.database.Close(); err != nil {
		slog.Error("close database", "error", err)
	}
	s.database = nil
}

func (s *Server) startHTTP() {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(apimw.RequestID)
	r.Use(apimw.CORS(s.cfg.AllowedOrigin))

	httpapi.RegisterRoutes(r, s.database, s.api)
	s.registerStaticRoutes(r)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.HTTPPort),
		Handler: r,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		slog.Info("listening", "protocol", "HTTP", "port", s.cfg.HTTPPort)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()
}

func (s *Server) startLogPruner(ctx context.Context) {
	if s.cfg.LogPrune <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.database.PruneLogs(time.Now().Add(-s.cfg.LogPrune))
			}
		}
	}()
}

// startSessionJanitor periodically deletes expired sessions so the table does
// not grow without bound.
func (s *Server) startSessionJanitor(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.database.SweepExpiredSessions() // sweep once at startup
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.database.SweepExpiredSessions()
			}
		}
	}()
}

func ensureDBDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func applySavedSettings(cfg config.Config, saved map[string]string) config.Config {
	if v, ok := saved["upstream_dns"]; ok && v != "" {
		cfg.UpstreamDNS = v
		cfg.UpstreamTLS = strings.HasSuffix(v, ":853")
	}
	if v, ok := saved["block_nxdomain"]; ok {
		cfg.BlockNX = v == "true"
	}
	return cfg
}
