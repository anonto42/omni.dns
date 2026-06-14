package app

import "net/http"

type StaticFiles struct {
	Embedded   bool
	FileSystem http.FileSystem
}

// spaFileServer serves static assets normally; for any path that is not a real
// file it falls back to index.html so React Router can handle the route.
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

		stat, err := f.Stat()
		if err == nil && stat.IsDir() {
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
