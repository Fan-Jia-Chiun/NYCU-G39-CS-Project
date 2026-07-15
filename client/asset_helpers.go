package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AssetRegistrationInput struct {
	SessionToken  string
	IdentityDID   string
	AssetName     string
	AssetLocation string
	Description   string
	PhotoPath     string
}

type AssetRegistrationPayload struct {
	Fields     map[string]string
	PhotoBytes []byte
	FileName   string
}

func newAssetRegistrationPayload(input AssetRegistrationInput, privateKey *ecdsa.PrivateKey, now time.Time) (AssetRegistrationPayload, error) {
	input.PhotoPath = strings.TrimSpace(input.PhotoPath)
	if input.PhotoPath == "" {
		return AssetRegistrationPayload{}, fmt.Errorf("photo path is required")
	}

	photoBytes, err := os.ReadFile(input.PhotoPath)
	if err != nil {
		return AssetRegistrationPayload{}, fmt.Errorf("failed to read photo: %w", err)
	}

	return newAssetRegistrationPayloadFromBytes(input, filepath.Base(input.PhotoPath), photoBytes, privateKey, now)
}

func newAssetRegistrationPayloadFromBytes(input AssetRegistrationInput, fileName string, photoBytes []byte, privateKey *ecdsa.PrivateKey, now time.Time) (AssetRegistrationPayload, error) {
	input.SessionToken = strings.TrimSpace(input.SessionToken)
	input.IdentityDID = strings.TrimSpace(input.IdentityDID)
	input.AssetName = strings.TrimSpace(input.AssetName)
	input.AssetLocation = strings.TrimSpace(input.AssetLocation)
	input.Description = strings.TrimSpace(input.Description)
	fileName = strings.TrimSpace(fileName)

	if input.SessionToken == "" {
		return AssetRegistrationPayload{}, fmt.Errorf("sessionToken is required")
	}
	if input.IdentityDID == "" {
		return AssetRegistrationPayload{}, fmt.Errorf("identityDID is required")
	}
	if input.AssetName == "" {
		return AssetRegistrationPayload{}, fmt.Errorf("assetName is required")
	}
	if input.AssetLocation == "" {
		return AssetRegistrationPayload{}, fmt.Errorf("assetLocation is required")
	}
	if privateKey == nil {
		return AssetRegistrationPayload{}, fmt.Errorf("private key is required")
	}
	if len(photoBytes) == 0 {
		return AssetRegistrationPayload{}, fmt.Errorf("photo file is empty")
	}
	if fileName == "" {
		fileName = "photo.bin"
	}

	credentialFields := map[string]string{
		"identityDID":   input.IdentityDID,
		"assetName":     input.AssetName,
		"assetLocation": input.AssetLocation,
		"description":   input.Description,
	}
	for name, value := range credentialFields {
		if err := validateAssetCredentialField(name, value); err != nil {
			return AssetRegistrationPayload{}, err
		}
	}

	photoHash := assetSHA256Hex(photoBytes)
	timestamp := now.UTC().Format(time.RFC3339)
	credential := buildRegisterAssetCredential(
		input.IdentityDID,
		input.AssetName,
		input.AssetLocation,
		input.Description,
		photoHash,
		timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return AssetRegistrationPayload{}, fmt.Errorf("failed to sign asset registration credential: %w", err)
	}

	return AssetRegistrationPayload{
		Fields: map[string]string{
			"sessionToken":  input.SessionToken,
			"identityDID":   input.IdentityDID,
			"assetName":     input.AssetName,
			"assetLocation": input.AssetLocation,
			"description":   input.Description,
			"timestamp":     timestamp,
			"photoHash":     photoHash,
			"signature":     base64.StdEncoding.EncodeToString(signature),
		},
		PhotoBytes: photoBytes,
		FileName:   fileName,
	}, nil
}

func buildRegisterAssetCredential(identityDID string, assetName string, assetLocation string, description string, photoHash string, timestamp string) string {
	return "REGISTER_ASSET|" + identityDID + "|" + assetName + "|" + assetLocation + "|" + description + "|" + photoHash + "|" + timestamp
}

func validateAssetCredentialField(name string, value string) error {
	if strings.Contains(value, "|") {
		return fmt.Errorf("%s cannot contain '|'", name)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains an invalid null character", name)
	}

	return nil
}

func assetSHA256Hex(data []byte) string {
	hash := sha256.Sum256(data)

	return hex.EncodeToString(hash[:])
}
