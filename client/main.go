package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
)

type RegisterRequest struct {
	UserName     string `json:"userName"`
	IDCardNumber string `json:"idCardNumber"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	PublicKey    string `json:"publicKey"`
}

type LoginRequest struct {
	UserDID   string `json:"userDID"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

func main() {
	mode := flag.String("mode", "register", "client mode: register or login")
	registerURL := flag.String("url", "http://localhost:8081/register", "authority server register endpoint")
	loginURL := flag.String("loginURL", "http://localhost:8082/login", "transaction server login endpoint")
	assetURL := flag.String("assetURL", "http://localhost:8082/assets", "transaction server asset registration endpoint")
	userName := flag.String("userName", "", "user name")
	idCardNumber := flag.String("idCardNumber", "", "ID card number")
	email := flag.String("email", "", "email")
	phone := flag.String("phone", "", "phone")
	userDID := flag.String("userDID", "", "user DID for login")
	identityDID := flag.String("identityDID", "", "identity DID alias for requests that use identityDID")
	sessionToken := flag.String("sessionToken", "", "login session token")
	assetName := flag.String("assetName", "", "asset name")
	assetLocation := flag.String("assetLocation", "", "asset location")
	description := flag.String("description", "", "asset description")
	photoPath := flag.String("photo", "", "asset photo path")
	keyDir := flag.String("keyDir", defaultKeyDir(), "directory for the local identity key pair")
	flag.Parse()

	resolvedUserDID := firstNonEmpty(*userDID, *identityDID)
	switch *mode {
	case "register":
		if err := runRegister(*registerURL, *keyDir, RegisterRequest{
			UserName:     *userName,
			IDCardNumber: *idCardNumber,
			Email:        *email,
			Phone:        *phone,
		}); err != nil {
			log.Printf("register request failed: %v", err)
			os.Exit(1)
		}
	case "login":
		if err := runLogin(*loginURL, *keyDir, resolvedUserDID); err != nil {
			log.Printf("login request failed: %v", err)
			os.Exit(1)
		}
	case "register-asset":
		if err := runRegisterAsset(*assetURL, *keyDir, AssetRegistrationInput{
			SessionToken:  *sessionToken,
			IdentityDID:   resolvedUserDID,
			AssetName:     *assetName,
			AssetLocation: *assetLocation,
			Description:   *description,
			PhotoPath:     *photoPath,
		}); err != nil {
			log.Printf("asset registration request failed: %v", err)
			os.Exit(1)
		}
	default:
		log.Printf("unsupported mode: %s", *mode)
		os.Exit(1)
	}
}

func runRegister(registerURL string, keyDir string, req RegisterRequest) error {
	resolvedPublicKey, err := ensureIdentityKeyPair(keyDir)
	if err != nil {
		return fmt.Errorf("failed to prepare public key: %w", err)
	}

	req.PublicKey = resolvedPublicKey

	return callRegister(registerURL, req)
}

func runLogin(loginURL string, keyDir string, userDID string) error {
	privateKey, err := readPrivateKey(privateKeyPath(keyDir))
	if err != nil {
		return fmt.Errorf("failed to read local private key: %w", err)
	}

	req, err := newLoginRequest(userDID, privateKey, nowUTC())
	if err != nil {
		return err
	}

	return callLogin(loginURL, req)
}

func runRegisterAsset(assetURL string, keyDir string, input AssetRegistrationInput) error {
	privateKey, err := readPrivateKey(privateKeyPath(keyDir))
	if err != nil {
		return fmt.Errorf("failed to read local private key: %w", err)
	}

	payload, err := newAssetRegistrationPayload(input, privateKey, nowUTC())
	if err != nil {
		return err
	}

	return callRegisterAsset(assetURL, payload)
}

func callRegister(registerURL string, req RegisterRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := http.Post(registerURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to call authority server: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(respBody))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("authority server returned non-success status: %s", resp.Status)
	}

	return nil
}

func callLogin(loginURL string, req LoginRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := http.Post(loginURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to call transaction server: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(respBody))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("transaction server returned non-success status: %s", resp.Status)
	}

	return nil
}

func callRegisterAsset(assetURL string, payload AssetRegistrationPayload) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range payload.Fields {
		if err := writer.WriteField(name, value); err != nil {
			return fmt.Errorf("failed to write multipart field %s: %w", name, err)
		}
	}

	part, err := writer.CreateFormFile("photo", payload.FileName)
	if err != nil {
		return fmt.Errorf("failed to create multipart photo field: %w", err)
	}
	if _, err := part.Write(payload.PhotoBytes); err != nil {
		return fmt.Errorf("failed to write multipart photo: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart request: %w", err)
	}

	resp, err := http.Post(assetURL, writer.FormDataContentType(), &body)
	if err != nil {
		return fmt.Errorf("failed to call transaction server: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(respBody))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("transaction server returned non-success status: %s", resp.Status)
	}

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
