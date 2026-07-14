package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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
	userName := flag.String("userName", "", "user name")
	idCardNumber := flag.String("idCardNumber", "", "ID card number")
	email := flag.String("email", "", "email")
	phone := flag.String("phone", "", "phone")
	userDID := flag.String("userDID", "", "user DID for login")
	keyDir := flag.String("keyDir", defaultKeyDir(), "directory for the local identity key pair")
	flag.Parse()

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
		if err := runLogin(*loginURL, *keyDir, *userDID); err != nil {
			log.Printf("login request failed: %v", err)
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
