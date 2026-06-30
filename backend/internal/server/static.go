package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) registerStaticRoutes(r chi.Router) {
	if s.static.Embedded {
		slog.Info("serving embedded static files")
		r.Handle("/", spaFileServer(s.static.FileSystem))
		r.Handle("/*", spaFileServer(s.static.FileSystem))
		return
	}
	if s.cfg.StaticDir != "" {
		slog.Info("serving static files", "dir", s.cfg.StaticDir)
		r.Handle("/", spaFileServer(http.Dir(s.cfg.StaticDir)))
		r.Handle("/*", spaFileServer(http.Dir(s.cfg.StaticDir)))
	}
}

// spaFileServer serves static assets, falling back to index.html for unknown
// paths so a client-side router (e.g. React Router) can handle the route.
func spaFileServer(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		defer f.Close()

		if stat, err := f.Stat(); err == nil && stat.IsDir() {
			idx, err := fs.Open(r.URL.Path + "/index.html")
			if err != nil {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			idx.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}
