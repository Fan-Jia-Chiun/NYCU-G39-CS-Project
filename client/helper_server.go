package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const localMaxAssetRequestSize = 11 << 20

type helperServer struct {
	keyDir      string
	registerURL string
	loginURL    string
	assetURL    string
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

type apiRegisterRequest struct {
	UserName     string `json:"userName"`
	IDCardNumber string `json:"idCardNumber"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
}

type apiRegisterResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	IdentityDID  string `json:"identityDID"`
	UserDID      string `json:"userDID"`
	PIMgrAddr    string `json:"pimgrAddr,omitempty"`
	BuyerDID     string `json:"buyerDID,omitempty"`
	SellerDID    string `json:"sellerDID,omitempty"`
	IdentityPath string `json:"identityPath,omitempty"`
}

type apiAssetRegistrationResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	AssetID       string `json:"assetID,omitempty"`
	AssetAddr     string `json:"assetAddr,omitempty"`
	PhotoCID      string `json:"photoCID,omitempty"`
	AssetInfoAddr string `json:"assetInfoAddr,omitempty"`
	PhotoHash     string `json:"photoHash,omitempty"`
}

type apiIdentityResponse struct {
	Success      bool   `json:"success"`
	CacheFound   bool   `json:"cacheFound"`
	IdentityDID  string `json:"identityDID"`
	BuyerDID     string `json:"buyerDID,omitempty"`
	SellerDID    string `json:"sellerDID,omitempty"`
	IdentityPath string `json:"identityPath,omitempty"`
	Message      string `json:"message,omitempty"`
}

func runHelperServer(addr string, keyDir string, registerURL string, loginURL string, assetURL string) error {
	server := helperServer{
		keyDir:      strings.TrimSpace(keyDir),
		registerURL: strings.TrimSpace(registerURL),
		loginURL:    strings.TrimSpace(loginURL),
		assetURL:    strings.TrimSpace(assetURL),
	}
	if server.keyDir == "" {
		return fmt.Errorf("key directory is required")
	}
	if server.registerURL == "" || server.loginURL == "" || server.assetURL == "" {
		return fmt.Errorf("authority and transaction server URLs are required")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", withCORS(server.healthHandler))
	mux.HandleFunc("/api/identity", withCORS(server.apiIdentityHandler))
	mux.HandleFunc("/api/register", withCORS(server.apiRegisterHandler))
	mux.HandleFunc("/api/login", withCORS(server.apiLoginHandler))
	mux.HandleFunc("/api/assets/register", withCORS(server.apiAssetRegistrationHandler))
	mux.HandleFunc("/sign/login", withCORS(server.signLoginHandler))
	mux.HandleFunc("/sign/register-asset", withCORS(server.signAssetHandler))
	mux.Handle("/", http.FileServer(http.Dir(localClientWebDir())))

	log.Printf("Client Local Server listening on: %s", addr)
	log.Printf("Client Demo URL: http://%s/", addr)
	log.Printf("Authority register endpoint: %s", server.registerURL)
	log.Printf("Transaction login endpoint: %s", server.loginURL)
	log.Printf("Transaction asset endpoint: %s", server.assetURL)
	return http.ListenAndServe(addr, mux)
}

func (s helperServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		helperError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	helperJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s helperServer) apiIdentityHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		helperError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	identityPath, pathErr := defaultIdentityStoreDisplayPath()
	if pathErr != nil {
		log.Printf("failed to resolve local identity cache path: %v", pathErr)
	}

	cache, err := loadLocalIdentityCache()
	if err != nil {
		if os.IsNotExist(err) {
			helperJSON(w, http.StatusOK, apiIdentityResponse{
				Success:      true,
				CacheFound:   false,
				IdentityPath: identityPath,
				Message:      "identity cache not found",
			})
			return
		}
		log.Printf("failed to load local identity cache: %v", err)
		helperJSON(w, http.StatusOK, apiIdentityResponse{
			Success:      true,
			CacheFound:   false,
			IdentityPath: identityPath,
			Message:      "identity cache is unreadable",
		})
		return
	}

	helperJSON(w, http.StatusOK, apiIdentityResponse{
		Success:      true,
		CacheFound:   true,
		IdentityDID:  cache.IdentityDID,
		BuyerDID:     cache.BuyerDID,
		SellerDID:    cache.SellerDID,
		IdentityPath: identityPath,
		Message:      "loaded local identity cache",
	})
}

func (s helperServer) apiRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helperError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req apiRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		helperError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.UserName = strings.TrimSpace(req.UserName)
	req.IDCardNumber = strings.TrimSpace(req.IDCardNumber)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.UserName == "" || req.IDCardNumber == "" || req.Email == "" || req.Phone == "" {
		helperError(w, http.StatusBadRequest, "userName, idCardNumber, email, and phone are required")
		return
	}

	publicKey, err := ensureIdentityKeyPair(s.keyDir)
	if err != nil {
		helperError(w, http.StatusInternalServerError, "failed to prepare local identity key pair")
		return
	}

	body, statusCode, err := postJSONToServer(s.registerURL, RegisterRequest{
		UserName:     req.UserName,
		IDCardNumber: req.IDCardNumber,
		Email:        req.Email,
		Phone:        req.Phone,
		PublicKey:    publicKey,
	})
	if err != nil {
		helperError(w, http.StatusBadGateway, "failed to call authority server")
		return
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		forwardUpstreamError(w, statusCode, body, "authority server rejected registration")
		return
	}

	var authorityResp struct {
		Message   string `json:"message"`
		UserDID   string `json:"userDID"`
		PIMgrAddr string `json:"pimgrAddr"`
		BuyerDID  string `json:"buyerDID"`
		SellerDID string `json:"sellerDID"`
	}
	if err := json.Unmarshal(body, &authorityResp); err != nil {
		helperError(w, http.StatusBadGateway, "authority server returned invalid response")
		return
	}
	if authorityResp.UserDID == "" {
		helperError(w, http.StatusBadGateway, "authority server returned empty userDID")
		return
	}

	if err := saveLocalIdentityCache(localIdentityCache{
		IdentityDID: authorityResp.UserDID,
		BuyerDID:    authorityResp.BuyerDID,
		SellerDID:   authorityResp.SellerDID,
	}); err != nil {
		log.Printf("failed to save local identity cache: %v", err)
	}
	identityPath, err := defaultIdentityStoreDisplayPath()
	if err != nil {
		log.Printf("failed to resolve local identity cache path: %v", err)
	}

	helperJSON(w, http.StatusOK, apiRegisterResponse{
		Success:      true,
		Message:      firstNonEmpty(authorityResp.Message, "identity registered"),
		IdentityDID:  authorityResp.UserDID,
		UserDID:      authorityResp.UserDID,
		PIMgrAddr:    authorityResp.PIMgrAddr,
		BuyerDID:     authorityResp.BuyerDID,
		SellerDID:    authorityResp.SellerDID,
		IdentityPath: identityPath,
	})
}

func (s helperServer) apiLoginHandler(w http.ResponseWriter, r *http.Request) {
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
	if userDID == "" {
		if cache, err := loadLocalIdentityCache(); err == nil {
			userDID = cache.IdentityDID
		}
	}

	privateKey, err := readPrivateKey(privateKeyPath(s.keyDir))
	if err != nil {
		helperError(w, http.StatusBadRequest, "local private key is not ready; register first")
		return
	}
	loginReq, err := newLoginRequest(userDID, privateKey, nowUTC())
	if err != nil {
		helperError(w, http.StatusBadRequest, err.Error())
		return
	}

	body, statusCode, err := postJSONToServer(s.loginURL, loginReq)
	if err != nil {
		helperError(w, http.StatusBadGateway, "failed to call transaction server")
		return
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		forwardUpstreamError(w, statusCode, body, "transaction server rejected login")
		return
	}

	if err := saveIdentityCacheFromLoginResponse(body, userDID); err != nil {
		log.Printf("failed to sync local identity cache after login: %v", err)
	}
	if identityPath, err := defaultIdentityStoreDisplayPath(); err == nil {
		body = addJSONField(body, "identityPath", identityPath)
	} else {
		log.Printf("failed to resolve local identity cache path: %v", err)
	}

	writeRawJSON(w, http.StatusOK, body)
}

func (s helperServer) apiAssetRegistrationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helperError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(w, r.Body, localMaxAssetRequestSize)
	if err := r.ParseMultipartForm(localMaxAssetRequestSize); err != nil {
		helperError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	input := AssetRegistrationInput{
		SessionToken:  firstFormValue(r.MultipartForm, "sessionToken"),
		IdentityDID:   firstFormValue(r.MultipartForm, "identityDID"),
		AssetName:     firstFormValue(r.MultipartForm, "assetName"),
		AssetLocation: firstFormValue(r.MultipartForm, "assetLocation"),
		Description:   firstFormValue(r.MultipartForm, "description"),
	}

	photoFile, photoHeader, err := r.FormFile("photo")
	if err != nil {
		helperError(w, http.StatusBadRequest, "photo file is required")
		return
	}
	defer photoFile.Close()

	photoBytes, err := io.ReadAll(io.LimitReader(photoFile, localMaxAssetRequestSize+1))
	if err != nil {
		helperError(w, http.StatusBadRequest, "failed to read photo file")
		return
	}
	if len(photoBytes) == 0 {
		helperError(w, http.StatusBadRequest, "photo file is empty")
		return
	}
	if len(photoBytes) > localMaxAssetRequestSize {
		helperError(w, http.StatusBadRequest, "photo file is too large")
		return
	}

	privateKey, err := readPrivateKey(privateKeyPath(s.keyDir))
	if err != nil {
		helperError(w, http.StatusBadRequest, "local private key is not ready; register first")
		return
	}

	payload, err := newAssetRegistrationPayloadFromBytes(input, photoHeader.Filename, photoBytes, privateKey, nowUTC())
	if err != nil {
		helperError(w, http.StatusBadRequest, err.Error())
		return
	}

	body, statusCode, err := postMultipartToServer(s.assetURL, payload)
	if err != nil {
		helperError(w, http.StatusBadGateway, "failed to call transaction server")
		return
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		forwardUpstreamError(w, statusCode, body, "transaction server rejected asset registration")
		return
	}

	var assetResp apiAssetRegistrationResponse
	if err := json.Unmarshal(body, &assetResp); err != nil {
		helperError(w, http.StatusBadGateway, "transaction server returned invalid response")
		return
	}
	assetResp.PhotoHash = payload.Fields["photoHash"]
	helperJSON(w, http.StatusOK, assetResp)
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

	privateKey, err := readPrivateKey(privateKeyPath(s.keyDir))
	if err != nil {
		helperError(w, http.StatusBadRequest, "local private key is not ready; register first")
		return
	}

	userDID := strings.TrimSpace(firstNonEmpty(req.UserDID, req.IdentityDID))
	loginReq, err := newLoginRequest(userDID, privateKey, nowUTC())
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

	privateKey, err := readPrivateKey(privateKeyPath(s.keyDir))
	if err != nil {
		helperError(w, http.StatusBadRequest, "local private key is not ready; register first")
		return
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
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
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

func postJSONToServer(url string, payload any) ([]byte, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to encode request: %w", err)
	}

	client := http.Client{Timeout: 45 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return respBody, resp.StatusCode, nil
}

func postMultipartToServer(url string, payload AssetRegistrationPayload) ([]byte, int, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range payload.Fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, 0, fmt.Errorf("failed to write multipart field %s: %w", name, err)
		}
	}

	part, err := writer.CreateFormFile("photo", payload.FileName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create multipart photo field: %w", err)
	}
	if _, err := part.Write(payload.PhotoBytes); err != nil {
		return nil, 0, fmt.Errorf("failed to write multipart photo: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("failed to close multipart request: %w", err)
	}

	client := http.Client{Timeout: 75 * time.Second}
	resp, err := client.Post(url, writer.FormDataContentType(), &body)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return respBody, resp.StatusCode, nil
}

func localClientWebDir() string {
	candidates := []string{
		os.Getenv("CLIENT_WEB_DIR"),
		filepath.Join("..", "transaction-server", "web"),
		filepath.Join("transaction-server", "web"),
		"web",
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return filepath.Join("..", "transaction-server", "web")
}

func saveIdentityCacheFromLoginResponse(body []byte, fallbackIdentityDID string) error {
	var resp struct {
		IdentityDID string `json:"identityDID"`
		UserDID     string `json:"userDID"`
		BuyerDID    string `json:"buyerDID"`
		SellerDID   string `json:"sellerDID"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}

	identityDID := strings.TrimSpace(firstNonEmpty(resp.IdentityDID, resp.UserDID, fallbackIdentityDID))
	if identityDID == "" {
		return fmt.Errorf("login response did not include identityDID")
	}

	return saveLocalIdentityCache(localIdentityCache{
		IdentityDID: identityDID,
		BuyerDID:    resp.BuyerDID,
		SellerDID:   resp.SellerDID,
	})
}

func firstFormValue(form *multipart.Form, name string) string {
	if form == nil || len(form.Value[name]) == 0 {
		return ""
	}

	return strings.TrimSpace(form.Value[name][0])
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

func writeRawJSON(w http.ResponseWriter, statusCode int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(body); err != nil {
		log.Printf("failed to write helper response: %v", err)
	}
}

func addJSONField(body []byte, name string, value string) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}

	payload[name] = value
	updated, err := json.Marshal(payload)
	if err != nil {
		return body
	}

	return updated
}

func helperError(w http.ResponseWriter, statusCode int, message string) {
	helperJSON(w, statusCode, map[string]any{
		"success": false,
		"message": message,
	})
}

func forwardUpstreamError(w http.ResponseWriter, statusCode int, body []byte, fallback string) {
	message := fallback

	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		message = strings.TrimSpace(payload.Message)
	} else if text := strings.TrimSpace(string(body)); text != "" && len(text) < 240 {
		message = text
	}

	helperError(w, statusCode, message)
}
