package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TransactionLaunchRequest struct {
	SessionToken    string `json:"sessionToken"`
	UserDID         string `json:"userDID"`
	SellerDID       string `json:"sellerDID"`
	AssetID         string `json:"assetID"`
	TransactionMode uint   `json:"transactionMode"`
	BasicPrice      uint   `json:"basicPrice"`
	FinalizingTime  string `json:"finalizingTime,omitempty"`
	Timestamp       string `json:"timestamp"`
	Signature       string `json:"signature"`
}

func runLaunchTransaction(transactionURL string, keyDir string, req TransactionLaunchRequest) error {
	privateKey, err := readPrivateKey(privateKeyPath(keyDir))
	if err != nil {
		return fmt.Errorf("failed to read local private key: %w", err)
	}

	req, err = newTransactionLaunchRequest(req, privateKey, nowUTC())
	if err != nil {
		return err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := http.Post(transactionURL, "application/json", bytes.NewReader(body))
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

func newTransactionLaunchRequest(
	req TransactionLaunchRequest,
	privateKey *ecdsa.PrivateKey,
	now time.Time,
) (TransactionLaunchRequest, error) {
	req.SessionToken = strings.TrimSpace(req.SessionToken)
	req.UserDID = strings.TrimSpace(req.UserDID)
	req.SellerDID = strings.TrimSpace(req.SellerDID)
	req.AssetID = strings.TrimSpace(req.AssetID)
	req.FinalizingTime = strings.TrimSpace(req.FinalizingTime)

	if req.SessionToken == "" {
		return TransactionLaunchRequest{}, fmt.Errorf("sessionToken is required")
	}
	if req.UserDID == "" {
		return TransactionLaunchRequest{}, fmt.Errorf("userDID is required")
	}
	if req.SellerDID == "" {
		return TransactionLaunchRequest{}, fmt.Errorf("sellerDID is required")
	}
	if req.AssetID == "" {
		return TransactionLaunchRequest{}, fmt.Errorf("assetID is required")
	}
	if privateKey == nil {
		return TransactionLaunchRequest{}, fmt.Errorf("private key is required")
	}

	for name, value := range map[string]string{
		"sellerDID":      req.SellerDID,
		"assetID":        req.AssetID,
		"finalizingTime": req.FinalizingTime,
	} {
		if err := validateTransactionLaunchCredentialField(name, value); err != nil {
			return TransactionLaunchRequest{}, err
		}
	}

	req.Timestamp = now.UTC().Format(time.RFC3339)
	credential := buildTransactionLaunchCredential(
		req.SellerDID,
		req.AssetID,
		req.TransactionMode,
		req.BasicPrice,
		req.FinalizingTime,
		req.Timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return TransactionLaunchRequest{}, fmt.Errorf("failed to sign transaction launch credential: %w", err)
	}

	req.Signature = base64.StdEncoding.EncodeToString(signature)

	return req, nil
}

// Canonical Message: Property Transaction Launch
// format: TRANSACTION_LAUNCH|sellerDID|assetID|
//         transactionMode|basicPrice|finalizingTime|timestamp
func buildTransactionLaunchCredential(
	sellerDID string,
	assetID string,
	transactionMode uint,
	basicPrice uint,
	finalizingTime string,
	timestamp string,
) string {
	return "TRANSACTION_LAUNCH|" +
		sellerDID + "|" +
		assetID + "|" +
		strconv.FormatUint(uint64(transactionMode), 10) + "|" +
		strconv.FormatUint(uint64(basicPrice), 10) + "|" +
		finalizingTime + "|" +
		timestamp
}

func validateTransactionLaunchCredentialField(name string, value string) error {
	if strings.Contains(value, "|") {
		return fmt.Errorf("%s cannot contain '|'", name)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains an invalid null character", name)
	}

	return nil
}
