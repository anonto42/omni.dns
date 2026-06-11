package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/sohidul/dns-server/internal/db"
	"github.com/sohidul/dns-server/internal/dns"
)

func RegisterRoutes(r chi.Router, database *db.DB, dnsHandler *dns.Handler) {
	h := NewHandler(database, dnsHandler)

	r.Get("/health", h.Health)
	r.Post("/api/login", h.Login)

	r.Route("/api", func(r chi.Router) {
		r.Use(Auth(database))

		r.Get("/status", h.GetStatus)
		r.Get("/logs", h.GetLogsFiltered)
		r.Delete("/logs", h.ClearLogs)
		r.Get("/records", h.GetRecords)
		r.Post("/records", h.AddRecord)
		r.Delete("/records", h.DeleteRecord)
		r.Get("/blocklist", h.GetBlocklist)
		r.Post("/blocklist", h.AddToBlocklist)
		r.Delete("/blocklist", h.RemoveFromBlocklist)
		r.Get("/settings", h.GetSettings)
		r.Put("/settings", h.SaveSettings)
		r.Delete("/session", h.Logout)
		r.Put("/password", h.ChangePassword)
		r.Get("/profile", h.GetProfile)
		r.Put("/profile", h.UpdateProfile)
		r.Get("/notifications", h.GetNotifications)
		r.Put("/notifications/read", h.MarkNotificationsRead)
		r.Delete("/notifications", h.DeleteNotifications)
		r.Get("/steering", h.GetSteeringRules)
		r.Post("/steering", h.AddSteeringRule)
		r.Put("/steering", h.UpdateSteeringRule)
		r.Delete("/steering", h.DeleteSteeringRule)
	})
}
