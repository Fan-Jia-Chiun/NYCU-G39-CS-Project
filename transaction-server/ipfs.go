package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultIPFSEndpoint = "http://127.0.0.1:5001"
const defaultIPFSGatewayEndpoint = "http://127.0.0.1:8080"

type ipfsClient struct {
	endpoint string
	client   http.Client
}

func newIPFSClient(endpoint string) *ipfsClient {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		endpoint = defaultIPFSEndpoint
	}

	return &ipfsClient{
		endpoint: endpoint,
		client: http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *ipfsClient) Add(ctx context.Context, fileName string, data []byte) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", fmt.Errorf("failed to create IPFS multipart file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("failed to write IPFS multipart file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close IPFS multipart body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/v0/add", &body)
	if err != nil {
		return "", fmt.Errorf("failed to build IPFS add request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call IPFS add: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IPFS response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("IPFS add returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Hash string `json:"Hash"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to decode IPFS add response: %w", err)
	}
	if strings.TrimSpace(result.Hash) == "" {
		return "", fmt.Errorf("IPFS add returned empty CID")
	}

	return result.Hash, nil
}

func (c *ipfsClient) Cat(ctx context.Context, cid string) ([]byte, error) {
	cid = normalizeIPFSCID(cid)
	if cid == "" {
		return nil, fmt.Errorf("IPFS CID is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/api/v0/cat?arg="+url.QueryEscape(cid), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build IPFS cat request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call IPFS cat: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read IPFS cat response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("IPFS cat returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	return respBody, nil
}

func normalizeIPFSCID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "ipfs://")
	value = strings.TrimPrefix(value, "/ipfs/")

	return value
}

func ipfsGatewayURL(cid string) string {
	cid = normalizeIPFSCID(cid)
	if cid == "" {
		return ""
	}

	gatewayEndpoint := strings.TrimRight(strings.TrimSpace(envOrDefault("IPFS_GATEWAY_ENDPOINT", defaultIPFSGatewayEndpoint)), "/")
	if gatewayEndpoint == "" {
		return ""
	}

	return gatewayEndpoint + "/ipfs/" + cid
}
