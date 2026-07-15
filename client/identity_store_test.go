package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadLocalIdentityCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PropertyAuction", "identity.json")
	t.Setenv("PROPERTY_AUCTION_IDENTITY_PATH", path)

	want := localIdentityCache{
		IdentityDID: "did:nycu-g39:identity:test",
		BuyerDID:    "did:nycu-g39:buyer:test",
		SellerDID:   "did:nycu-g39:seller:test",
	}
	if err := saveLocalIdentityCache(want); err != nil {
		t.Fatalf("saveLocalIdentityCache() error = %v", err)
	}

	got, err := loadLocalIdentityCache()
	if err != nil {
		t.Fatalf("loadLocalIdentityCache() error = %v", err)
	}

	if got != want {
		t.Fatalf("cache = %+v, want %+v", got, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("identity.json is invalid JSON: %v", err)
	}
	for _, forbidden := range []string{"privateKey", "password", "sessionToken", "signature"} {
		if _, ok := raw[forbidden]; ok {
			t.Fatalf("identity.json contains forbidden field %q", forbidden)
		}
	}
}

func TestDefaultIdentityStorePathUsesAppData(t *testing.T) {
	t.Setenv("PROPERTY_AUCTION_IDENTITY_PATH", "")
	t.Setenv("APPDATA", `C:\Users\demo\AppData\Roaming`)

	path, err := defaultIdentityStorePath()
	if err != nil {
		t.Fatalf("defaultIdentityStorePath() error = %v", err)
	}

	wantSuffix := filepath.Join("PropertyAuction", "identity.json")
	if filepath.Base(filepath.Dir(path)) != "PropertyAuction" || filepath.Base(path) != "identity.json" {
		t.Fatalf("path = %q, want suffix %q", path, wantSuffix)
	}
}

func TestDisplayIdentityStorePathConvertsWSLPath(t *testing.T) {
	got := displayIdentityStorePath("/mnt/c/Users/demo/AppData/Roaming/PropertyAuction/identity.json")
	want := `C:\Users\demo\AppData\Roaming\PropertyAuction\identity.json`
	if got != want {
		t.Fatalf("displayIdentityStorePath() = %q, want %q", got, want)
	}
}
