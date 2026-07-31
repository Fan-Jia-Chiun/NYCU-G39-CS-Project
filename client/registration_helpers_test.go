package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestBuildIdentityRegisterCredential(t *testing.T) {
	got := buildIdentityRegisterCredential(
		userTypeGeneral,
		"Alice",
		"A123456789",
		"alice@example.com",
		"0912345678",
		"public-key-text",
		"2026-07-29T08:30:00Z",
	)
	want := "IDENTITY_REGISTER|0|Alice|A123456789|alice@example.com|0912345678|public-key-text|2026-07-29T08:30:00Z"
	if got != want {
		t.Fatalf("credential = %q, want %q", got, want)
	}
}

func TestNewSignedRegisterRequestSignsCanonicalCredential(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicKey, err := encodePublicKeyText(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("encodePublicKeyText() error = %v", err)
	}

	req, err := newSignedRegisterRequest(RegisterRequest{
		UserType:     userTypeGeneral,
		UserName:     "Alice",
		IDCardNumber: "A123456789",
		Email:        "alice@example.com",
		Phone:        "0912345678",
		PublicKey:    publicKey,
	}, privateKey, time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("newSignedRegisterRequest() error = %v", err)
	}

	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	credential := buildIdentityRegisterCredential(
		req.UserType,
		req.UserName,
		req.IDCardNumber,
		req.Email,
		req.Phone,
		req.PublicKey,
		req.Timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	if !ecdsa.VerifyASN1(&privateKey.PublicKey, digest[:], signature) {
		t.Fatal("signature did not verify")
	}
}

func TestNewSignedRegisterRequestRejectsMismatchedPublicKey(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	otherPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	otherPublicKey, err := encodePublicKeyText(&otherPrivateKey.PublicKey)
	if err != nil {
		t.Fatalf("encodePublicKeyText() error = %v", err)
	}

	_, err = newSignedRegisterRequest(RegisterRequest{
		UserType:     userTypeGeneral,
		UserName:     "Alice",
		IDCardNumber: "A123456789",
		Email:        "alice@example.com",
		Phone:        "0912345678",
		PublicKey:    otherPublicKey,
	}, privateKey, time.Now())
	if err == nil {
		t.Fatal("expected public key mismatch error")
	}
}

func TestNewSignedRegisterRequestRejectsSeparator(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicKey, err := encodePublicKeyText(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("encodePublicKeyText() error = %v", err)
	}

	_, err = newSignedRegisterRequest(RegisterRequest{
		UserType:     userTypeGeneral,
		UserName:     "Alice|Admin",
		IDCardNumber: "A123456789",
		Email:        "alice@example.com",
		Phone:        "0912345678",
		PublicKey:    publicKey,
	}, privateKey, time.Now())
	if err == nil {
		t.Fatal("expected separator validation error")
	}
}

func TestNewSignedRegisterRequestIncludesLogisticsUserType(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicKey, err := encodePublicKeyText(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("encodePublicKeyText() error = %v", err)
	}

	req, err := newSignedRegisterRequest(RegisterRequest{
		UserType:     userTypeLogistics,
		UserName:     "Logistics User",
		IDCardNumber: "L123456789",
		Email:        "logistics@example.com",
		Phone:        "0912345678",
		PublicKey:    publicKey,
	}, privateKey, time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("newSignedRegisterRequest() error = %v", err)
	}

	credential := buildIdentityRegisterCredential(
		req.UserType,
		req.UserName,
		req.IDCardNumber,
		req.Email,
		req.Phone,
		req.PublicKey,
		req.Timestamp,
	)
	if !strings.HasPrefix(credential, "IDENTITY_REGISTER|1|") {
		t.Fatalf("credential = %q, want logistics userType", credential)
	}
}

func TestNewSignedRegisterRequestRejectsInvalidUserType(t *testing.T) {
	_, err := newSignedRegisterRequest(RegisterRequest{UserType: 2}, nil, time.Now())
	if err == nil {
		t.Fatal("expected userType validation error")
	}
}
