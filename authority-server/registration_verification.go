package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	identityRegisterTimestampSkew = 60 * time.Second
	userTypeGeneral               = uint(0)
	userTypeLogistics             = uint(1)
)

type registrationVerificationError struct {
	StatusCode    int
	ClientMessage string
	Cause         error
}

func verifyIdentityRegisterRequest(
	req RegisterRequest,
	now time.Time,
) *registrationVerificationError {
	if err := validateIdentityRegisterTimestamp(req.Timestamp, now, identityRegisterTimestampSkew); err != nil {
		statusCode := http.StatusBadRequest
		message := "timestamp is invalid"
		if strings.Contains(err.Error(), "outside") {
			statusCode = http.StatusUnauthorized
			message = "timestamp is outside the allowed window"
		}
		return &registrationVerificationError{
			StatusCode:    statusCode,
			ClientMessage: message,
			Cause:         err,
		}
	}

	publicKey, err := parseIdentityRegisterPublicKey(req.PublicKey)
	if err != nil {
		return &registrationVerificationError{
			StatusCode:    http.StatusBadRequest,
			ClientMessage: "publicKey is invalid",
			Cause:         err,
		}
	}

	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return &registrationVerificationError{
			StatusCode:    http.StatusBadRequest,
			ClientMessage: "signature is invalid base64",
			Cause:         err,
		}
	}

	credential := buildIdentityRegisterCredential(
		req.UserType,
		req.UserName,
		req.LogisticsCompanyName,
		req.IDCardNumber,
		req.Email,
		req.Phone,
		req.PublicKey,
		req.Timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	if !ecdsa.VerifyASN1(publicKey, digest[:], signature) {
		return &registrationVerificationError{
			StatusCode:    http.StatusUnauthorized,
			ClientMessage: "signature verification failed",
		}
	}

	return nil
}

// Canonical Message: Registration & Initialization
// format: IDENTITY_REGISTER|<userType>|<userName>|<logisticsCompanyName>|<idCardNumber>|<email>|<phone>|<publicKey>|<timestamp>
func buildIdentityRegisterCredential(
	userType uint,
	userName string,
	logisticsCompanyName string,
	idCardNumber string,
	email string,
	phone string,
	publicKey string,
	timestamp string,
) string {
	return "IDENTITY_REGISTER|" +
		strconv.FormatUint(uint64(userType), 10) + "|" +
		userName + "|" +
		logisticsCompanyName + "|" +
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

func validateIdentityRegisterTimestamp(
	timestamp string,
	now time.Time,
	allowedSkew time.Duration,
) error {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil || parsed.UTC().Format(time.RFC3339) != timestamp {
		return fmt.Errorf("timestamp must be UTC RFC3339")
	}

	now = now.UTC()
	if parsed.Before(now.Add(-allowedSkew)) || parsed.After(now.Add(allowedSkew)) {
		return fmt.Errorf("timestamp is outside the allowed window")
	}

	return nil
}

func parseIdentityRegisterPublicKey(publicKeyText string) (*ecdsa.PublicKey, error) {
	publicKeyDER, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyText))
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
