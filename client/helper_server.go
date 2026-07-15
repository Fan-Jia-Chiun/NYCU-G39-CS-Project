package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type helperServer struct {
	privateKey *ecdsa.PrivateKey
}

type signLoginRequest struct {
	UserDID     string `json:"userDID"`
	IdentityDID string `json:"identityDID"`
}

type signLoginResponse struct {
	UserDID   string `json:"userDID"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

type signAssetRequest struct {
	IdentityDID   string `json:"identityDID"`
	AssetName     string `json:"assetName"`
	AssetLocation string `json:"assetLocation"`
	Description   string `json:"description"`
	PhotoHash     string `json:"photoHash"`
}

type signAssetResponse struct {
	IdentityDID string `json:"identityDID"`
	Timestamp   string `json:"timestamp"`
	Signature   string `json:"signature"`
}

func runHelperServer(addr string, keyDir string) error {
	privateKey, err := readPrivateKey(privateKeyPath(keyDir))
	if err != nil {
		return fmt.Errorf("failed to read local private key: %w", err)
	}

	server := helperServer{privateKey: privateKey}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", withCORS(server.healthHandler))
	mux.HandleFunc("/sign/login", withCORS(server.signLoginHandler))
	mux.HandleFunc("/sign/register-asset", withCORS(server.signAssetHandler))

	log.Printf("Client signing helper listening on: %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s helperServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		helperError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	helperJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s helperServer) signLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helperError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req signLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helperError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userDID := strings.TrimSpace(firstNonEmpty(req.UserDID, req.IdentityDID))
	loginReq, err := newLoginRequest(userDID, s.privateKey, nowUTC())
	if err != nil {
		helperError(w, http.StatusBadRequest, err.Error())
		return
	}

	helperJSON(w, http.StatusOK, signLoginResponse{
		UserDID:   loginReq.UserDID,
		Timestamp: loginReq.Timestamp,
		Signature: loginReq.Signature,
	})
}

func (s helperServer) signAssetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helperError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req signAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helperError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.IdentityDID = strings.TrimSpace(req.IdentityDID)
	req.AssetName = strings.TrimSpace(req.AssetName)
	req.AssetLocation = strings.TrimSpace(req.AssetLocation)
	req.Description = strings.TrimSpace(req.Description)
	req.PhotoHash = strings.TrimSpace(req.PhotoHash)

	if req.IdentityDID == "" {
		helperError(w, http.StatusBadRequest, "identityDID is required")
		return
	}
	if req.AssetName == "" {
		helperError(w, http.StatusBadRequest, "assetName is required")
		return
	}
	if req.AssetLocation == "" {
		helperError(w, http.StatusBadRequest, "assetLocation is required")
		return
	}
	if req.PhotoHash == "" {
		helperError(w, http.StatusBadRequest, "photoHash is required")
		return
	}

	fields := map[string]string{
		"identityDID":   req.IdentityDID,
		"assetName":     req.AssetName,
		"assetLocation": req.AssetLocation,
		"description":   req.Description,
		"photoHash":     req.PhotoHash,
	}
	for name, value := range fields {
		if err := validateAssetCredentialField(name, value); err != nil {
			helperError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	timestamp := nowUTC().UTC().Format(time.RFC3339)
	credential := buildRegisterAssetCredential(
		req.IdentityDID,
		req.AssetName,
		req.AssetLocation,
		req.Description,
		req.PhotoHash,
		timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	signature, err := ecdsa.SignASN1(rand.Reader, s.privateKey, digest[:])
	if err != nil {
		helperError(w, http.StatusInternalServerError, "failed to sign credential")
		return
	}

	helperJSON(w, http.StatusOK, signAssetResponse{
		IdentityDID: req.IdentityDID,
		Timestamp:   timestamp,
		Signature:   base64.StdEncoding.EncodeToString(signature),
	})
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func helperJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write helper response: %v", err)
	}
}

func helperError(w http.ResponseWriter, statusCode int, message string) {
	helperJSON(w, statusCode, map[string]any{
		"success": false,
		"message": message,
	})
}
