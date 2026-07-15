package main

import (
	"net/http"
	"os"
	"path/filepath"
)

func staticWebDir() string {
	candidates := []string{
		envOrDefault("TRANSACTION_WEB_DIR", ""),
		filepath.Join("transaction-server", "web"),
		"web",
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return filepath.Join("transaction-server", "web")
}

func noCacheFileServer(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		fileServer.ServeHTTP(w, r)
	})
}
