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

var nowUTC = func() time.Time {
	return time.Now().UTC()
}

func buildLoginCredential(userDID string, timestamp string) string {
	return "LOGIN|" + userDID + "|" + timestamp
}

func newLoginRequest(userDID string, privateKey *ecdsa.PrivateKey, now time.Time) (LoginRequest, error) {
	userDID = strings.TrimSpace(userDID)
	if userDID == "" {
		return LoginRequest{}, fmt.Errorf("userDID is required")
	}
	if privateKey == nil {
		return LoginRequest{}, fmt.Errorf("private key is required")
	}

	timestamp := now.UTC().Format(time.RFC3339)
	loginCredential := buildLoginCredential(userDID, timestamp)
	digest := sha256.Sum256([]byte(loginCredential))

	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return LoginRequest{}, fmt.Errorf("failed to sign login credential: %w", err)
	}

	return LoginRequest{
		UserDID:   userDID,
		Timestamp: timestamp,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}, nil
}
