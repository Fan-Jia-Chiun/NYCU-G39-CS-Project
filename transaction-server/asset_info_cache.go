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

func cacheAssetInfoName(assetInfoCID string, assetName string) {
	assetInfoCID = normalizeIPFSCID(assetInfoCID)
	assetName = strings.TrimSpace(assetName)
	if assetInfoCID == "" || assetName == "" {
		return
	}

	assetInfoNameCache.Lock()
	defer assetInfoNameCache.Unlock()
	info := assetInfoNameCache.infos[assetInfoCID]
	info.AssetName = assetName
	assetInfoNameCache.infos[assetInfoCID] = info
}

func getCachedAssetInfoName(assetInfoCID string) string {
	assetInfoCID = normalizeIPFSCID(assetInfoCID)
	if assetInfoCID == "" {
		return ""
	}

	assetInfoNameCache.RLock()
	defer assetInfoNameCache.RUnlock()
	return assetInfoNameCache.infos[assetInfoCID].AssetName
}

func cacheAssetInfo(assetInfoCID string, info AssetInfo) {
	assetInfoCID = normalizeIPFSCID(assetInfoCID)
	info.AssetName = strings.TrimSpace(info.AssetName)
	info.PhotoURL = strings.TrimSpace(info.PhotoURL)
	if assetInfoCID == "" {
		return
	}

	assetInfoNameCache.Lock()
	defer assetInfoNameCache.Unlock()
	assetInfoNameCache.infos[assetInfoCID] = info
}

func getCachedAssetInfo(assetInfoCID string) (AssetInfo, bool) {
	assetInfoCID = normalizeIPFSCID(assetInfoCID)
	if assetInfoCID == "" {
		return AssetInfo{}, false
	}

	assetInfoNameCache.RLock()
	defer assetInfoNameCache.RUnlock()
	info, ok := assetInfoNameCache.infos[assetInfoCID]

	return info, ok
}
