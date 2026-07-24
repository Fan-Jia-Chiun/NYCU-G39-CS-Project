package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type TransactionLaunchRequest struct {
	SessionToken    string `json:"sessionToken"`
	UserDID         string `json:"userDID"`
	SellerDID       string `json:"sellerDID"`
	AssetID         string `json:"assetID"`
	TransactionMode uint   `json:"transactionMode"`
	BasicPrice      uint   `json:"basicPrice"`
	FinalizingTime  string `json:"finalizingTime,omitempty"`
}

func runLaunchTransaction(transactionURL string, req TransactionLaunchRequest) error {
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
