package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewAssetRegistrationPayloadSignsCredential(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	dir := t.TempDir()
	photoPath := filepath.Join(dir, "photo.bin")
	photoBytes := []byte("photo bytes")
	if err := os.WriteFile(photoPath, photoBytes, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	now := time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	payload, err := newAssetRegistrationPayload(AssetRegistrationInput{
		SessionToken:  "session-token",
		IdentityDID:   "did:nycu-g39:identity:abc",
		AssetName:     "House",
		AssetLocation: "Taipei",
		Description:   "",
		PhotoPath:     photoPath,
	}, privateKey, now)
	if err != nil {
		t.Fatalf("newAssetRegistrationPayload() error = %v", err)
	}

	wantHash := assetSHA256Hex(photoBytes)
	if payload.Fields["photoHash"] != wantHash {
		t.Fatalf("photoHash = %q, want %q", payload.Fields["photoHash"], wantHash)
	}
	if payload.Fields["timestamp"] != "2026-07-15T08:30:00Z" {
		t.Fatalf("timestamp = %q", payload.Fields["timestamp"])
	}

	signature, err := base64.StdEncoding.DecodeString(payload.Fields["signature"])
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	credential := buildRegisterAssetCredential(
		payload.Fields["identityDID"],
		payload.Fields["assetName"],
		payload.Fields["assetLocation"],
		payload.Fields["description"],
		payload.Fields["photoHash"],
		payload.Fields["timestamp"],
	)
	digest := sha256.Sum256([]byte(credential))
	if !ecdsa.VerifyASN1(&privateKey.PublicKey, digest[:], signature) {
		t.Fatal("signature did not verify")
	}
}

func TestNewAssetRegistrationPayloadRejectsSeparator(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	_, err = newAssetRegistrationPayload(AssetRegistrationInput{
		SessionToken:  "session-token",
		IdentityDID:   "did:nycu-g39:identity:abc",
		AssetName:     "House|Bad",
		AssetLocation: "Taipei",
		PhotoPath:     "unused",
	}, privateKey, time.Now())
	if err == nil {
		t.Fatal("expected separator validation error")
	}
}
