package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/sohidul/dns-server/internal/db"
	"github.com/sohidul/dns-server/internal/dns"
)

func RegisterRoutes(r chi.Router, database *db.DB, dnsHandler *dns.Handler) {
	h := NewHandler(database, dnsHandler)

	r.Get("/health", h.Health)

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", h.GetStatus)
		r.Get("/logs", h.GetLogs)
		r.Delete("/logs", h.ClearLogs)
		r.Get("/records", h.GetRecords)
		r.Post("/records", h.AddRecord)
		r.Delete("/records", h.DeleteRecord)
		r.Get("/blocklist", h.GetBlocklist)
		r.Post("/blocklist", h.AddToBlocklist)
		r.Delete("/blocklist", h.RemoveFromBlocklist)
	})
}
