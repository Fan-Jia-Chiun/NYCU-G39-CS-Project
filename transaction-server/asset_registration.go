package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxPhotoSize        = 10 << 20
	maxAssetRequestSize = maxPhotoSize + (1 << 20)
)

var lowercaseSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type AssetRegistrationResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	AssetID       string `json:"assetID"`
	AssetAddr     string `json:"assetAddr,omitempty"`
	PhotoCID      string `json:"photoCID"`
	AssetInfoAddr string `json:"assetInfoAddr"`
}

type AssetInfo struct {
	AssetName        string   `json:"assetName"`
	AssetLocation    string   `json:"assetLocation"`
	RegistrationTime TimeInfo `json:"registrationTime"`
	PhotoURL         string   `json:"photoUrl"`
	Description      string   `json:"description"`
}

type TimeInfo struct {
	Year   uint `json:"year"`
	Month  uint `json:"month"`
	Day    uint `json:"day"`
	Hour   uint `json:"hour"`
	Minute uint `json:"minute"`
	Second uint `json:"second"`
}

type ipfsAdder interface {
	Add(ctx context.Context, fileName string, data []byte) (string, error)
}

func assetRegistrationHandler(fabricGateway *FabricGateway, ipfs ipfsAdder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeLoginError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		defer r.Body.Close()

		r.Body = http.MaxBytesReader(w, r.Body, maxAssetRequestSize)
		if err := r.ParseMultipartForm(maxAssetRequestSize); err != nil {
			writeLoginError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		req := readAssetRegistrationForm(r.MultipartForm)
		if err := validateAssetRegistrationFields(req); err != nil {
			writeLoginError(w, http.StatusBadRequest, err.Error())
			return
		}

		if _, err := loginSessions.Validate(req.SessionToken, req.IdentityDID, nowUTC()); err != nil {
			switch err {
			case errSessionExpired:
				writeLoginError(w, http.StatusUnauthorized, "session expired; please log in again")
			case errSessionMismatch:
				writeLoginError(w, http.StatusForbidden, "session does not match identityDID")
			default:
				writeLoginError(w, http.StatusUnauthorized, "session is invalid; please log in again")
			}
			return
		}

		photoFile, photoHeader, err := r.FormFile("photo")
		if err != nil {
			writeLoginError(w, http.StatusBadRequest, "photo file is required")
			return
		}
		defer photoFile.Close()

		photoBytes, err := readPhotoBytes(photoFile)
		if err != nil {
			writeLoginError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validatePhotoBytes(photoBytes); err != nil {
			writeLoginError(w, http.StatusBadRequest, err.Error())
			return
		}

		calculatedPhotoHash := sha256Hex(photoBytes)
		if calculatedPhotoHash != req.PhotoHash {
			writeLoginError(w, http.StatusBadRequest, "photoHash does not match uploaded photo")
			return
		}

		credential := buildRegisterAssetCredential(
			req.IdentityDID,
			req.AssetName,
			req.AssetLocation,
			req.Description,
			req.PhotoHash,
			req.Timestamp,
		)

		publicKeyText, err := evaluatePublicKey(fabricGateway, req.IdentityDID)
		if err != nil {
			log.Printf("failed to get public key for DID %s: %v", req.IdentityDID, err)
			if isPublicKeyNotFoundError(err) {
				writeLoginError(w, http.StatusNotFound, "user public key not found")
				return
			}
			writeLoginError(w, http.StatusBadGateway, "failed to query public key")
			return
		}

		publicKey, err := parseECDSAPublicKey(publicKeyText)
		if err != nil {
			log.Printf("stored public key for DID %s is invalid: %v", req.IdentityDID, err)
			writeLoginError(w, http.StatusInternalServerError, "stored public key is invalid")
			return
		}
		if err := verifyCredentialSignature(publicKey, credential, req.Signature); err != nil {
			if isSignatureFormatError(err) {
				writeLoginError(w, http.StatusBadRequest, "signature is invalid base64")
				return
			}
			writeLoginError(w, http.StatusUnauthorized, "signature verification failed")
			return
		}
		if err := validateLoginTimestamp(req.Timestamp, nowUTC(), defaultLoginTimestampSkew); err != nil {
			switch err {
			case errInvalidTimestamp:
				writeLoginError(w, http.StatusBadRequest, "timestamp is invalid")
			case errExpiredTimestamp, errFutureTimestamp:
				writeLoginError(w, http.StatusUnauthorized, "timestamp is outside the allowed window")
			default:
				writeLoginError(w, http.StatusBadRequest, "timestamp is invalid")
			}
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()

		photoCID, err := ipfs.Add(ctx, safeUploadFileName(photoHeader.Filename, "photo.bin"), photoBytes)
		if err != nil {
			log.Printf("failed to upload photo to IPFS: %v", err)
			writeLoginError(w, http.StatusBadGateway, "failed to upload photo")
			return
		}

		assetInfo := AssetInfo{
			AssetName:        req.AssetName,
			AssetLocation:    req.AssetLocation,
			RegistrationTime: timeInfoFromTime(nowUTC()),
			PhotoURL:         "ipfs://" + photoCID,
			Description:      req.Description,
		}
		assetInfoBytes, err := json.Marshal(assetInfo)
		if err != nil {
			log.Printf("failed to encode asset info: %v", err)
			writeLoginError(w, http.StatusInternalServerError, "failed to build asset info")
			return
		}

		assetInfoAddr, err := ipfs.Add(ctx, "asset-info.json", assetInfoBytes)
		if err != nil {
			log.Printf("failed to upload asset info to IPFS: %v", err)
			writeLoginError(w, http.StatusBadGateway, "failed to upload asset info")
			return
		}
		cacheAssetInfo(assetInfoAddr, assetInfo)

		assetID, assetAddr, err := registerAssetOnChain(fabricGateway, assetInfoAddr, req.IdentityDID)
		if err != nil {
			log.Printf("failed to register asset on chain after IPFS upload photo=%s assetInfo=%s: %v", photoCID, assetInfoAddr, err)
			writeLoginError(w, http.StatusBadGateway, "failed to register asset on chain")
			return
		}

		writeJSON(w, http.StatusOK, AssetRegistrationResponse{
			Success:       true,
			Message:       "asset registered",
			AssetID:       assetID,
			AssetAddr:     assetAddr,
			PhotoCID:      photoCID,
			AssetInfoAddr: assetInfoAddr,
		})
	}
}

type assetRegistrationForm struct {
	SessionToken  string
	IdentityDID   string
	AssetName     string
	AssetLocation string
	Description   string
	Timestamp     string
	PhotoHash     string
	Signature     string
}

func readAssetRegistrationForm(form *multipart.Form) assetRegistrationForm {
	return assetRegistrationForm{
		SessionToken:  firstFormValue(form, "sessionToken"),
		IdentityDID:   firstFormValue(form, "identityDID"),
		AssetName:     firstFormValue(form, "assetName"),
		AssetLocation: firstFormValue(form, "assetLocation"),
		Description:   firstFormValue(form, "description"),
		Timestamp:     firstFormValue(form, "timestamp"),
		PhotoHash:     firstFormValue(form, "photoHash"),
		Signature:     firstFormValue(form, "signature"),
	}
}

func firstFormValue(form *multipart.Form, name string) string {
	if form == nil || len(form.Value[name]) == 0 {
		return ""
	}

	return strings.TrimSpace(form.Value[name][0])
}

func validateAssetRegistrationFields(req assetRegistrationForm) error {
	required := map[string]string{
		"sessionToken":  req.SessionToken,
		"identityDID":   req.IdentityDID,
		"assetName":     req.AssetName,
		"assetLocation": req.AssetLocation,
		"timestamp":     req.Timestamp,
		"photoHash":     req.PhotoHash,
		"signature":     req.Signature,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}

	credentialFields := map[string]string{
		"identityDID":   req.IdentityDID,
		"assetName":     req.AssetName,
		"assetLocation": req.AssetLocation,
		"description":   req.Description,
		"photoHash":     req.PhotoHash,
		"timestamp":     req.Timestamp,
	}
	for name, value := range credentialFields {
		if err := validateCredentialField(name, value); err != nil {
			return err
		}
	}
	if !lowercaseSHA256Pattern.MatchString(req.PhotoHash) {
		return fmt.Errorf("photoHash must be lowercase SHA-256 hex")
	}

	return nil
}

func validateCredentialField(name string, value string) error {
	if strings.Contains(value, "|") {
		return fmt.Errorf("%s cannot contain '|'", name)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains an invalid null character", name)
	}
	for _, r := range value {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return fmt.Errorf("%s contains an invalid control character", name)
		}
	}

	return nil
}

func buildRegisterAssetCredential(identityDID string, assetName string, assetLocation string, description string, photoHash string, timestamp string) string {
	return "REGISTER_ASSET|" + identityDID + "|" + assetName + "|" + assetLocation + "|" + description + "|" + photoHash + "|" + timestamp
}

func readPhotoBytes(file multipart.File) ([]byte, error) {
	photoBytes, err := io.ReadAll(io.LimitReader(file, maxPhotoSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read photo file")
	}
	if len(photoBytes) == 0 {
		return nil, fmt.Errorf("photo file is empty")
	}
	if len(photoBytes) > maxPhotoSize {
		return nil, fmt.Errorf("photo file is too large")
	}

	return photoBytes, nil
}

func validatePhotoBytes(photoBytes []byte) error {
	contentType := http.DetectContentType(photoBytes)
	if contentType == "image/jpeg" || contentType == "image/png" || isWebP(photoBytes) {
		return nil
	}

	return fmt.Errorf("photo must be JPEG, PNG, or WebP")
}

func isWebP(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WEBP"
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:])
}

func safeUploadFileName(name string, fallback string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "/" || name == "" {
		return fallback
	}

	return name
}

func timeInfoFromTime(t time.Time) TimeInfo {
	t = t.UTC()

	return TimeInfo{
		Year:   uint(t.Year()),
		Month:  uint(t.Month()),
		Day:    uint(t.Day()),
		Hour:   uint(t.Hour()),
		Minute: uint(t.Minute()),
		Second: uint(t.Second()),
	}
}

func evaluatePublicKey(fabricGateway *FabricGateway, identityDID string) (string, error) {
	result, err := fabricGateway.Contract.EvaluateTransaction("GetPublicKey", identityDID)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func registerAssetOnChain(fabricGateway *FabricGateway, assetInfoAddr string, identityDID string) (string, string, error) {
	result, err := fabricGateway.Contract.SubmitTransaction("RegisterAsset", assetInfoAddr, identityDID)
	if err != nil {
		return "", "", err
	}

	assetID := strings.TrimSpace(string(result))
	if assetID == "" {
		return "", "", fmt.Errorf("RegisterAsset returned empty assetID")
	}

	assetAddrResult, err := fabricGateway.Contract.EvaluateTransaction("GetCertAddr", assetID)
	if err != nil {
		return assetID, "", err
	}

	return assetID, strings.TrimSpace(string(assetAddrResult)), nil
}
