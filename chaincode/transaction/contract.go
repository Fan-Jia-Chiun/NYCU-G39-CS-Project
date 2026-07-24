package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
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

// GetBySellerDID returns the User DID mapped to a seller DID.
// Input: sellerDID. Output: User DID.
func (c *TransactionIdentityRegistryContract) GetBySellerDID(ctx contractapi.TransactionContextInterface, sellerDID string) (string, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return "", err
	}
	if err := validateRequired("sellerDID", sellerDID); err != nil {
		return "", err
	}

	userDID, err := getIndex(ctx, sellerDIDKey(strings.TrimSpace(sellerDID)))
	if err != nil {
		return "", err
	}

	return userDID, nil
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
// Input: AssetInfoCID, userDID. Output: assetID.
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
		ObjectType:   objectTypeAssetCert,
		AssetID:      assetID,
		AssetInfoCID: assetData,
		LegalStatus:  legalStatusNormal,
		CreatedAt:    now,
		UpdatedAt:    now,
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

// GetAssetCertificate returns an asset certificate by certificate address.
// Input: asset certificate address. Output: asset certificate.
func (c *TransactionIdentityRegistryContract) GetAssetCertificate(ctx contractapi.TransactionContextInterface, assetAddr string) (*AssetCertificate, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	return getAssetCertificate(ctx, assetAddr)
}

// GetOwner returns the current owner User DID for an asset.
// Input: assetID. Output: owner User DID.
func (c *TransactionIdentityRegistryContract) GetOwner(ctx contractapi.TransactionContextInterface, assetID string) (string, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return "", err
	}

	property, err := getPropertyIndex(ctx, assetID)
	if err != nil {
		return "", err
	}

	return property.OwnerDID, nil
}

// UpdateStatus updates the legal status stored in an asset certificate.
// Input: asset certificate address, target status. Output: success.
func (c *TransactionIdentityRegistryContract) UpdateStatus(ctx contractapi.TransactionContextInterface, assetAddr string, targetStatus int) (bool, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return false, err
	}
	if targetStatus < legalStatusNormal || targetStatus > legalStatusRestricted {
		return false, fmt.Errorf("targetStatus must be between %d and %d", legalStatusNormal, legalStatusRestricted)
	}

	cert, err := getAssetCertificate(ctx, assetAddr)
	if err != nil {
		return false, err
	}

	now, err := txTimestamp(ctx)
	if err != nil {
		return false, err
	}

	cert.LegalStatus = targetStatus
	cert.UpdatedAt = now
	if err := putAssetCertificate(ctx, strings.TrimSpace(assetAddr), cert); err != nil {
		return false, err
	}

	return true, nil
}

// AddNewTransaction creates a transaction in Reviewing status.
// Input: assetID, sellerDID. Output: transactionID.
func (c *TransactionIdentityRegistryContract) AddNewTransaction(ctx contractapi.TransactionContextInterface, assetID string, sellerDID string) (uint, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return 0, err
	}
	if err := validateRequired("assetID", assetID); err != nil {
		return 0, err
	}
	if err := validateRequired("sellerDID", sellerDID); err != nil {
		return 0, err
	}

	assetID = strings.TrimSpace(assetID)
	sellerDID = strings.TrimSpace(sellerDID)
	if _, err := getAssetCertificateAddress(ctx, assetID); err != nil {
		return 0, err
	}

	transactionID, err := nextTransactionID(ctx)
	if err != nil {
		return 0, err
	}

	info := TransactionInfo{
		ObjectType:        objectTypeTransactionInfo,
		TransactionID:     transactionID,
		AssetID:           assetID,
		SellerDID:         sellerDID,
		TransactionStatus: transactionStatusReviewing,
	}
	if err := putTransactionInfo(ctx, &info); err != nil {
		return 0, err
	}

	return transactionID, nil
}

// GetTransactionInfo returns the current state of a transaction.
// Input: transactionID. Output: TransactionInfo.
func (c *TransactionIdentityRegistryContract) GetTransactionInfo(ctx contractapi.TransactionContextInterface, transactionID uint) (*TransactionInfo, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	return getTransactionInfo(ctx, transactionID)
}

// ChangeTransactionStatus updates a transaction status except Reviewing and In Progress.
// Input: transactionID, newStatus. Output: success.
func (c *TransactionIdentityRegistryContract) ChangeTransactionStatus(ctx contractapi.TransactionContextInterface, transactionID uint, newStatus uint) (bool, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return false, err
	}
	if newStatus > transactionStatusRejected {
		return false, fmt.Errorf("newStatus must be between %d and %d", transactionStatusReviewing, transactionStatusRejected)
	}
	if newStatus == transactionStatusReviewing || newStatus == transactionStatusInProgress {
		return false, fmt.Errorf("Reviewing and In Progress must be set by AddNewTransaction and StartTransaction")
	}

	info, err := getTransactionInfo(ctx, transactionID)
	if err != nil {
		return false, err
	}
	info.TransactionStatus = newStatus
	if err := putTransactionInfo(ctx, info); err != nil {
		return false, err
	}

	return true, nil
}

// StartTransaction sets mode-specific transaction data and changes the status to In Progress.
// Input: transactionID, price, transactionMode, finalizingTime. Output: success.
func (c *TransactionIdentityRegistryContract) StartTransaction(ctx contractapi.TransactionContextInterface, transactionID uint, basicPrice uint, transactionMode uint, finalizingTime TimeInfo) (bool, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return false, err
	}
	if basicPrice == 0 {
		return false, fmt.Errorf("basicPrice must be greater than zero")
	}
	if transactionMode > transactionModeSealedBid {
		return false, fmt.Errorf("transactionMode must be 0, 1, or 2")
	}

	info, err := getTransactionInfo(ctx, transactionID)
	if err != nil {
		return false, err
	}
	if info.TransactionStatus != transactionStatusReviewing {
		return false, fmt.Errorf("transaction must be in Reviewing status")
	}

	startTime, startAt, err := transactionTimeInfo(ctx)
	if err != nil {
		return false, err
	}

	info.TransactionMode = transactionMode
	info.StartTime = startTime
	info.FixedPrice = 0
	info.BasicPrice = 0
	info.CurrentHighestBid = 0
	info.FinalizingTime = TimeInfo{}

	switch transactionMode {
	case transactionModeFixedPrice:
		info.FixedPrice = basicPrice
	case transactionModeBidding, transactionModeSealedBid:
		finalizesAt, err := validateTransactionTimeInfo(finalizingTime)
		if err != nil {
			return false, fmt.Errorf("invalid finalizingTime: %w", err)
		}
		if !finalizesAt.After(startAt) {
			return false, fmt.Errorf("finalizingTime must be after the transaction start time")
		}
		info.BasicPrice = basicPrice
		info.FinalizingTime = finalizingTime
	}

	info.TransactionStatus = transactionStatusInProgress
	if err := putTransactionInfo(ctx, info); err != nil {
		return false, err
	}

	return true, nil
}

// GetTransactionList returns transaction IDs, asset IDs, roles, and active flags.
// Input: userDID. Output: parallel transaction list fields.
func (c *TransactionIdentityRegistryContract) GetTransactionList(ctx contractapi.TransactionContextInterface, userDID string) (*TradeListResult, error) {
	if err := requireAnyRole(ctx, roleTransactionService, roleVerifier); err != nil {
		return nil, err
	}

	list, err := getUserTransactionList(ctx, userDID)
	if err != nil {
		return nil, err
	}

	return &TradeListResult{
		TransactionIDList:   list.TransactionIDList,
		AssetIDList:         list.AssetIDList,
		TransactionRoleList: list.TransactionRoleList,
		IsActiveList:        list.IsActiveList,
	}, nil
}

// UpdateTransactionList inserts a transaction record or updates its active status.
// Input: userDID, transactionID, assetID, transactionRole, isActive. Output: success.
func (c *TransactionIdentityRegistryContract) UpdateTransactionList(ctx contractapi.TransactionContextInterface, userDID string, transactionID uint, assetID string, transactionRole uint, isActive bool) (bool, error) {
	if err := requireAnyRole(ctx, roleTransactionService); err != nil {
		return false, err
	}

	list, err := getUserTransactionList(ctx, userDID)
	if err != nil {
		return false, err
	}

	for i, existingTransactionID := range list.TransactionIDList {
		if existingTransactionID == transactionID {
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
	if transactionRole > transactionRoleSeller {
		return false, fmt.Errorf("transactionRole must be 0, 1, or 2")
	}

	list.TransactionIDList = append(list.TransactionIDList, transactionID)
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

// transactionInfoKey builds the ledger key for a transaction record.
func transactionInfoKey(transactionID uint) string {
	return fmt.Sprintf("TRANSACTION_INFO:%d", transactionID)
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

// getAssetCertificateAddress resolves an asset ID to its certificate address.
func getAssetCertificateAddress(ctx contractapi.TransactionContextInterface, assetID string) (string, error) {
	if err := validateRequired("assetID", assetID); err != nil {
		return "", err
	}

	assetID = strings.TrimSpace(assetID)
	data, err := ctx.GetStub().GetState(assetCertAddrKey(assetID))
	if err != nil {
		return "", fmt.Errorf("failed to read asset certificate address: %w", err)
	}
	if data == nil {
		return "", fmt.Errorf("asset certificate address not found for asset: %s", assetID)
	}

	return string(data), nil
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

// getPropertyIndex reads an asset owner record.
func getPropertyIndex(ctx contractapi.TransactionContextInterface, assetID string) (*PropertyIndex, error) {
	if err := validateRequired("assetID", assetID); err != nil {
		return nil, err
	}

	assetID = strings.TrimSpace(assetID)
	data, err := ctx.GetStub().GetState(propertyIndexKey(assetID))
	if err != nil {
		return nil, fmt.Errorf("failed to read property index: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("property index not found for asset: %s", assetID)
	}

	var property PropertyIndex
	if err := json.Unmarshal(data, &property); err != nil {
		return nil, fmt.Errorf("failed to decode property index: %w", err)
	}

	return &property, nil
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

// getTransactionInfo reads a transaction record by transaction ID.
func getTransactionInfo(ctx contractapi.TransactionContextInterface, transactionID uint) (*TransactionInfo, error) {
	data, err := ctx.GetStub().GetState(transactionInfoKey(transactionID))
	if err != nil {
		return nil, fmt.Errorf("failed to read transaction info: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("transaction info not found: %d", transactionID)
	}

	var info TransactionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to decode transaction info: %w", err)
	}
	info.TransactionID = transactionID

	return &info, nil
}

// putTransactionInfo writes a transaction record to world state.
func putTransactionInfo(ctx contractapi.TransactionContextInterface, info *TransactionInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to encode transaction info: %w", err)
	}

	if err := ctx.GetStub().PutState(transactionInfoKey(info.TransactionID), data); err != nil {
		return fmt.Errorf("failed to write transaction info: %w", err)
	}

	return nil
}

// nextTransactionID increments the deterministic ledger transaction counter.
func nextTransactionID(ctx contractapi.TransactionContextInterface) (uint, error) {
	data, err := ctx.GetStub().GetState(transactionCounterKey)
	if err != nil {
		return 0, fmt.Errorf("failed to read transaction counter: %w", err)
	}

	var current uint64
	if data != nil {
		current, err = strconv.ParseUint(string(data), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to decode transaction counter: %w", err)
		}
	}
	current++
	if current == 0 || uint64(uint(current)) != current {
		return 0, fmt.Errorf("transaction counter exceeds supported uint range")
	}

	if err := ctx.GetStub().PutState(transactionCounterKey, []byte(strconv.FormatUint(current, 10))); err != nil {
		return 0, fmt.Errorf("failed to write transaction counter: %w", err)
	}

	return uint(current), nil
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
			TransactionIDList:   []uint{},
			AssetIDList:         []string{},
			TransactionRoleList: []uint{},
			IsActiveList:        []bool{},
		}, nil
	}

	var list UserTransactionList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("failed to decode user transaction list: %w", err)
	}
	if list.TransactionIDList == nil {
		list.TransactionIDList = []uint{}
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
	if len(list.TransactionIDList) != len(list.AssetIDList) ||
		len(list.TransactionIDList) != len(list.TransactionRoleList) ||
		len(list.TransactionIDList) != len(list.IsActiveList) {
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

// transactionTimeInfo returns the Fabric transaction timestamp in TimeInfo form.
func transactionTimeInfo(ctx contractapi.TransactionContextInterface) (TimeInfo, time.Time, error) {
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return TimeInfo{}, time.Time{}, fmt.Errorf("failed to read transaction timestamp: %w", err)
	}

	value := time.Unix(ts.Seconds, int64(ts.Nanos)).UTC()

	return TimeInfo{
		Year:   uint(value.Year()),
		Month:  uint(value.Month()),
		Day:    uint(value.Day()),
		Hour:   uint(value.Hour()),
		Minute: uint(value.Minute()),
		Second: uint(value.Second()),
	}, value, nil
}

// validateTransactionTimeInfo validates a complete date and time value.
func validateTransactionTimeInfo(value TimeInfo) (time.Time, error) {
	if value.Year == 0 || value.Month == 0 || value.Day == 0 {
		return time.Time{}, fmt.Errorf("year, month, and day are required")
	}
	if value.Hour > 23 || value.Minute > 59 || value.Second > 59 {
		return time.Time{}, fmt.Errorf("time fields are out of range")
	}

	result := time.Date(
		int(value.Year),
		time.Month(value.Month),
		int(value.Day),
		int(value.Hour),
		int(value.Minute),
		int(value.Second),
		0,
		time.UTC,
	)
	if result.Year() != int(value.Year) ||
		uint(result.Month()) != value.Month ||
		uint(result.Day()) != value.Day {
		return time.Time{}, fmt.Errorf("date fields are invalid")
	}

	return result, nil
}
