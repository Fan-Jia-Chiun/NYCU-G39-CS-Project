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

func TestBuildTransactionLaunchCredential(t *testing.T) {
	got := buildTransactionLaunchCredential(
		"did:nycu-g39:seller:abc",
		"asset-123",
		1,
		300000,
		"2026-07-30T08:30:00Z",
		"2026-07-29T08:30:00Z",
	)
	want := "TRANSACTION_LAUNCH|did:nycu-g39:seller:abc|asset-123|1|300000|2026-07-30T08:30:00Z|2026-07-29T08:30:00Z"
	if got != want {
		t.Fatalf("credential = %q, want %q", got, want)
	}
}

func TestNewTransactionLaunchRequestSignsCanonicalCredential(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req, err := newTransactionLaunchRequest(TransactionLaunchRequest{
		SessionToken:    "session-token",
		UserDID:         "did:nycu-g39:identity:user",
		SellerDID:       "did:nycu-g39:seller:abc",
		AssetID:         "asset-123",
		TransactionMode: 0,
		BasicPrice:      500000,
	}, privateKey, now)
	if err != nil {
		t.Fatalf("newTransactionLaunchRequest() error = %v", err)
	}

	if req.Timestamp != "2026-07-29T08:30:00Z" {
		t.Fatalf("timestamp = %q", req.Timestamp)
	}

	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	credential := buildTransactionLaunchCredential(
		req.SellerDID,
		req.AssetID,
		req.TransactionMode,
		req.BasicPrice,
		req.FinalizingTime,
		req.Timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	if !ecdsa.VerifyASN1(&privateKey.PublicKey, digest[:], signature) {
		t.Fatal("signature did not verify")
	}
}

func TestNewTransactionLaunchRequestRejectsSeparator(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	_, err = newTransactionLaunchRequest(TransactionLaunchRequest{
		SessionToken: "session-token",
		UserDID:      "did:nycu-g39:identity:user",
		SellerDID:    "did:nycu-g39:seller:bad|value",
		AssetID:      "asset-123",
		BasicPrice:   1,
	}, privateKey, time.Now())
	if err == nil {
		t.Fatal("expected separator validation error")
	}
}
