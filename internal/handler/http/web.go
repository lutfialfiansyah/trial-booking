package http

import (
	"embed"
	"io/fs"
	"net/http"
)

// webFS embeds the static frontend assets so they ship inside the single API
// binary and are served with zero CORS concerns.
//
//go:embed web/*
var webFS embed.FS

// webHandler returns an http.Handler that serves the embedded frontend. The
// index.html is served at "/" and app.js at "/app.js".
func webHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		// This can only happen if the embed directive is misconfigured.
		panic(err)
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html for the bare root path.
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, webFS, "web/index.html")
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}
