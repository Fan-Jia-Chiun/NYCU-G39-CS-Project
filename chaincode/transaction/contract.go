package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type TransactionIdentityRegistryContract struct {
	contractapi.Contract
}

// RegisterTradingIdentity maps an identity DID to buyer and seller DIDs.
// Input: identityDID. Output: buyerDID and sellerDID.
func (c *TransactionIdentityRegistryContract) RegisterTradingIdentity(ctx contractapi.TransactionContextInterface, identityDID string) (*TradingIdentityResult, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return nil, err
	}
	if err := validateRequired("identityDID", identityDID); err != nil {
		return nil, err
	}

	identityDID = strings.TrimSpace(identityDID)
	exists, err := tradingIdentityExists(ctx, identityDID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("trading identity already exists for identity DID: %s", identityDID)
	}

	txID := ctx.GetStub().GetTxID()
	now, err := txTimestamp(ctx)
	if err != nil {
		return nil, err
	}

	record := TradingIdentity{
		ObjectType:  objectTypeTradingIdentity,
		IdentityDID: identityDID,
		BuyerDID:    "did:nycu-g39:buyer:" + txID,
		SellerDID:   "did:nycu-g39:seller:" + txID,
		Status:      tradingIdentityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := putTradingIdentity(ctx, &record); err != nil {
		return nil, err
	}
	if err := putIndex(ctx, buyerDIDKey(record.BuyerDID), identityDID); err != nil {
		return nil, err
	}
	if err := putIndex(ctx, sellerDIDKey(record.SellerDID), identityDID); err != nil {
		return nil, err
	}

	return &TradingIdentityResult{
		BuyerDID:  record.BuyerDID,
		SellerDID: record.SellerDID,
	}, nil
}

// SetPublicKey registers the user's public key on the transaction chain.
// Input: identityDID, public key. Output: success.
func (c *TransactionIdentityRegistryContract) SetPublicKey(ctx contractapi.TransactionContextInterface, identityDID string, publicKey string) (bool, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return false, err
	}
	if err := validateRequired("publicKey", publicKey); err != nil {
		return false, err
	}

	record, err := getTradingIdentity(ctx, identityDID)
	if err != nil {
		return false, err
	}

	now, err := txTimestamp(ctx)
	if err != nil {
		return false, err
	}

	record.PublicKey = strings.TrimSpace(publicKey)
	record.UpdatedAt = now

	if err := putTradingIdentity(ctx, record); err != nil {
		return false, err
	}

	return true, nil
}

// GetTradingIdentity returns the trading identity mapping for an identity DID.
// Input: identityDID. Output: trading identity record.
func (c *TransactionIdentityRegistryContract) GetTradingIdentity(ctx contractapi.TransactionContextInterface, identityDID string) (*TradingIdentity, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	return getTradingIdentity(ctx, identityDID)
}

// GetByBuyerDID returns the trading identity mapped to a buyer DID.
// Input: buyerDID. Output: trading identity record.
func (c *TransactionIdentityRegistryContract) GetByBuyerDID(ctx contractapi.TransactionContextInterface, buyerDID string) (*TradingIdentity, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}
	if err := validateRequired("buyerDID", buyerDID); err != nil {
		return nil, err
	}

	identityDID, err := getIndex(ctx, buyerDIDKey(strings.TrimSpace(buyerDID)))
	if err != nil {
		return nil, err
	}

	return getTradingIdentity(ctx, identityDID)
}

// GetBySellerDID returns the trading identity mapped to a seller DID.
// Input: sellerDID. Output: trading identity record.
func (c *TransactionIdentityRegistryContract) GetBySellerDID(ctx contractapi.TransactionContextInterface, sellerDID string) (*TradingIdentity, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}
	if err := validateRequired("sellerDID", sellerDID); err != nil {
		return nil, err
	}

	identityDID, err := getIndex(ctx, sellerDIDKey(strings.TrimSpace(sellerDID)))
	if err != nil {
		return nil, err
	}

	return getTradingIdentity(ctx, identityDID)
}

// GetPublicKey returns the public key stored in the trading identity registry.
// Input: identityDID. Output: public key.
func (c *TransactionIdentityRegistryContract) GetPublicKey(ctx contractapi.TransactionContextInterface, identityDID string) (string, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return "", err
	}

	record, err := getTradingIdentity(ctx, identityDID)
	if err != nil {
		return "", err
	}
	if record.PublicKey == "" {
		return "", fmt.Errorf("public key is not set for identity DID: %s", identityDID)
	}

	return record.PublicKey, nil
}

// tradingIdentityKey builds the ledger key for an identity DID mapping.
func tradingIdentityKey(identityDID string) string {
	return "TRADING_IDENTITY:" + identityDID
}

// buyerDIDKey builds the reverse index key for a buyer DID.
func buyerDIDKey(buyerDID string) string {
	return "BUYER_DID:" + buyerDID
}

// sellerDIDKey builds the reverse index key for a seller DID.
func sellerDIDKey(sellerDID string) string {
	return "SELLER_DID:" + sellerDID
}

// tradingIdentityExists checks whether an identity DID already has a mapping.
func tradingIdentityExists(ctx contractapi.TransactionContextInterface, identityDID string) (bool, error) {
	data, err := ctx.GetStub().GetState(tradingIdentityKey(identityDID))
	if err != nil {
		return false, fmt.Errorf("failed to check trading identity state: %w", err)
	}

	return data != nil, nil
}

// getTradingIdentity reads and decodes a trading identity record by identity DID.
func getTradingIdentity(ctx contractapi.TransactionContextInterface, identityDID string) (*TradingIdentity, error) {
	if err := validateRequired("identityDID", identityDID); err != nil {
		return nil, err
	}

	identityDID = strings.TrimSpace(identityDID)
	data, err := ctx.GetStub().GetState(tradingIdentityKey(identityDID))
	if err != nil {
		return nil, fmt.Errorf("failed to read trading identity state: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("trading identity not found for identity DID: %s", identityDID)
	}

	var record TradingIdentity
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to decode trading identity state: %w", err)
	}

	return &record, nil
}

// putTradingIdentity encodes and writes a trading identity record to world state.
func putTradingIdentity(ctx contractapi.TransactionContextInterface, record *TradingIdentity) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to encode trading identity state: %w", err)
	}

	if err := ctx.GetStub().PutState(tradingIdentityKey(record.IdentityDID), data); err != nil {
		return fmt.Errorf("failed to write trading identity state: %w", err)
	}

	return nil
}

// putIndex writes a reverse DID index entry to world state.
func putIndex(ctx contractapi.TransactionContextInterface, key string, identityDID string) error {
	if err := ctx.GetStub().PutState(key, []byte(identityDID)); err != nil {
		return fmt.Errorf("failed to write DID index %q: %w", key, err)
	}

	return nil
}

// getIndex reads a reverse DID index entry from world state.
func getIndex(ctx contractapi.TransactionContextInterface, key string) (string, error) {
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return "", fmt.Errorf("failed to read DID index %q: %w", key, err)
	}
	if data == nil {
		return "", fmt.Errorf("DID index not found: %s", key)
	}

	return string(data), nil
}

// requireAnyRole allows the call only when the client has one accepted role.
func requireAnyRole(ctx contractapi.TransactionContextInterface, roles ...string) error {
	value, found, err := ctx.GetClientIdentity().GetAttributeValue("role")
	if err != nil {
		return fmt.Errorf("failed to read client role attribute: %w", err)
	}
	if !found {
		return fmt.Errorf("client role attribute is required")
	}

	for _, role := range roles {
		if value == role {
			return nil
		}
	}

	return fmt.Errorf("client role %q is not authorized", value)
}

// validateRequired rejects blank values and null characters.
func validateRequired(name string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return fmt.Errorf("%s contains an invalid null character", name)
	}
	return nil
}

// txTimestamp returns the Fabric transaction timestamp as UTC RFC3339 text.
func txTimestamp(ctx contractapi.TransactionContextInterface) (string, error) {
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", fmt.Errorf("failed to read transaction timestamp: %w", err)
	}

	return time.Unix(ts.Seconds, int64(ts.Nanos)).UTC().Format(time.RFC3339Nano), nil
}
