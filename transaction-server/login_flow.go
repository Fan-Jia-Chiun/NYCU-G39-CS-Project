package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	accountStatusAvailable    uint = 0
	accountStatusDisabled     uint = 1
	accountStatusDeregistered uint = 2

	transactionStatusCompleted uint = 5
	transactionStatusCancelled uint = 6
	transactionStatusReturned  uint = 9
	transactionStatusRejected  uint = 10
)

type LoginRequest struct {
	UserDID   string `json:"userDID"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

type LoginResponse struct {
	Success                   bool             `json:"success"`
	IdentityDID               string           `json:"identityDID"`
	UserDID                   string           `json:"userDID"`
	BuyerDID                  string           `json:"buyerDID"`
	SellerDID                 string           `json:"sellerDID"`
	Message                   string           `json:"message"`
	SessionToken              string           `json:"sessionToken"`
	ExpiresAt                 string           `json:"expiresAt"`
	SessionExpiresAt          string           `json:"sessionExpiresAt"`
	AccountStatus             uint             `json:"accountStatus"`
	CreditScores              CreditScores     `json:"creditScores"`
	Assets                    []AssetLoginInfo `json:"assets"`
	ActiveTrades              []TradeLoginInfo `json:"activeTrades"`
	HistoricalTrades          []TradeLoginInfo `json:"historicalTrades"`
	CurrentActiveTransactions []TradeInfo      `json:"currentActiveTransactions"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type CreditScores struct {
	BuyerCreditScore  uint `json:"buyerCreditScore"`
	SellerCreditScore uint `json:"sellerCreditScore"`
}

type AssetLoginInfo struct {
	AssetAddr   string `json:"assetAddr"`
	LegalStatus int    `json:"legalStatus"`
}

type TradeLoginInfo struct {
	TradeID         uint      `json:"tradeID"`
	TransactionRole uint      `json:"transactionRole"`
	IsActive        bool      `json:"isActive"`
	TradeInfo       TradeInfo `json:"tradeInfo"`
}

type TradeInfo struct {
	TradeID             uint   `json:"tradeID,omitempty"`
	AssetID             string `json:"assetID"`
	TransactionStatus   uint   `json:"transactionStatus"`
	TransactionMode     uint   `json:"transactionMode"`
	CurrentHighestPrice uint   `json:"currentHighestPrice"`
}

type TradeListResult struct {
	TradeIDList         []uint `json:"tradeIDList"`
	TransactionRoleList []uint `json:"transactionRoleList"`
	IsActiveList        []bool `json:"isActiveList"`
}

type TradingIdentityRecord struct {
	UserDID           string `json:"userDID"`
	BuyerDID          string `json:"buyerDID"`
	SellerDID         string `json:"sellerDID"`
	AccountStatus     uint   `json:"accountStatus"`
	BuyerCreditScore  uint   `json:"buyerCreditScore"`
	SellerCreditScore uint   `json:"sellerCreditScore"`
}

type LoginInitializationData struct {
	BuyerDID                  string
	SellerDID                 string
	AccountStatus             uint
	CreditScores              CreditScores
	Assets                    []AssetLoginInfo
	ActiveTrades              []TradeLoginInfo
	HistoricalTrades          []TradeLoginInfo
	CurrentActiveTransactions []TradeInfo
}

type publicKeyResolver func(userDID string) (string, error)
type loginInitializer func(userDID string) (*LoginInitializationData, error)

var currentActiveTransactionCache = []TradeInfo{}

func loginHandler(fabricGateway *FabricGateway) http.HandlerFunc {
	return loginHandlerWithDependencies(
		func(userDID string) (string, error) {
			result, err := fabricGateway.Contract.EvaluateTransaction("GetPublicKey", userDID)
			if err != nil {
				return "", err
			}
			return string(result), nil
		},
		func(userDID string) (*LoginInitializationData, error) {
			return loadLoginInitialization(fabricGateway, userDID)
		},
	)
}

func loginHandlerWithPublicKeyResolver(resolver publicKeyResolver) http.HandlerFunc {
	return loginHandlerWithDependencies(resolver, emptyLoginInitialization)
}

func loginHandlerWithDependencies(resolvePublicKey publicKeyResolver, initialize loginInitializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeLoginError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		defer r.Body.Close()

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeLoginError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		req.UserDID = strings.TrimSpace(req.UserDID)
		req.Timestamp = strings.TrimSpace(req.Timestamp)
		req.Signature = strings.TrimSpace(req.Signature)
		if req.UserDID == "" || req.Timestamp == "" || req.Signature == "" {
			writeLoginError(w, http.StatusBadRequest, "userDID, timestamp, and signature are required")
			return
		}

		if err := validateLoginTimestamp(req.Timestamp, nowUTC(), defaultLoginTimestampSkew); err != nil {
			switch err {
			case errInvalidTimestamp:
				writeLoginError(w, http.StatusBadRequest, "timestamp is invalid")
			case errExpiredTimestamp, errFutureTimestamp:
				writeLoginError(w, http.StatusUnauthorized, "timestamp is outside the allowed window")
			default:
				writeLoginError(w, http.StatusBadRequest, "timestamp is invalid")
			}
			return
		}

		publicKeyText, err := resolvePublicKey(req.UserDID)
		if err != nil {
			log.Printf("failed to get public key for DID %s: %v", req.UserDID, err)
			if isPublicKeyNotFoundError(err) {
				writeLoginError(w, http.StatusNotFound, "user public key not found")
				return
			}
			writeLoginError(w, http.StatusBadGateway, "failed to query public key")
			return
		}
		if strings.TrimSpace(publicKeyText) == "" {
			writeLoginError(w, http.StatusNotFound, "user public key not found")
			return
		}

		publicKey, err := parseECDSAPublicKey(publicKeyText)
		if err != nil {
			log.Printf("stored public key for DID %s is invalid: %v", req.UserDID, err)
			writeLoginError(w, http.StatusInternalServerError, "stored public key is invalid")
			return
		}

		if err := verifyLoginSignature(publicKey, req.UserDID, req.Timestamp, req.Signature); err != nil {
			if isSignatureFormatError(err) {
				writeLoginError(w, http.StatusBadRequest, "signature is invalid base64")
				return
			}
			writeLoginError(w, http.StatusUnauthorized, "signature verification failed")
			return
		}

		data, err := initialize(req.UserDID)
		if err != nil {
			if dataErr, ok := err.(*loginDataError); ok {
				writeLoginError(w, dataErr.statusCode, dataErr.message)
				return
			}
			log.Printf("failed to initialize login data for DID %s: %v", req.UserDID, err)
			writeLoginError(w, http.StatusBadGateway, "failed to initialize login data")
			return
		}

		session, err := loginSessions.Create(req.UserDID, nowUTC())
		if err != nil {
			log.Printf("failed to create login session for DID %s: %v", req.UserDID, err)
			writeLoginError(w, http.StatusInternalServerError, "failed to create login session")
			return
		}

		writeJSON(w, http.StatusOK, LoginResponse{
			Success:                   true,
			IdentityDID:               req.UserDID,
			UserDID:                   req.UserDID,
			BuyerDID:                  data.BuyerDID,
			SellerDID:                 data.SellerDID,
			Message:                   "login credential verified",
			SessionToken:              session.Token,
			ExpiresAt:                 session.ExpiresAt.UTC().Format(time.RFC3339),
			SessionExpiresAt:          session.ExpiresAt.UTC().Format(time.RFC3339),
			AccountStatus:             data.AccountStatus,
			CreditScores:              data.CreditScores,
			Assets:                    data.Assets,
			ActiveTrades:              data.ActiveTrades,
			HistoricalTrades:          data.HistoricalTrades,
			CurrentActiveTransactions: data.CurrentActiveTransactions,
		})
	}
}

func emptyLoginInitialization(userDID string) (*LoginInitializationData, error) {
	return &LoginInitializationData{
		AccountStatus:             accountStatusAvailable,
		Assets:                    []AssetLoginInfo{},
		ActiveTrades:              []TradeLoginInfo{},
		HistoricalTrades:          []TradeLoginInfo{},
		CurrentActiveTransactions: []TradeInfo{},
	}, nil
}

func loadLoginInitialization(fabricGateway *FabricGateway, userDID string) (*LoginInitializationData, error) {
	tradingIdentity, err := evaluateTradingIdentity(fabricGateway, userDID)
	if err != nil {
		return nil, newLoginDataError(http.StatusBadGateway, "failed to query trading identity", err)
	}
	if tradingIdentity.UserDID != "" && tradingIdentity.UserDID != userDID {
		return nil, newLoginDataError(http.StatusBadGateway, "trading identity does not match login DID", nil)
	}
	if tradingIdentity.AccountStatus != accountStatusAvailable {
		return nil, newLoginDataError(http.StatusForbidden, identityStatusMessage(tradingIdentity.AccountStatus), nil)
	}

	assets, err := evaluateUserAssets(fabricGateway, userDID)
	if err != nil {
		return nil, newLoginDataError(http.StatusBadGateway, "failed to query user assets", err)
	}

	activeTrades, historicalTrades, err := evaluateUserTrades(fabricGateway, userDID)
	if err != nil {
		return nil, newLoginDataError(http.StatusBadGateway, "failed to query user trades", err)
	}

	return &LoginInitializationData{
		BuyerDID:                  tradingIdentity.BuyerDID,
		SellerDID:                 tradingIdentity.SellerDID,
		AccountStatus:             tradingIdentity.AccountStatus,
		CreditScores:              tradingIdentity.CreditScores(),
		Assets:                    assets,
		ActiveTrades:              activeTrades,
		HistoricalTrades:          historicalTrades,
		CurrentActiveTransactions: append([]TradeInfo(nil), currentActiveTransactionCache...),
	}, nil
}

func evaluateTradingIdentity(fabricGateway *FabricGateway, userDID string) (TradingIdentityRecord, error) {
	var record TradingIdentityRecord
	if err := evaluateJSON(fabricGateway, &record, "GetTradingIdentity", userDID); err != nil {
		return TradingIdentityRecord{}, err
	}

	return record, nil
}

func (r TradingIdentityRecord) CreditScores() CreditScores {
	return CreditScores{
		BuyerCreditScore:  r.BuyerCreditScore,
		SellerCreditScore: r.SellerCreditScore,
	}
}

func evaluateUint(fabricGateway *FabricGateway, fn string, args ...string) (uint, error) {
	result, err := fabricGateway.Contract.EvaluateTransaction(fn, args...)
	if err != nil {
		return 0, err
	}

	return decodeUint(result)
}

func evaluateCreditScores(fabricGateway *FabricGateway, userDID string) (CreditScores, error) {
	var scores CreditScores
	if err := evaluateJSON(fabricGateway, &scores, "GetCreditScores", userDID); err != nil {
		return CreditScores{}, err
	}

	return scores, nil
}

func evaluateUserAssets(fabricGateway *FabricGateway, userDID string) ([]AssetLoginInfo, error) {
	var assetAddrs []string
	if err := evaluateJSON(fabricGateway, &assetAddrs, "GetAssetList", userDID); err != nil {
		return nil, err
	}
	if assetAddrs == nil {
		assetAddrs = []string{}
	}

	assets := make([]AssetLoginInfo, 0, len(assetAddrs))
	for _, assetAddr := range assetAddrs {
		assetAddr = strings.TrimSpace(assetAddr)
		if assetAddr == "" {
			continue
		}

		status, err := evaluateInt(fabricGateway, "CheckStatus", assetAddr)
		if err != nil {
			return nil, err
		}
		assets = append(assets, AssetLoginInfo{
			AssetAddr:   assetAddr,
			LegalStatus: status,
		})
	}

	return assets, nil
}

func evaluateUserTrades(fabricGateway *FabricGateway, userDID string) ([]TradeLoginInfo, []TradeLoginInfo, error) {
	var tradeList TradeListResult
	if err := evaluateJSON(fabricGateway, &tradeList, "GetTradeList", userDID); err != nil {
		return nil, nil, err
	}
	if tradeList.TradeIDList == nil {
		tradeList.TradeIDList = []uint{}
	}
	if tradeList.TransactionRoleList == nil {
		tradeList.TransactionRoleList = []uint{}
	}
	if tradeList.IsActiveList == nil {
		tradeList.IsActiveList = []bool{}
	}
	if len(tradeList.TradeIDList) != len(tradeList.TransactionRoleList) || len(tradeList.TradeIDList) != len(tradeList.IsActiveList) {
		return nil, nil, fmt.Errorf("trade list has inconsistent field lengths")
	}

	activeTrades := []TradeLoginInfo{}
	historicalTrades := []TradeLoginInfo{}
	for i, tradeID := range tradeList.TradeIDList {
		var tradeInfo TradeInfo
		if err := evaluateJSON(fabricGateway, &tradeInfo, "GetTradeInfo", strconv.FormatUint(uint64(tradeID), 10)); err != nil {
			return nil, nil, err
		}
		tradeInfo.TradeID = tradeID

		actualActive := isTradeStatusActive(tradeInfo.TransactionStatus)
		if actualActive != tradeList.IsActiveList[i] {
			if err := updateTradeActiveStatus(fabricGateway, userDID, tradeID, tradeInfo.AssetID, tradeList.TransactionRoleList[i], actualActive); err != nil {
				return nil, nil, err
			}
		}

		entry := TradeLoginInfo{
			TradeID:         tradeID,
			TransactionRole: tradeList.TransactionRoleList[i],
			IsActive:        actualActive,
			TradeInfo:       tradeInfo,
		}
		if actualActive {
			activeTrades = append(activeTrades, entry)
		} else {
			historicalTrades = append(historicalTrades, entry)
		}
	}

	return activeTrades, historicalTrades, nil
}

func updateTradeActiveStatus(fabricGateway *FabricGateway, userDID string, tradeID uint, assetID string, transactionRole uint, isActive bool) error {
	_, err := fabricGateway.Contract.SubmitTransaction(
		"UpdateTradeList",
		userDID,
		strconv.FormatUint(uint64(tradeID), 10),
		assetID,
		strconv.FormatUint(uint64(transactionRole), 10),
		strconv.FormatBool(isActive),
	)

	return err
}

func evaluateJSON(fabricGateway *FabricGateway, target any, fn string, args ...string) error {
	result, err := fabricGateway.Contract.EvaluateTransaction(fn, args...)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(result))) == 0 {
		return fmt.Errorf("%s returned empty result", fn)
	}

	return json.Unmarshal(result, target)
}

func evaluateInt(fabricGateway *FabricGateway, fn string, args ...string) (int, error) {
	result, err := fabricGateway.Contract.EvaluateTransaction(fn, args...)
	if err != nil {
		return 0, err
	}

	return decodeInt(result)
}

func decodeUint(result []byte) (uint, error) {
	text := strings.TrimSpace(string(result))
	if text == "" {
		return 0, fmt.Errorf("empty uint result")
	}

	var value uint
	if err := json.Unmarshal(result, &value); err == nil {
		return value, nil
	}

	parsed, err := strconv.ParseUint(text, 10, 0)
	if err != nil {
		return 0, fmt.Errorf("invalid uint result: %w", err)
	}

	return uint(parsed), nil
}

func decodeInt(result []byte) (int, error) {
	text := strings.TrimSpace(string(result))
	if text == "" {
		return 0, fmt.Errorf("empty int result")
	}

	var value int
	if err := json.Unmarshal(result, &value); err == nil {
		return value, nil
	}

	parsed, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid int result: %w", err)
	}

	return parsed, nil
}

func isTradeStatusActive(status uint) bool {
	switch status {
	case transactionStatusCompleted, transactionStatusCancelled, transactionStatusReturned, transactionStatusRejected:
		return false
	default:
		return true
	}
}

func identityStatusMessage(status uint) string {
	switch status {
	case accountStatusDisabled:
		return "user is disabled"
	case accountStatusDeregistered:
		return "user is deregistered"
	default:
		return "user is not available"
	}
}

func writeLoginError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, ErrorResponse{
		Success: false,
		Message: message,
	})
}

type loginDataError struct {
	statusCode int
	message    string
	cause      error
}

func newLoginDataError(statusCode int, message string, cause error) *loginDataError {
	return &loginDataError{
		statusCode: statusCode,
		message:    message,
		cause:      cause,
	}
}

func (e *loginDataError) Error() string {
	if e.cause == nil {
		return e.message
	}

	return e.message + ": " + e.cause.Error()
}
