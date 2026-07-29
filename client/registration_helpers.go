package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

func newSignedRegisterRequest(
	req RegisterRequest,
	privateKey *ecdsa.PrivateKey,
	now time.Time,
) (RegisterRequest, error) {
	req.UserName = strings.TrimSpace(req.UserName)
	req.IDCardNumber = strings.TrimSpace(req.IDCardNumber)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.PublicKey = strings.TrimSpace(req.PublicKey)

	requiredFields := map[string]string{
		"userName":     req.UserName,
		"idCardNumber": req.IDCardNumber,
		"email":        req.Email,
		"phone":        req.Phone,
		"publicKey":    req.PublicKey,
	}
	for name, value := range requiredFields {
		if value == "" {
			return RegisterRequest{}, fmt.Errorf("%s is required", name)
		}
		if err := validateIdentityRegisterCredentialField(name, value); err != nil {
			return RegisterRequest{}, err
		}
	}
	if privateKey == nil {
		return RegisterRequest{}, fmt.Errorf("private key is required")
	}

	expectedPublicKey, err := encodePublicKeyText(&privateKey.PublicKey)
	if err != nil {
		return RegisterRequest{}, err
	}
	if req.PublicKey != expectedPublicKey {
		return RegisterRequest{}, fmt.Errorf("publicKey does not match the local private key")
	}

	req.Timestamp = now.UTC().Format(time.RFC3339)
	credential := buildIdentityRegisterCredential(
		req.UserName,
		req.IDCardNumber,
		req.Email,
		req.Phone,
		req.PublicKey,
		req.Timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return RegisterRequest{}, fmt.Errorf("failed to sign identity registration credential: %w", err)
	}

	req.Signature = base64.StdEncoding.EncodeToString(signature)

	return req, nil
}

func buildIdentityRegisterCredential(
	userName string,
	idCardNumber string,
	email string,
	phone string,
	publicKey string,
	timestamp string,
) string {
	return "IDENTITY_REGISTER|" +
		userName + "|" +
		idCardNumber + "|" +
		email + "|" +
		phone + "|" +
		publicKey + "|" +
		timestamp
}

func validateIdentityRegisterCredentialField(name string, value string) error {
	if strings.Contains(value, "|") {
		return fmt.Errorf("%s cannot contain '|'", name)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains an invalid null character", name)
	}

	return nil
}
