package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultLoginTimestampSkew = 60 * time.Second

var (
	errInvalidTimestamp = errors.New("timestamp must be UTC RFC3339")
	errExpiredTimestamp = errors.New("timestamp is expired")
	errFutureTimestamp  = errors.New("timestamp is too far in the future")

	nowUTC = func() time.Time {
		return time.Now().UTC()
	}
)

func buildLoginCredential(userDID string, timestamp string) string {
	return "LOGIN|" + userDID + "|" + timestamp
}

func validateLoginTimestamp(timestamp string, now time.Time, allowedSkew time.Duration) error {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil || parsed.UTC().Format(time.RFC3339) != timestamp {
		return errInvalidTimestamp
	}

	now = now.UTC()
	if parsed.Before(now.Add(-allowedSkew)) {
		return errExpiredTimestamp
	}
	if parsed.After(now.Add(allowedSkew)) {
		return errFutureTimestamp
	}

	return nil
}

func parseECDSAPublicKey(publicKeyText string) (*ecdsa.PublicKey, error) {
	publicKeyText = strings.TrimSpace(publicKeyText)
	if publicKeyText == "" {
		return nil, fmt.Errorf("public key is empty")
	}

	publicKeyDER, err := base64.StdEncoding.DecodeString(publicKeyText)
	if err != nil {
		return nil, fmt.Errorf("public key is not valid base64")
	}

	key, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return nil, fmt.Errorf("public key is not valid PKIX DER")
	}

	publicKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not ECDSA")
	}
	if publicKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("public key is not P-256")
	}

	return publicKey, nil
}

func verifyLoginSignature(publicKey *ecdsa.PublicKey, userDID string, timestamp string, signatureText string) error {
	loginCredential := buildLoginCredential(userDID, timestamp)

	return verifyCredentialSignature(publicKey, loginCredential, signatureText)
}

func verifyCredentialSignature(publicKey *ecdsa.PublicKey, credential string, signatureText string) error {
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureText))
	if err != nil {
		return fmt.Errorf("signature is not valid base64")
	}

	digest := sha256.Sum256([]byte(credential))
	if !ecdsa.VerifyASN1(publicKey, digest[:], signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

func isSignatureFormatError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "signature is not valid base64")
}

func isPublicKeyNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "trading identity not found") ||
		strings.Contains(message, "public key is not set") ||
		strings.Contains(message, "not found")
}
