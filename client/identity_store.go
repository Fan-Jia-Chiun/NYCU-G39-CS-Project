package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	identityStoreDirName  = "PropertyAuction"
	identityStoreFileName = "identity.json"
)

type localIdentityCache struct {
	UserDID   string `json:"userDID"`
	BuyerDID  string `json:"buyerDID,omitempty"`
	SellerDID string `json:"sellerDID,omitempty"`
}

func defaultIdentityStorePath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("PROPERTY_AUCTION_IDENTITY_PATH")); override != "" {
		return override, nil
	}

	if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
		if runtime.GOOS == "windows" {
			return filepath.Join(appData, identityStoreDirName, identityStoreFileName), nil
		}
		return filepath.Join(fromWindowsPath(appData), identityStoreDirName, identityStoreFileName), nil
	}

	if runtime.GOOS == "windows" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("failed to locate user config directory: %w", err)
		}
		return filepath.Join(configDir, identityStoreDirName, identityStoreFileName), nil
	}

	if appData := detectWSLAppData(); appData != "" {
		return filepath.Join(appData, identityStoreDirName, identityStoreFileName), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to locate user config directory: %w", err)
	}

	return filepath.Join(configDir, identityStoreDirName, identityStoreFileName), nil
}

func defaultIdentityStoreDisplayPath() (string, error) {
	path, err := defaultIdentityStorePath()
	if err != nil {
		return "", err
	}

	return displayIdentityStorePath(path), nil
}

func loadLocalIdentityCache() (localIdentityCache, error) {
	path, err := defaultIdentityStorePath()
	if err != nil {
		return localIdentityCache{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return localIdentityCache{}, err
	}

	var cache localIdentityCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return localIdentityCache{}, err
	}

	cache.UserDID = strings.TrimSpace(cache.UserDID)
	cache.BuyerDID = strings.TrimSpace(cache.BuyerDID)
	cache.SellerDID = strings.TrimSpace(cache.SellerDID)
	return cache, nil
}

func saveLocalIdentityCache(cache localIdentityCache) error {
	cache.UserDID = strings.TrimSpace(cache.UserDID)
	cache.BuyerDID = strings.TrimSpace(cache.BuyerDID)
	cache.SellerDID = strings.TrimSpace(cache.SellerDID)
	if cache.UserDID == "" {
		return fmt.Errorf("userDID is required")
	}

	path, err := defaultIdentityStorePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create identity directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode identity cache: %w", err)
	}
	data = append(data, '\n')

	tmpFile, err := os.CreateTemp(dir, "."+identityStoreFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary identity file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temporary identity file: %w", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to set identity file permissions: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temporary identity file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary identity file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to replace identity file: %w", err)
	}
	cleanup = false

	return nil
}

func displayIdentityStorePath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[0] == "" && parts[1] == "mnt" && len(parts[2]) == 1 {
		drive := strings.ToUpper(parts[2])
		return drive + ":\\" + strings.Join(parts[3:], "\\")
	}

	if len(path) >= 3 && path[1] == ':' && (path[2] == '/' || path[2] == '\\') {
		return strings.ReplaceAll(path, "/", "\\")
	}

	return strings.ReplaceAll(path, "/", string(os.PathSeparator))
}

func fromWindowsPath(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		drive := strings.ToLower(path[:1])
		rest := strings.ReplaceAll(path[2:], "\\", "/")
		return filepath.Join("/mnt", drive, rest)
	}

	return path
}

func detectWSLAppData() string {
	if userProfile := strings.TrimSpace(os.Getenv("USERPROFILE")); userProfile != "" {
		candidate := filepath.Join(fromWindowsPath(userProfile), "AppData", "Roaming")
		if isDir(candidate) {
			return candidate
		}
	}

	names := []string{
		strings.TrimSpace(os.Getenv("USERNAME")),
		"user",
	}
	if currentUser, err := user.Current(); err == nil {
		names = append(names, filepath.Base(currentUser.HomeDir), currentUser.Username)
	}

	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(filepath.Base(name))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		candidate := filepath.Join("/mnt/c/Users", name, "AppData", "Roaming")
		if isDir(candidate) {
			return candidate
		}
	}

	return ""
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
