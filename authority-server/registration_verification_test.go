package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"testing"
	"time"
)

func TestBuildIdentityRegisterCredential(t *testing.T) {
	got := buildIdentityRegisterCredential(
		userTypeGeneral,
		"Alice",
		"",
		"A123456789",
		"alice@example.com",
		"0912345678",
		"public-key-text",
		"2026-07-29T08:30:00Z",
	)
	want := "IDENTITY_REGISTER|0|Alice||A123456789|alice@example.com|0912345678|public-key-text|2026-07-29T08:30:00Z"
	if got != want {
		t.Fatalf("credential = %q, want %q", got, want)
	}
}

func TestVerifyIdentityRegisterRequest(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequest(t, now)

	if err := verifyIdentityRegisterRequest(req, now); err != nil {
		t.Fatalf("verifyIdentityRegisterRequest() error = %v", err)
	}
}

func TestVerifyIdentityRegisterRequestAcceptsLogisticsUserType(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequestWithUserType(t, now, userTypeLogistics)

	if err := verifyIdentityRegisterRequest(req, now); err != nil {
		t.Fatalf("verifyIdentityRegisterRequest() error = %v", err)
	}
}

func TestVerifyIdentityRegisterRequestRejectsModifiedField(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequest(t, now)
	req.Email = "mallory@example.com"

	err := verifyIdentityRegisterRequest(req, now)
	if err == nil || err.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %+v, want unauthorized", err)
	}
}

func TestVerifyIdentityRegisterRequestRejectsModifiedLogisticsCompanyName(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequestWithUserType(t, now, userTypeLogistics)
	req.LogisticsCompanyName = "Modified Delivery"

	err := verifyIdentityRegisterRequest(req, now)
	if err == nil || err.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %+v, want unauthorized", err)
	}
}

func TestVerifyIdentityRegisterRequestRejectsModifiedUserType(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequest(t, now)
	req.UserType = userTypeLogistics

	err := verifyIdentityRegisterRequest(req, now)
	if err == nil || err.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %+v, want unauthorized", err)
	}
}

func TestVerifyIdentityRegisterRequestRejectsExpiredTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequest(t, now.Add(-61*time.Second))

	err := verifyIdentityRegisterRequest(req, now)
	if err == nil || err.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %+v, want unauthorized", err)
	}
}

func TestVerifyIdentityRegisterRequestRejectsInvalidPublicKey(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequest(t, now)
	req.PublicKey = "not-base64"

	err := verifyIdentityRegisterRequest(req, now)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %+v, want bad request", err)
	}
}

func TestVerifyIdentityRegisterRequestRejectsInvalidSignatureEncoding(t *testing.T) {
	now := time.Date(2026, 7, 29, 8, 30, 0, 0, time.UTC)
	req := signedIdentityRegisterRequest(t, now)
	req.Signature = "not-base64!"

	err := verifyIdentityRegisterRequest(req, now)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %+v, want bad request", err)
	}
}

func TestValidateRegisterRequestRejectsInvalidUserType(t *testing.T) {
	err := validateRegisterRequest(RegisterRequest{UserType: 2})
	if err == nil {
		t.Fatal("expected userType validation error")
	}
}

func TestValidateRegisterRequestRejectsMissingLogisticsCompanyName(t *testing.T) {
	err := validateRegisterRequest(RegisterRequest{UserType: userTypeLogistics})
	if err == nil {
		t.Fatal("expected logisticsCompanyName validation error")
	}
}

func signedIdentityRegisterRequest(t *testing.T, timestamp time.Time) RegisterRequest {
	return signedIdentityRegisterRequestWithUserType(t, timestamp, userTypeGeneral)
}

func signedIdentityRegisterRequestWithUserType(t *testing.T, timestamp time.Time, userType uint) RegisterRequest {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}

	logisticsCompanyName := ""
	if userType == userTypeLogistics {
		logisticsCompanyName = "Fast Delivery"
	}

	req := RegisterRequest{
		UserType:             userType,
		UserName:             "Alice",
		LogisticsCompanyName: logisticsCompanyName,
		IDCardNumber:         "A123456789",
		Email:                "alice@example.com",
		Phone:                "0912345678",
		PublicKey:            base64.StdEncoding.EncodeToString(publicKeyDER),
		Timestamp:            timestamp.UTC().Format(time.RFC3339),
	}
	credential := buildIdentityRegisterCredential(
		req.UserType,
		req.UserName,
		req.LogisticsCompanyName,
		req.IDCardNumber,
		req.Email,
		req.Phone,
		req.PublicKey,
		req.Timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("SignASN1() error = %v", err)
	}
	req.Signature = base64.StdEncoding.EncodeToString(signature)

	return req
}
