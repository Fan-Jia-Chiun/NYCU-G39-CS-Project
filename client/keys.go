package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	privateKeyFileName = "identity_private.pem"
	publicKeyFileName  = "identity_public.pem"
)

func defaultKeyDir() string {
	workingDir, err := os.Getwd()
	if err == nil && filepath.Base(workingDir) == "client" {
		return "keys"
	}

	if info, err := os.Stat("client"); err == nil && info.IsDir() {
		return filepath.Join("client", "keys")
	}

	return "keys"
}

func ensureIdentityKeyPair(keyDir string) (string, error) {
	keyDir = strings.TrimSpace(keyDir)
	if keyDir == "" {
		return "", fmt.Errorf("key directory is required")
	}

	privateKeyPath := filepath.Join(keyDir, privateKeyFileName)
	publicKeyPath := filepath.Join(keyDir, publicKeyFileName)

	privateKey, err := readPrivateKey(privateKeyPath)

	// Reuse the existing private key and derive its public key.
	if err == nil {
		publicKeyPEM, err := encodePublicKeyPEM(&privateKey.PublicKey)
		if err != nil {
			return "", err
		}
		publicKeyText, err := encodePublicKeyText(&privateKey.PublicKey)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(keyDir, 0700); err != nil {
			return "", fmt.Errorf("failed to create key directory: %w", err)
		}
		if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
			return "", fmt.Errorf("failed to write public key: %w", err)
		}
		return publicKeyText, nil
	}

	// Stop on read or parse errors; only a missing private key should generate a new pair.
	if !os.IsNotExist(err) {
		return "", err
	}
	
	if _, err := os.Stat(publicKeyPath); err == nil {
		return "", fmt.Errorf("public key exists but private key is missing: %s", privateKeyPath)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check public key: %w", err)
	}

	// No private key exists, so generate a new identity key pair.
	privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyPEM, err := encodePrivateKeyPEM(privateKey)
	if err != nil {
		return "", err
	}
	publicKeyPEM, err := encodePublicKeyPEM(&privateKey.PublicKey)
	if err != nil {
		return "", err
	}
	publicKeyText, err := encodePublicKeyText(&privateKey.PublicKey)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create key directory: %w", err)
	}
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		return "", fmt.Errorf("failed to write private key: %w", err)
	}
	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
		return "", fmt.Errorf("failed to write public key: %w", err)
	}

	return publicKeyText, nil
}

func readPrivateKey(privateKeyPath string) (*ecdsa.PrivateKey, error) {
	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key PEM: %s", privateKeyPath)
	}

	if block.Type == "EC PRIVATE KEY" {
		privateKey, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse EC private key: %w", err)
		}
		return privateKey, nil
	}

	if block.Type == "PRIVATE KEY" {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		privateKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not ECDSA: %s", privateKeyPath)
		}
		return privateKey, nil
	}

	return nil, fmt.Errorf("unsupported private key type %q", block.Type)
}

func encodePrivateKeyPEM(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	privateKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyDER,
	}), nil
}

func encodePublicKeyPEM(publicKey *ecdsa.PublicKey) ([]byte, error) {
	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	}), nil
}

func encodePublicKeyText(publicKey *ecdsa.PublicKey) (string, error) {
	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to encode public key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(publicKeyDER), nil
}
