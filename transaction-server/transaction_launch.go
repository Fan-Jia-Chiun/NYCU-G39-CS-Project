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

type transactionContractClient interface {
	EvaluateTransaction(name string, args ...string) ([]byte, error)
	SubmitTransaction(name string, args ...string) ([]byte, error)
}

type TransactionLaunchRequest struct {
	SessionToken    string `json:"sessionToken"`
	UserDID         string `json:"userDID"`
	SellerDID       string `json:"sellerDID"`
	AssetID         string `json:"assetID"`
	TransactionMode uint   `json:"transactionMode"`
	BasicPrice      uint   `json:"basicPrice"`
	FinalizingTime  string `json:"finalizingTime,omitempty"`
}

type TransactionLaunchResponse struct {
	Success           bool      `json:"success"`
	Approved          bool      `json:"approved"`
	Message           string    `json:"message"`
	ReviewReason      string    `json:"reviewReason,omitempty"`
	TransactionID     uint      `json:"transactionID"`
	AssetID           string    `json:"assetID"`
	TransactionMode   uint      `json:"transactionMode"`
	TransactionStatus uint      `json:"transactionStatus"`
	LegalStatus       int       `json:"legalStatus"`
	SellerCreditScore uint      `json:"sellerCreditScore,omitempty"`
	TransactionInfo   TradeInfo `json:"transactionInfo,omitempty"`
}

func transactionLaunchHandler(fabricGateway *FabricGateway) http.HandlerFunc {
	return transactionLaunchHandlerWithDependencies(
		fabricGateway.Contract,
		loginSessions,
		currentActiveTransactions,
		nowUTC,
	)
}

func transactionLaunchHandlerWithDependencies(
	contract transactionContractClient,
	sessions *sessionStore,
	activeCache *activeTransactionCache,
	now func() time.Time,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeLoginError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		defer r.Body.Close()

		var req TransactionLaunchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeLoginError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.SessionToken = strings.TrimSpace(req.SessionToken)
		req.UserDID = strings.TrimSpace(req.UserDID)
		req.SellerDID = strings.TrimSpace(req.SellerDID)
		req.AssetID = strings.TrimSpace(req.AssetID)
		req.FinalizingTime = strings.TrimSpace(req.FinalizingTime)

		if req.SessionToken == "" || req.UserDID == "" {
			writeLoginError(w, http.StatusBadRequest, "sessionToken and userDID are required")
			return
		}
		if _, err := sessions.Validate(req.SessionToken, req.UserDID, now()); err != nil {
			writeSessionValidationError(w, err)
			return
		}
		if err := validateTransactionLaunchRequest(req, now()); err != nil {
			writeLoginError(w, http.StatusBadRequest, err.Error())
			return
		}

		finalizingTime, err := transactionFinalizingTime(req)
		if err != nil {
			writeLoginError(w, http.StatusBadRequest, err.Error())
			return
		}

		transactionID, err := addNewTransaction(contract, req.AssetID, req.SellerDID)
		if err != nil {
			log.Printf("failed to add transaction for asset %s: %v", req.AssetID, err)
			writeLoginError(w, http.StatusBadGateway, "failed to create transaction")
			return
		}
		if err := updateUserTransactionList(contract, req.UserDID, transactionID, req.AssetID, transactionRoleSeller, true); err != nil {
			log.Printf("failed to index transaction %d for user %s: %v", transactionID, req.UserDID, err)
			if rejectErr := rejectTransaction(contract, req, transactionID, "", false); rejectErr != nil {
				log.Printf("failed to reject transaction %d after index error: %v", transactionID, rejectErr)
			}
			writeLoginError(w, http.StatusBadGateway, "failed to update user transaction list")
			return
		}

		assetAddr, err := evaluateString(contract, "GetCertAddr", req.AssetID)
		if err != nil || assetAddr == "" {
			log.Printf("failed to resolve asset certificate for %s: %v", req.AssetID, err)
			if rejectErr := rejectTransaction(contract, req, transactionID, "", false); rejectErr != nil {
				log.Printf("failed to reject transaction %d after certificate lookup failure: %v", transactionID, rejectErr)
			}
			writeLoginError(w, http.StatusBadGateway, "failed to query asset certificate")
			return
		}

		legalStatus, err := evaluateIntContract(contract, "CheckStatus", assetAddr)
		if err != nil {
			log.Printf("failed to query legal status for asset %s: %v", req.AssetID, err)
			if rejectErr := rejectTransaction(contract, req, transactionID, assetAddr, false); rejectErr != nil {
				log.Printf("failed to reject transaction %d after legal status query error: %v", transactionID, rejectErr)
			}
			writeLoginError(w, http.StatusBadGateway, "failed to query asset legal status")
			return
		}
		if legalStatus != legalStatusNormal {
			if err := rejectTransaction(contract, req, transactionID, assetAddr, false); err != nil {
				log.Printf("failed to reject transaction %d: %v", transactionID, err)
				writeLoginError(w, http.StatusBadGateway, "failed to finalize rejected transaction")
				return
			}
			writeJSON(w, http.StatusOK, rejectedTransactionLaunchResponse(
				req,
				transactionID,
				legalStatus,
				"asset legal status is not Normal",
				0,
			))
			return
		}

		if err := submitBool(contract, "UpdateStatus", assetAddr, strconv.Itoa(legalStatusPending)); err != nil {
			log.Printf("failed to set asset %s legal status to Pending: %v", req.AssetID, err)
			if rejectErr := rejectTransaction(contract, req, transactionID, assetAddr, true); rejectErr != nil {
				log.Printf("failed to reject transaction %d after Pending status error: %v", transactionID, rejectErr)
			}
			writeLoginError(w, http.StatusBadGateway, "failed to update asset legal status")
			return
		}

		_, scores, ownerDID, reviewReason, err := reviewTransactionLaunch(contract, req)
		if err != nil {
			log.Printf("failed to review transaction %d: %v", transactionID, err)
			if rejectErr := rejectTransaction(contract, req, transactionID, assetAddr, true); rejectErr != nil {
				log.Printf("failed to reject transaction %d after review error: %v", transactionID, rejectErr)
			}
			writeLoginError(w, http.StatusBadGateway, "failed to review transaction")
			return
		}
		if reviewReason != "" {
			if err := rejectTransaction(contract, req, transactionID, assetAddr, true); err != nil {
				log.Printf("failed to reject transaction %d: %v", transactionID, err)
				writeLoginError(w, http.StatusBadGateway, "failed to finalize rejected transaction")
				return
			}
			log.Printf(
				"transaction %d rejected for user %s seller %s owner %s: %s",
				transactionID,
				req.UserDID,
				req.SellerDID,
				ownerDID,
				reviewReason,
			)
			writeJSON(w, http.StatusOK, rejectedTransactionLaunchResponse(
				req,
				transactionID,
				legalStatusNormal,
				reviewReason,
				scores.SellerCreditScore,
			))
			return
		}

		approvedLegalStatus := legalStatusSelling
		if req.TransactionMode != transactionModeFixedPrice {
			approvedLegalStatus = legalStatusBidding
		}
		if err := submitBool(contract, "UpdateStatus", assetAddr, strconv.Itoa(approvedLegalStatus)); err != nil {
			log.Printf("failed to approve legal status for transaction %d: %v", transactionID, err)
			if rejectErr := rejectTransaction(contract, req, transactionID, assetAddr, true); rejectErr != nil {
				log.Printf("failed to reject transaction %d after legal status error: %v", transactionID, rejectErr)
			}
			writeLoginError(w, http.StatusBadGateway, "failed to approve asset legal status")
			return
		}

		if err := startTransaction(contract, transactionID, req.BasicPrice, req.TransactionMode, finalizingTime); err != nil {
			log.Printf("failed to start transaction %d: %v", transactionID, err)
			if rejectErr := rejectTransaction(contract, req, transactionID, assetAddr, true); rejectErr != nil {
				log.Printf("failed to reject transaction %d after start error: %v", transactionID, rejectErr)
			}
			writeLoginError(w, http.StatusBadGateway, "failed to start transaction")
			return
		}

		transactionInfo := transactionInfoFromLaunch(req, transactionID, finalizingTime)
		if err := evaluateJSONContract(contract, &transactionInfo, "GetTransactionInfo", strconv.FormatUint(uint64(transactionID), 10)); err != nil {
			log.Printf("failed to load started transaction %d: %v", transactionID, err)
		}
		transactionInfo.TransactionID = transactionID
		activeCache.Add(transactionInfo)

		writeJSON(w, http.StatusOK, TransactionLaunchResponse{
			Success:           true,
			Approved:          true,
			Message:           "transaction approved and started",
			TransactionID:     transactionID,
			AssetID:           req.AssetID,
			TransactionMode:   req.TransactionMode,
			TransactionStatus: transactionStatusInProgress,
			LegalStatus:       approvedLegalStatus,
			SellerCreditScore: scores.SellerCreditScore,
			TransactionInfo:   transactionInfo,
		})
	}
}

func validateTransactionLaunchRequest(req TransactionLaunchRequest, now time.Time) error {
	if req.SellerDID == "" {
		return fmt.Errorf("sellerDID is required")
	}
	if req.AssetID == "" {
		return fmt.Errorf("assetID is required")
	}
	if req.BasicPrice == 0 {
		return fmt.Errorf("basicPrice must be greater than zero")
	}
	if req.TransactionMode > transactionModeSealedBid {
		return fmt.Errorf("transactionMode must be 0, 1, or 2")
	}
	if req.TransactionMode == transactionModeFixedPrice {
		return nil
	}
	if req.FinalizingTime == "" {
		return fmt.Errorf("finalizingTime is required for auction modes")
	}

	finalizesAt, err := time.Parse(time.RFC3339, req.FinalizingTime)
	if err != nil {
		return fmt.Errorf("finalizingTime must use RFC3339")
	}
	if !finalizesAt.After(now) {
		return fmt.Errorf("finalizingTime must be in the future")
	}

	return nil
}

func transactionFinalizingTime(req TransactionLaunchRequest) (TimeInfo, error) {
	if req.TransactionMode == transactionModeFixedPrice {
		return TimeInfo{}, nil
	}

	finalizesAt, err := time.Parse(time.RFC3339, req.FinalizingTime)
	if err != nil {
		return TimeInfo{}, fmt.Errorf("finalizingTime must use RFC3339")
	}

	return timeInfoFromTime(finalizesAt), nil
}

func transactionInfoFromLaunch(req TransactionLaunchRequest, transactionID uint, finalizingTime TimeInfo) TradeInfo {
	info := TradeInfo{
		TransactionID:     transactionID,
		AssetID:           req.AssetID,
		SellerDID:         req.SellerDID,
		TransactionStatus: transactionStatusInProgress,
		TransactionMode:   req.TransactionMode,
		FinalizingTime:    finalizingTime,
	}
	if req.TransactionMode == transactionModeFixedPrice {
		info.FixedPrice = req.BasicPrice
	} else {
		info.BasicPrice = req.BasicPrice
	}

	return info
}

func addNewTransaction(contract transactionContractClient, assetID string, sellerDID string) (uint, error) {
	result, err := contract.SubmitTransaction("AddNewTransaction", assetID, sellerDID)
	if err != nil {
		return 0, err
	}

	return decodeUint(result)
}

func reviewTransactionLaunch(
	contract transactionContractClient,
	req TransactionLaunchRequest,
) (string, CreditScores, string, string, error) {
	userDID, err := evaluateString(contract, "GetBySellerDID", req.SellerDID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return "", CreditScores{}, "", "seller DID is not registered", nil
		}
		return "", CreditScores{}, "", "", err
	}

	publicKey, err := evaluateString(contract, "GetPublicKey", userDID)
	if err != nil {
		if isPublicKeyNotFoundError(err) {
			return userDID, CreditScores{}, "", "seller public key is not registered", nil
		}
		return "", CreditScores{}, "", "", err
	}

	accountStatus, err := evaluateUintContract(contract, "CheckIdentityStatus", userDID)
	if err != nil {
		return "", CreditScores{}, "", "", err
	}

	var scores CreditScores
	if err := evaluateJSONContract(contract, &scores, "GetCreditScores", userDID); err != nil {
		return "", CreditScores{}, "", "", err
	}

	ownerDID, err := evaluateString(contract, "GetOwner", req.AssetID)
	if err != nil {
		return "", CreditScores{}, "", "", err
	}

	switch {
	case userDID != req.UserDID:
		return userDID, scores, ownerDID, "seller DID does not belong to the logged-in user", nil
	case accountStatus != accountStatusAvailable:
		return userDID, scores, ownerDID, "seller account is not available", nil
	case strings.TrimSpace(publicKey) == "":
		return userDID, scores, ownerDID, "seller public key is not registered", nil
	case ownerDID != req.UserDID:
		return userDID, scores, ownerDID, "logged-in user is not the asset owner", nil
	default:
		return userDID, scores, ownerDID, "", nil
	}
}

func rejectTransaction(
	contract transactionContractClient,
	req TransactionLaunchRequest,
	transactionID uint,
	assetAddr string,
	restoreLegalStatus bool,
) error {
	if restoreLegalStatus {
		if err := submitBool(contract, "UpdateStatus", assetAddr, strconv.Itoa(legalStatusNormal)); err != nil {
			return fmt.Errorf("failed to restore legal status: %w", err)
		}
	}
	if err := submitBool(
		contract,
		"ChangeTransactionStatus",
		strconv.FormatUint(uint64(transactionID), 10),
		strconv.FormatUint(uint64(transactionStatusRejected), 10),
	); err != nil {
		return fmt.Errorf("failed to reject transaction: %w", err)
	}
	if err := updateUserTransactionList(contract, req.UserDID, transactionID, req.AssetID, transactionRoleSeller, false); err != nil {
		return fmt.Errorf("failed to deactivate user transaction: %w", err)
	}

	return nil
}

func updateUserTransactionList(
	contract transactionContractClient,
	userDID string,
	transactionID uint,
	assetID string,
	transactionRole uint,
	isActive bool,
) error {
	return submitBool(
		contract,
		"UpdateTransactionList",
		userDID,
		strconv.FormatUint(uint64(transactionID), 10),
		assetID,
		strconv.FormatUint(uint64(transactionRole), 10),
		strconv.FormatBool(isActive),
	)
}

func startTransaction(
	contract transactionContractClient,
	transactionID uint,
	basicPrice uint,
	transactionMode uint,
	finalizingTime TimeInfo,
) error {
	finalizingTimeJSON, err := json.Marshal(finalizingTime)
	if err != nil {
		return err
	}

	return submitBool(
		contract,
		"StartTransaction",
		strconv.FormatUint(uint64(transactionID), 10),
		strconv.FormatUint(uint64(basicPrice), 10),
		strconv.FormatUint(uint64(transactionMode), 10),
		string(finalizingTimeJSON),
	)
}

func submitBool(contract transactionContractClient, name string, args ...string) error {
	result, err := contract.SubmitTransaction(name, args...)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(result))) == 0 {
		return nil
	}

	var success bool
	if err := json.Unmarshal(result, &success); err != nil {
		return fmt.Errorf("%s returned invalid boolean result", name)
	}
	if !success {
		return fmt.Errorf("%s returned false", name)
	}

	return nil
}

func evaluateJSONContract(contract transactionContractClient, target any, name string, args ...string) error {
	result, err := contract.EvaluateTransaction(name, args...)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(result))) == 0 {
		return fmt.Errorf("%s returned empty result", name)
	}

	return json.Unmarshal(result, target)
}

func evaluateString(contract transactionContractClient, name string, args ...string) (string, error) {
	result, err := contract.EvaluateTransaction(name, args...)
	if err != nil {
		return "", err
	}

	var value string
	if err := json.Unmarshal(result, &value); err == nil {
		return strings.TrimSpace(value), nil
	}

	return strings.TrimSpace(string(result)), nil
}

func evaluateIntContract(contract transactionContractClient, name string, args ...string) (int, error) {
	result, err := contract.EvaluateTransaction(name, args...)
	if err != nil {
		return 0, err
	}

	return decodeInt(result)
}

func evaluateUintContract(contract transactionContractClient, name string, args ...string) (uint, error) {
	result, err := contract.EvaluateTransaction(name, args...)
	if err != nil {
		return 0, err
	}

	return decodeUint(result)
}

func rejectedTransactionLaunchResponse(
	req TransactionLaunchRequest,
	transactionID uint,
	legalStatus int,
	reason string,
	sellerCreditScore uint,
) TransactionLaunchResponse {
	return TransactionLaunchResponse{
		Success:           true,
		Approved:          false,
		Message:           "transaction application rejected",
		ReviewReason:      reason,
		TransactionID:     transactionID,
		AssetID:           req.AssetID,
		TransactionMode:   req.TransactionMode,
		TransactionStatus: transactionStatusRejected,
		LegalStatus:       legalStatus,
		SellerCreditScore: sellerCreditScore,
	}
}

func writeSessionValidationError(w http.ResponseWriter, err error) {
	switch err {
	case errSessionExpired:
		writeLoginError(w, http.StatusUnauthorized, "session expired; please log in again")
	case errSessionMismatch:
		writeLoginError(w, http.StatusForbidden, "session does not match userDID")
	default:
		writeLoginError(w, http.StatusUnauthorized, "session is invalid; please log in again")
	}
}
