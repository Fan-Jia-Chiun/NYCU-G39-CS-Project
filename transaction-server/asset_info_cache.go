package main

import (
	"strings"
	"sync"
)

var assetInfoNameCache = struct {
	sync.RWMutex
	infos map[string]AssetInfo
}{
	infos: map[string]AssetInfo{},
}

func cacheAssetInfoName(assetInfoAddr string, assetName string) {
	assetInfoAddr = normalizeIPFSCID(assetInfoAddr)
	assetName = strings.TrimSpace(assetName)
	if assetInfoAddr == "" || assetName == "" {
		return
	}

	assetInfoNameCache.Lock()
	defer assetInfoNameCache.Unlock()
	info := assetInfoNameCache.infos[assetInfoAddr]
	info.AssetName = assetName
	assetInfoNameCache.infos[assetInfoAddr] = info
}

func getCachedAssetInfoName(assetInfoAddr string) string {
	assetInfoAddr = normalizeIPFSCID(assetInfoAddr)
	if assetInfoAddr == "" {
		return ""
	}

	assetInfoNameCache.RLock()
	defer assetInfoNameCache.RUnlock()
	return assetInfoNameCache.infos[assetInfoAddr].AssetName
}

func cacheAssetInfo(assetInfoAddr string, info AssetInfo) {
	assetInfoAddr = normalizeIPFSCID(assetInfoAddr)
	info.AssetName = strings.TrimSpace(info.AssetName)
	info.PhotoURL = strings.TrimSpace(info.PhotoURL)
	if assetInfoAddr == "" {
		return
	}

	assetInfoNameCache.Lock()
	defer assetInfoNameCache.Unlock()
	assetInfoNameCache.infos[assetInfoAddr] = info
}

func getCachedAssetInfo(assetInfoAddr string) (AssetInfo, bool) {
	assetInfoAddr = normalizeIPFSCID(assetInfoAddr)
	if assetInfoAddr == "" {
		return AssetInfo{}, false
	}

	assetInfoNameCache.RLock()
	defer assetInfoNameCache.RUnlock()
	info, ok := assetInfoNameCache.infos[assetInfoAddr]

	return info, ok
}
