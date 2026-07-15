package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type TransactionIdentityRegistryContract struct {
	contractapi.Contract
}

// RegisterTradingIdentity maps an user DID to buyer and seller DIDs.
// Input: userDID. Output: buyerDID and sellerDID.
func (c *TransactionIdentityRegistryContract) RegisterTradingIdentity(ctx contractapi.TransactionContextInterface, userDID string) (*TradingIdentityResult, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return nil, err
	}
	if err := validateRequired("userDID", userDID); err != nil {
		return nil, err
	}

	userDID = strings.TrimSpace(userDID)
	exists, err := tradingIdentityExists(ctx, userDID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("trading identity already exists for user DID: %s", userDID)
	}

	now, err := txTimestamp(ctx)
	if err != nil {
		return nil, err
	}

	buyerDID := deriveTradingDID(userDID, "buyer")
	sellerDID := deriveTradingDID(userDID, "seller")

	record := TradingIdentity{
		ObjectType:        objectTypeTradingIdentity,
		UserDID:           userDID,
		BuyerDID:          buyerDID,
		SellerDID:         sellerDID,
		Status:            tradingIdentityStatusActive,
		AccountStatus:     accountStatusAvailable,
		BuyerCreditScore:  defaultBuyerCreditScore,
		SellerCreditScore: defaultSellerCreditScore,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := putTradingIdentity(ctx, &record); err != nil {
		return nil, err
	}
	if err := putIndex(ctx, buyerDIDKey(record.BuyerDID), userDID); err != nil {
		return nil, err
	}
	if err := putIndex(ctx, sellerDIDKey(record.SellerDID), userDID); err != nil {
		return nil, err
	}

	return &TradingIdentityResult{
		BuyerDID:  record.BuyerDID,
		SellerDID: record.SellerDID,
	}, nil
}

// SetPublicKey registers the user's public key on the transaction chain.
// Input: userDID, public key. Output: success.
func (c *TransactionIdentityRegistryContract) SetPublicKey(ctx contractapi.TransactionContextInterface, userDID string, publicKey string) (bool, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return false, err
	}
	if err := validateRequired("publicKey", publicKey); err != nil {
		return false, err
	}

	record, err := getTradingIdentity(ctx, userDID)
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

// GetTradingIdentity returns the trading identity mapping for an user DID.
// Input: userDID. Output: trading identity record.
func (c *TransactionIdentityRegistryContract) GetTradingIdentity(ctx contractapi.TransactionContextInterface, userDID string) (*TradingIdentity, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	return getTradingIdentity(ctx, userDID)
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

	userDID, err := getIndex(ctx, buyerDIDKey(strings.TrimSpace(buyerDID)))
	if err != nil {
		return nil, err
	}

	return getTradingIdentity(ctx, userDID)
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

	userDID, err := getIndex(ctx, sellerDIDKey(strings.TrimSpace(sellerDID)))
	if err != nil {
		return nil, err
	}

	return getTradingIdentity(ctx, userDID)
}

// GetPublicKey returns the public key stored in the trading identity registry.
// Input: userDID. Output: public key.
func (c *TransactionIdentityRegistryContract) GetPublicKey(ctx contractapi.TransactionContextInterface, userDID string) (string, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return "", err
	}

	record, err := getTradingIdentity(ctx, userDID)
	if err != nil {
		return "", err
	}
	if record.PublicKey == "" {
		return "", fmt.Errorf("public key is not set for user DID: %s", userDID)
	}

	return record.PublicKey, nil
}

// CheckIdentityStatus returns whether the trading identity can log in.
// Input: userDID. Output: 0 available, 1 disabled, 2 deregistered.
func (c *TransactionIdentityRegistryContract) CheckIdentityStatus(ctx contractapi.TransactionContextInterface, userDID string) (uint, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return 0, err
	}

	record, err := getTradingIdentity(ctx, userDID)
	if err != nil {
		return 0, err
	}

	return record.AccountStatus, nil
}

// GetCreditScores returns the user's buyer and seller credit scores.
// Input: userDID. Output: buyer credit score and seller credit score.
func (c *TransactionIdentityRegistryContract) GetCreditScores(ctx contractapi.TransactionContextInterface, userDID string) (*CreditScores, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	record, err := getTradingIdentity(ctx, userDID)
	if err != nil {
		return nil, err
	}

	return &CreditScores{
		BuyerCreditScore:  record.BuyerCreditScore,
		SellerCreditScore: record.SellerCreditScore,
	}, nil
}

// GetAssetList returns the asset certificate addresses owned by a user.
// Input: userDID. Output: asset certificate address list.
func (c *TransactionIdentityRegistryContract) GetAssetList(ctx contractapi.TransactionContextInterface, userDID string) ([]string, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	list, err := getUserAssetList(ctx, userDID)
	if err != nil {
		return nil, err
	}

	return list.AssetAddrs, nil
}

// RegisterAsset creates an asset certificate and indexes it under the owner DID.
// Input: AssetInfoAddr, userDID. Output: assetID.
func (c *TransactionIdentityRegistryContract) RegisterAsset(ctx contractapi.TransactionContextInterface, assetData string, userDID string) (string, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return "", err
	}
	if err := validateRequired("assetData", assetData); err != nil {
		return "", err
	}
	if err := validateRequired("userDID", userDID); err != nil {
		return "", err
	}

	assetData = strings.TrimSpace(assetData)
	userDID = strings.TrimSpace(userDID)
	if _, err := getTradingIdentity(ctx, userDID); err != nil {
		return "", err
	}

	txID := ctx.GetStub().GetTxID()
	if txID == "" {
		return "", fmt.Errorf("transaction ID is required to register asset")
	}
	assetID := deriveAssetID(txID)
	assetAddr := assetCertificateKey(assetID)

	existing, err := ctx.GetStub().GetState(assetAddr)
	if err != nil {
		return "", fmt.Errorf("failed to check asset certificate state: %w", err)
	}
	if existing != nil {
		return "", fmt.Errorf("asset already exists: %s", assetID)
	}

	now, err := txTimestamp(ctx)
	if err != nil {
		return "", err
	}

	cert := AssetCertificate{
		ObjectType:    objectTypeAssetCert,
		AssetID:       assetID,
		AssetInfoAddr: assetData,
		LegalStatus:   legalStatusNormal,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := putAssetCertificate(ctx, assetAddr, &cert); err != nil {
		return "", err
	}
	if err := putIndex(ctx, assetCertAddrKey(assetID), assetAddr); err != nil {
		return "", err
	}

	property := PropertyIndex{
		ObjectType: objectTypePropertyIndex,
		AssetID:    assetID,
		OwnerDID:   userDID,
		ChangeLog:  []string{"REGISTER|" + now + "|" + userDID},
	}
	if err := putPropertyIndex(ctx, &property); err != nil {
		return "", err
	}

	assetList, err := getUserAssetList(ctx, userDID)
	if err != nil {
		return "", err
	}
	if !containsString(assetList.AssetAddrs, assetAddr) {
		assetList.AssetAddrs = append(assetList.AssetAddrs, assetAddr)
	}
	if err := putUserAssetList(ctx, assetList); err != nil {
		return "", err
	}

	return assetID, nil
}

// GetCertAddr returns the asset certificate address for an asset ID.
// Input: assetID. Output: asset certificate address, or an empty string when missing.
func (c *TransactionIdentityRegistryContract) GetCertAddr(ctx contractapi.TransactionContextInterface, assetID string) (string, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return "", err
	}
	if err := validateRequired("assetID", assetID); err != nil {
		return "", err
	}

	data, err := ctx.GetStub().GetState(assetCertAddrKey(strings.TrimSpace(assetID)))
	if err != nil {
		return "", fmt.Errorf("failed to read asset certificate address: %w", err)
	}
	if data == nil {
		return "", nil
	}

	return string(data), nil
}

// CheckStatus returns the legal status of an asset certificate.
// Input: asset certificate address. Output: legal status code.
func (c *TransactionIdentityRegistryContract) CheckStatus(ctx contractapi.TransactionContextInterface, assetAddr string) (int, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return 0, err
	}

	cert, err := getAssetCertificate(ctx, assetAddr)
	if err != nil {
		return 0, err
	}

	return cert.LegalStatus, nil
}

// GetTradeInfo returns the current state of a trade.
// Input: tradeID. Output: trade info.
func (c *TransactionIdentityRegistryContract) GetTradeInfo(ctx contractapi.TransactionContextInterface, tradeID uint) (*TradeInfo, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	return getTradeInfo(ctx, tradeID)
}

// GetTradeList returns the user's trade IDs, roles, and active flags.
// Input: userDID. Output: tradeIDList, transactionRoleList, isActiveList.
func (c *TransactionIdentityRegistryContract) GetTradeList(ctx contractapi.TransactionContextInterface, userDID string) (*TradeListResult, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	list, err := getUserTransactionList(ctx, userDID)
	if err != nil {
		return nil, err
	}

	return &TradeListResult{
		TradeIDList:         list.TradeIDList,
		TransactionRoleList: list.TransactionRoleList,
		IsActiveList:        list.IsActiveList,
	}, nil
}

// UpdateTradeList inserts a trade record or updates its active status.
// Input: userDID, tradeID, assetID, transactionRole, isActive. Output: success.
func (c *TransactionIdentityRegistryContract) UpdateTradeList(ctx contractapi.TransactionContextInterface, userDID string, tradeID uint, assetID string, transactionRole uint, isActive bool) (bool, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return false, err
	}

	list, err := getUserTransactionList(ctx, userDID)
	if err != nil {
		return false, err
	}

	for i, existingTradeID := range list.TradeIDList {
		if existingTradeID == tradeID {
			list.IsActiveList[i] = isActive
			if err := putUserTransactionList(ctx, list); err != nil {
				return false, err
			}

			return true, nil
		}
	}

	if err := validateRequired("assetID", assetID); err != nil {
		return false, err
	}
	if transactionRole > 2 {
		return false, fmt.Errorf("transactionRole must be 0, 1, or 2")
	}

	list.TradeIDList = append(list.TradeIDList, tradeID)
	list.AssetIDList = append(list.AssetIDList, strings.TrimSpace(assetID))
	list.TransactionRoleList = append(list.TransactionRoleList, transactionRole)
	list.IsActiveList = append(list.IsActiveList, isActive)

	if err := putUserTransactionList(ctx, list); err != nil {
		return false, err
	}

	return true, nil
}

// tradingIdentityKey builds the ledger key for an user DID mapping.
func tradingIdentityKey(userDID string) string {
	return "TRADING_IDENTITY:" + userDID
}

// buyerDIDKey builds the reverse index key for a buyer DID.
func buyerDIDKey(buyerDID string) string {
	return "BUYER_DID:" + buyerDID
}

// sellerDIDKey builds the reverse index key for a seller DID.
func sellerDIDKey(sellerDID string) string {
	return "SELLER_DID:" + sellerDID
}

// userAssetListKey builds the ledger key for a user's asset list.
func userAssetListKey(userDID string) string {
	return "USER_ASSET_LIST:" + userDID
}

// assetCertificateKey builds the ledger key for an asset certificate.
func assetCertificateKey(assetID string) string {
	return "ASSET_CERT:" + assetID
}

// assetCertAddrKey builds the asset ID to asset certificate address index key.
func assetCertAddrKey(assetID string) string {
	return "ASSET_CERT_ADDR:" + assetID
}

// propertyIndexKey builds the ledger key for a property index record.
func propertyIndexKey(assetID string) string {
	return "PROPERTY_INDEX:" + assetID
}

// tradeInfoKey builds the ledger key for a trade info record.
func tradeInfoKey(tradeID uint) string {
	return fmt.Sprintf("TRADE_INFO:%d", tradeID)
}

// userTransactionListKey builds the ledger key for a user's transaction list.
func userTransactionListKey(userDID string) string {
	return "USER_TRANSACTION_LIST:" + userDID
}

// deriveTradingDID deterministically derives a role-specific trading DID.
func deriveTradingDID(userDID string, role string) string {
	seed := "nycu-g39:trading-did:v1:" + role + ":" + userDID
	hash := sha256.Sum256([]byte(seed))

	return "did:nycu-g39:" + role + ":" + hex.EncodeToString(hash[:])
}

// deriveAssetID deterministically derives a unique asset ID from a Fabric txID.
func deriveAssetID(txID string) string {
	seed := "nycu-g39:asset:v1:" + txID
	hash := sha256.Sum256([]byte(seed))

	return "asset:" + hex.EncodeToString(hash[:])
}

// tradingIdentityExists checks whether an user DID already has a mapping.
func tradingIdentityExists(ctx contractapi.TransactionContextInterface, userDID string) (bool, error) {
	data, err := ctx.GetStub().GetState(tradingIdentityKey(userDID))
	if err != nil {
		return false, fmt.Errorf("failed to check trading identity state: %w", err)
	}

	return data != nil, nil
}

// getTradingIdentity reads and decodes a trading identity record by user DID.
func getTradingIdentity(ctx contractapi.TransactionContextInterface, userDID string) (*TradingIdentity, error) {
	if err := validateRequired("userDID", userDID); err != nil {
		return nil, err
	}

	userDID = strings.TrimSpace(userDID)
	data, err := ctx.GetStub().GetState(tradingIdentityKey(userDID))
	if err != nil {
		return nil, fmt.Errorf("failed to read trading identity state: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("trading identity not found for user DID: %s", userDID)
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

	if err := ctx.GetStub().PutState(tradingIdentityKey(record.UserDID), data); err != nil {
		return fmt.Errorf("failed to write trading identity state: %w", err)
	}

	return nil
}

// getUserAssetList reads a user's asset list, returning an empty list when absent.
func getUserAssetList(ctx contractapi.TransactionContextInterface, userDID string) (*UserAssetList, error) {
	if err := validateRequired("userDID", userDID); err != nil {
		return nil, err
	}

	userDID = strings.TrimSpace(userDID)
	data, err := ctx.GetStub().GetState(userAssetListKey(userDID))
	if err != nil {
		return nil, fmt.Errorf("failed to read user asset list: %w", err)
	}
	if data == nil {
		return &UserAssetList{
			ObjectType: objectTypeUserAssetList,
			UserDID:    userDID,
			AssetAddrs: []string{},
		}, nil
	}

	var list UserAssetList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("failed to decode user asset list: %w", err)
	}
	if list.AssetAddrs == nil {
		list.AssetAddrs = []string{}
	}

	return &list, nil
}

// getAssetCertificate reads an asset certificate by its ledger address.
func getAssetCertificate(ctx contractapi.TransactionContextInterface, assetAddr string) (*AssetCertificate, error) {
	if err := validateRequired("assetAddr", assetAddr); err != nil {
		return nil, err
	}

	assetAddr = strings.TrimSpace(assetAddr)
	data, err := ctx.GetStub().GetState(assetAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to read asset certificate: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("asset certificate not found: %s", assetAddr)
	}

	var cert AssetCertificate
	if err := json.Unmarshal(data, &cert); err != nil {
		return nil, fmt.Errorf("failed to decode asset certificate: %w", err)
	}

	return &cert, nil
}

// putAssetCertificate writes an asset certificate to world state.
func putAssetCertificate(ctx contractapi.TransactionContextInterface, assetAddr string, cert *AssetCertificate) error {
	data, err := json.Marshal(cert)
	if err != nil {
		return fmt.Errorf("failed to encode asset certificate: %w", err)
	}

	if err := ctx.GetStub().PutState(assetAddr, data); err != nil {
		return fmt.Errorf("failed to write asset certificate: %w", err)
	}

	return nil
}

// putPropertyIndex writes a property index record to world state.
func putPropertyIndex(ctx contractapi.TransactionContextInterface, property *PropertyIndex) error {
	data, err := json.Marshal(property)
	if err != nil {
		return fmt.Errorf("failed to encode property index: %w", err)
	}

	if err := ctx.GetStub().PutState(propertyIndexKey(property.AssetID), data); err != nil {
		return fmt.Errorf("failed to write property index: %w", err)
	}

	return nil
}

// getTradeInfo reads a trade info record by trade ID.
func getTradeInfo(ctx contractapi.TransactionContextInterface, tradeID uint) (*TradeInfo, error) {
	data, err := ctx.GetStub().GetState(tradeInfoKey(tradeID))
	if err != nil {
		return nil, fmt.Errorf("failed to read trade info: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("trade info not found: %d", tradeID)
	}

	var info TradeInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to decode trade info: %w", err)
	}
	info.TradeID = tradeID

	return &info, nil
}

// getUserTransactionList reads a user's transaction list, returning an empty list when absent.
func getUserTransactionList(ctx contractapi.TransactionContextInterface, userDID string) (*UserTransactionList, error) {
	if err := validateRequired("userDID", userDID); err != nil {
		return nil, err
	}

	userDID = strings.TrimSpace(userDID)
	data, err := ctx.GetStub().GetState(userTransactionListKey(userDID))
	if err != nil {
		return nil, fmt.Errorf("failed to read user transaction list: %w", err)
	}
	if data == nil {
		return &UserTransactionList{
			ObjectType:          objectTypeUserTradeList,
			UserDID:             userDID,
			TradeIDList:         []uint{},
			AssetIDList:         []string{},
			TransactionRoleList: []uint{},
			IsActiveList:        []bool{},
		}, nil
	}

	var list UserTransactionList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("failed to decode user transaction list: %w", err)
	}
	if list.TradeIDList == nil {
		list.TradeIDList = []uint{}
	}
	if list.AssetIDList == nil {
		list.AssetIDList = []string{}
	}
	if list.TransactionRoleList == nil {
		list.TransactionRoleList = []uint{}
	}
	if list.IsActiveList == nil {
		list.IsActiveList = []bool{}
	}
	if len(list.TradeIDList) != len(list.TransactionRoleList) || len(list.TradeIDList) != len(list.IsActiveList) {
		return nil, fmt.Errorf("user transaction list has inconsistent field lengths")
	}

	return &list, nil
}

// putUserTransactionList writes a user's transaction list to world state.
func putUserTransactionList(ctx contractapi.TransactionContextInterface, list *UserTransactionList) error {
	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("failed to encode user transaction list: %w", err)
	}

	if err := ctx.GetStub().PutState(userTransactionListKey(list.UserDID), data); err != nil {
		return fmt.Errorf("failed to write user transaction list: %w", err)
	}

	return nil
}

// putUserAssetList writes a user's asset list to world state.
func putUserAssetList(ctx contractapi.TransactionContextInterface, list *UserAssetList) error {
	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("failed to encode user asset list: %w", err)
	}

	if err := ctx.GetStub().PutState(userAssetListKey(list.UserDID), data); err != nil {
		return fmt.Errorf("failed to write user asset list: %w", err)
	}

	return nil
}

// putIndex writes a reverse DID index entry to world state.
func putIndex(ctx contractapi.TransactionContextInterface, key string, userDID string) error {
	if err := ctx.GetStub().PutState(key, []byte(userDID)); err != nil {
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

// containsString reports whether target already appears in values.
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
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
