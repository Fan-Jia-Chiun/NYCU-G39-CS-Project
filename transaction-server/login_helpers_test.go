package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testUserDID = "did:nycu-g39:identity:test"

func TestLoginSignatureVerificationSuccess(t *testing.T) {
	privateKey, publicKeyText := newTestKeyPair(t)
	timestamp := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC).Format(time.RFC3339)
	signature := signTestLoginCredential(t, privateKey, testUserDID, timestamp)

	publicKey, err := parseECDSAPublicKey(publicKeyText)
	if err != nil {
		t.Fatalf("parseECDSAPublicKey() error = %v", err)
	}

	if err := verifyLoginSignature(publicKey, testUserDID, timestamp, signature); err != nil {
		t.Fatalf("verifyLoginSignature() error = %v", err)
	}
}

func TestLoginSignatureModifiedFails(t *testing.T) {
	privateKey, publicKeyText := newTestKeyPair(t)
	timestamp := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC).Format(time.RFC3339)
	signature := signTestLoginCredential(t, privateKey, testUserDID, timestamp)
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatal(err)
	}
	signatureBytes[len(signatureBytes)-1] ^= 0x01
	modifiedSignature := base64.StdEncoding.EncodeToString(signatureBytes)

	publicKey, err := parseECDSAPublicKey(publicKeyText)
	if err != nil {
		t.Fatalf("parseECDSAPublicKey() error = %v", err)
	}

	if err := verifyLoginSignature(publicKey, testUserDID, timestamp, modifiedSignature); err == nil {
		t.Fatal("verifyLoginSignature() expected modified signature to fail")
	}
}

func TestLoginUserDIDModifiedFails(t *testing.T) {
	privateKey, publicKeyText := newTestKeyPair(t)
	timestamp := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC).Format(time.RFC3339)
	signature := signTestLoginCredential(t, privateKey, testUserDID, timestamp)

	publicKey, err := parseECDSAPublicKey(publicKeyText)
	if err != nil {
		t.Fatalf("parseECDSAPublicKey() error = %v", err)
	}

	if err := verifyLoginSignature(publicKey, testUserDID+"-tampered", timestamp, signature); err == nil {
		t.Fatal("verifyLoginSignature() expected modified userDID to fail")
	}
}

func TestLoginTimestampValidation(t *testing.T) {
	now := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		timestamp string
		wantErr   error
	}{
		{
			name:      "valid",
			timestamp: now.Format(time.RFC3339),
		},
		{
			name:      "expired",
			timestamp: now.Add(-61 * time.Second).Format(time.RFC3339),
			wantErr:   errExpiredTimestamp,
		},
		{
			name:      "too far future",
			timestamp: now.Add(61 * time.Second).Format(time.RFC3339),
			wantErr:   errFutureTimestamp,
		},
		{
			name:      "bad format",
			timestamp: "2026/07/14 08:30:00",
			wantErr:   errInvalidTimestamp,
		},
		{
			name:      "not UTC canonical",
			timestamp: "2026-07-14T16:30:00+08:00",
			wantErr:   errInvalidTimestamp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoginTimestamp(tt.timestamp, now, defaultLoginTimestampSkew)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("validateLoginTimestamp() error = %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("validateLoginTimestamp() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoginHandlerFailureCases(t *testing.T) {
	privateKey, publicKeyText := newTestKeyPair(t)
	now := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC)

	originalNowUTC := nowUTC
	nowUTC = func() time.Time { return now }
	t.Cleanup(func() { nowUTC = originalNowUTC })

	validRequest := LoginRequest{
		UserDID:   testUserDID,
		Timestamp: now.Format(time.RFC3339),
		Signature: signTestLoginCredential(t, privateKey, testUserDID, now.Format(time.RFC3339)),
	}

	tests := []struct {
		name       string
		req        LoginRequest
		resolver   publicKeyResolver
		wantStatus int
	}{
		{
			name:       "success",
			req:        validRequest,
			resolver:   func(string) (string, error) { return publicKeyText, nil },
			wantStatus: http.StatusOK,
		},
		{
			name: "signature base64 format error",
			req: LoginRequest{
				UserDID:   testUserDID,
				Timestamp: validRequest.Timestamp,
				Signature: "not-base64!",
			},
			resolver:   func(string) (string, error) { return publicKeyText, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "public key not found empty result",
			req:        validRequest,
			resolver:   func(string) (string, error) { return "", nil },
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "userDID not found",
			req:        validRequest,
			resolver:   func(string) (string, error) { return "", fmt.Errorf("trading identity not found") },
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "public key format error",
			req:        validRequest,
			resolver:   func(string) (string, error) { return "not-base64", nil },
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "timestamp format error",
			req: LoginRequest{
				UserDID:   testUserDID,
				Timestamp: "bad-time",
				Signature: validRequest.Signature,
			},
			resolver:   func(string) (string, error) { return publicKeyText, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "expired timestamp",
			req: LoginRequest{
				UserDID:   testUserDID,
				Timestamp: now.Add(-61 * time.Second).Format(time.RFC3339),
				Signature: signTestLoginCredential(t, privateKey, testUserDID, now.Add(-61*time.Second).Format(time.RFC3339)),
			},
			resolver:   func(string) (string, error) { return publicKeyText, nil },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "future timestamp",
			req: LoginRequest{
				UserDID:   testUserDID,
				Timestamp: now.Add(61 * time.Second).Format(time.RFC3339),
				Signature: signTestLoginCredential(t, privateKey, testUserDID, now.Add(61*time.Second).Format(time.RFC3339)),
			},
			resolver:   func(string) (string, error) { return publicKeyText, nil },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "signature verification failure",
			req: LoginRequest{
				UserDID:   testUserDID + "-tampered",
				Timestamp: validRequest.Timestamp,
				Signature: validRequest.Signature,
			},
			resolver:   func(string) (string, error) { return publicKeyText, nil },
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := performLoginRequest(t, tt.resolver, tt.req)
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", status, tt.wantStatus, body)
			}
			if strings.Contains(body, publicKeyText) || strings.Contains(body, tt.req.Signature) {
				t.Fatalf("response leaked key or signature: %s", body)
			}
		})
	}
}

func TestLoginHandlerReturnsInitializationData(t *testing.T) {
	privateKey, publicKeyText := newTestKeyPair(t)
	now := time.Date(2026, 7, 14, 8, 30, 0, 0, time.UTC)

	originalNowUTC := nowUTC
	nowUTC = func() time.Time { return now }
	t.Cleanup(func() { nowUTC = originalNowUTC })

	req := LoginRequest{
		UserDID:   testUserDID,
		Timestamp: now.Format(time.RFC3339),
		Signature: signTestLoginCredential(t, privateKey, testUserDID, now.Format(time.RFC3339)),
	}

	handler := loginHandlerWithDependencies(
		func(string) (string, error) { return publicKeyText, nil },
		func(string) (*LoginInitializationData, error) {
			return &LoginInitializationData{
				AccountStatus: accountStatusAvailable,
				CreditScores: CreditScores{
					BuyerCreditScore:  80,
					SellerCreditScore: 90,
				},
				Assets: []AssetLoginInfo{
					{
						AssetID:       "asset-1",
						AssetAddr:     "ASSET_CERT:asset-1",
						AssetInfoAddr: "QmAssetInfo",
						LegalStatus:   0,
					},
				},
				ActiveTrades: []TradeLoginInfo{
					{
						TradeID:         7,
						TransactionRole: 2,
						IsActive:        true,
						TradeInfo: TradeInfo{
							TradeID:           7,
							AssetID:           "asset-1",
							TransactionStatus: 1,
						},
					},
				},
				HistoricalTrades: []TradeLoginInfo{},
				CurrentActiveTransactions: []TradeInfo{
					{TradeID: 9, AssetID: "asset-2", TransactionStatus: 1},
				},
			}, nil
		},
	)

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
	}

	var got LoginResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.Success || got.AccountStatus != accountStatusAvailable {
		t.Fatalf("unexpected login response: %+v", got)
	}
	if got.CreditScores.BuyerCreditScore != 80 || got.CreditScores.SellerCreditScore != 90 {
		t.Fatalf("credit scores = %+v", got.CreditScores)
	}
	if len(got.Assets) != 1 || got.Assets[0].AssetID != "asset-1" || got.Assets[0].AssetInfoAddr != "QmAssetInfo" || got.Assets[0].LegalStatus != 0 {
		t.Fatalf("assets = %+v", got.Assets)
	}
	if len(got.ActiveTrades) != 1 || got.ActiveTrades[0].TradeID != 7 {
		t.Fatalf("active trades = %+v", got.ActiveTrades)
	}
	if len(got.CurrentActiveTransactions) != 1 || got.CurrentActiveTransactions[0].TradeID != 9 {
		t.Fatalf("current active transactions = %+v", got.CurrentActiveTransactions)
	}
}

func TestTradeStatusActiveMapping(t *testing.T) {
	activeStatuses := []uint{0, 1, 2, 3, 4, 7, 8}
	for _, status := range activeStatuses {
		if !isTradeStatusActive(status) {
			t.Fatalf("status %d should be active", status)
		}
	}

	inactiveStatuses := []uint{5, 6, 9, 10}
	for _, status := range inactiveStatuses {
		if isTradeStatusActive(status) {
			t.Fatalf("status %d should be inactive", status)
		}
	}
}

func newTestKeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}

	return privateKey, base64.StdEncoding.EncodeToString(publicKeyDER)
}

func signTestLoginCredential(t *testing.T, privateKey *ecdsa.PrivateKey, userDID string, timestamp string) string {
	t.Helper()

	loginCredential := buildLoginCredential(userDID, timestamp)
	digest := sha256.Sum256([]byte(loginCredential))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("SignASN1() error = %v", err)
	}

	return base64.StdEncoding.EncodeToString(signature)
}

func performLoginRequest(t *testing.T, resolver publicKeyResolver, req LoginRequest) (int, string) {
	t.Helper()

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	loginHandlerWithPublicKeyResolver(resolver).ServeHTTP(response, request)

	return response.Code, response.Body.String()
}
