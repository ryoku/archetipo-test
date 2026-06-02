package main

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	webui "github.com/ryoku/kubegate/web"
)

// registerSPA mounts the embedded React app on r, falling back to index.html
// for any path that does not match a static asset (enables client-side routing).
func registerSPA(r *gin.Engine) {
	distFS, err := fs.Sub(webui.FS, "dist")
	if err != nil {
		// In dev mode the FS is zero-value (no prod build tag) — Vite serves the frontend.
		return
	}
	registerSPAFromFS(r, distFS)
}

func registerSPAFromFS(r *gin.Engine, distFS fs.FS) {
	fileServer := http.FileServer(http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		// Strip leading slash; an empty stripped path means "/" which must always
		// fall through to the SPA fallback — opening "" on an fs.FS opens the root
		// directory, not index.html.
		stripped := strings.TrimPrefix(c.Request.URL.Path, "/")
		if stripped != "" {
			if f, err := distFS.Open(stripped); err == nil {
				info, statErr := f.Stat()
				_ = f.Close()
				// Only serve regular files. Directories would cause http.FileServer to
				// return a 301 redirect or a listing, both of which leak the internal
				// asset directory structure.
				if statErr == nil && !info.IsDir() {
					fileServer.ServeHTTP(c.Writer, c.Request)
					return
				}
			}
		}

		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
