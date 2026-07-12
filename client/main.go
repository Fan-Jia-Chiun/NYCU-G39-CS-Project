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

func main() {
	registerURL := flag.String("url", "http://localhost:8081/register", "authority server register endpoint")
	userName := flag.String("userName", "", "user name")
	idCardNumber := flag.String("idCardNumber", "", "ID card number")
	email := flag.String("email", "", "email")
	phone := flag.String("phone", "", "phone")
	keyDir := flag.String("keyDir", defaultKeyDir(), "directory for the local identity key pair")
	flag.Parse()

	resolvedPublicKey, err := ensureIdentityKeyPair(*keyDir)
	if err != nil {
		log.Printf("failed to prepare public key: %v", err)
		os.Exit(1)
	}

	req := RegisterRequest{
		UserName:     *userName,
		IDCardNumber: *idCardNumber,
		Email:        *email,
		Phone:        *phone,
		PublicKey:    resolvedPublicKey,
	}

	if err := callRegister(*registerURL, req); err != nil {
		log.Printf("register request failed: %v", err)
		os.Exit(1)
	}
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
