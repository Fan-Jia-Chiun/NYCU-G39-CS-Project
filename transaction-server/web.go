package main

import (
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
