package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func TestBuildLoginCredential(t *testing.T) {
	got := buildLoginCredential("did:nycu-g39:identity:abc123", "2026-07-14T08:30:00Z")
	want := "LOGIN|did:nycu-g39:identity:abc123|2026-07-14T08:30:00Z"
	if got != want {
		t.Fatalf("buildLoginCredential() = %q, want %q", got, want)
	}
}

func TestNewLoginRequestSignsCredential(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	now := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC)
	req, err := newLoginRequest("did:nycu-g39:identity:abc123", privateKey, now)
	if err != nil {
		t.Fatalf("newLoginRequest() error = %v", err)
	}

	if req.Timestamp != "2026-07-14T08:30:00Z" {
		t.Fatalf("timestamp = %q", req.Timestamp)
	}

	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}

	loginCredential := buildLoginCredential(req.UserDID, req.Timestamp)
	digest := sha256.Sum256([]byte(loginCredential))
	if !ecdsa.VerifyASN1(&privateKey.PublicKey, digest[:], signature) {
		t.Fatal("signature did not verify")
	}
}
