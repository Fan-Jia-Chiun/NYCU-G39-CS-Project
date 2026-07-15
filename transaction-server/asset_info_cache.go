package main

import (
	"strings"
	"sync"
)

var assetInfoNameCache = struct {
	sync.RWMutex
	names map[string]string
}{
	names: map[string]string{},
}

func cacheAssetInfoName(assetInfoAddr string, assetName string) {
	assetInfoAddr = normalizeIPFSCID(assetInfoAddr)
	assetName = strings.TrimSpace(assetName)
	if assetInfoAddr == "" || assetName == "" {
		return
	}

	assetInfoNameCache.Lock()
	defer assetInfoNameCache.Unlock()
	assetInfoNameCache.names[assetInfoAddr] = assetName
}

func getCachedAssetInfoName(assetInfoAddr string) string {
	assetInfoAddr = normalizeIPFSCID(assetInfoAddr)
	if assetInfoAddr == "" {
		return ""
	}

	assetInfoNameCache.RLock()
	defer assetInfoNameCache.RUnlock()
	return assetInfoNameCache.names[assetInfoAddr]
}
