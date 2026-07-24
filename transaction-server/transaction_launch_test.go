package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type contractCall struct {
	Name string
	Args []string
}

type fakeTransactionContract struct {
	legalStatus int
	ownerDID    string
	userDID     string
	sellerDID   string
	calls       []contractCall
}

func (f *fakeTransactionContract) EvaluateTransaction(name string, args ...string) ([]byte, error) {
	switch name {
	case "GetCertAddr":
		return []byte("ASSET_CERT:asset-1"), nil
	case "CheckStatus":
		return json.Marshal(f.legalStatus)
	case "GetBySellerDID":
		return []byte(f.userDID), nil
	case "GetPublicKey":
		return []byte("test-public-key"), nil
	case "CheckIdentityStatus":
		return json.Marshal(accountStatusAvailable)
	case "GetCreditScores":
		return json.Marshal(CreditScores{
			BuyerCreditScore:  80,
			SellerCreditScore: 80,
		})
	case "GetOwner":
		return []byte(f.ownerDID), nil
	case "GetTransactionInfo":
		return json.Marshal(TradeInfo{
			TransactionID:     101,
			AssetID:           "asset-1",
			SellerDID:         f.sellerDID,
			TransactionStatus: transactionStatusInProgress,
			TransactionMode:   transactionModeFixedPrice,
			FixedPrice:        500,
		})
	default:
		return nil, fmt.Errorf("unexpected evaluate call: %s", name)
	}
}

func (f *fakeTransactionContract) SubmitTransaction(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, contractCall{
		Name: name,
		Args: append([]string(nil), args...),
	})

	switch name {
	case "AddNewTransaction":
		return []byte("101"), nil
	case "UpdateTransactionList", "UpdateStatus", "ChangeTransactionStatus", "StartTransaction":
		return []byte("true"), nil
	default:
		return nil, fmt.Errorf("unexpected submit call: %s", name)
	}
}

func TestTransactionLaunchApproved(t *testing.T) {
	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	sessions := newSessionStore(time.Hour)
	session, err := sessions.Create("did:user:1", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	contract := &fakeTransactionContract{
		legalStatus: legalStatusNormal,
		ownerDID:    "did:user:1",
		userDID:     "did:user:1",
		sellerDID:   "did:seller:1",
	}
	cache := newActiveTransactionCache()
	handler := transactionLaunchHandlerWithDependencies(contract, sessions, cache, func() time.Time { return now })

	statusCode, response := performTransactionLaunchRequest(t, handler, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	})

	if statusCode != http.StatusOK {
		t.Fatalf("status = %d, response = %+v", statusCode, response)
	}
	if !response.Success || !response.Approved {
		t.Fatalf("response = %+v", response)
	}
	if response.TransactionStatus != transactionStatusInProgress || response.LegalStatus != legalStatusSelling {
		t.Fatalf("unexpected approved statuses: %+v", response)
	}

	wantCallNames := []string{
		"AddNewTransaction",
		"UpdateTransactionList",
		"UpdateStatus",
		"UpdateStatus",
		"StartTransaction",
	}
	if got := submittedCallNames(contract.calls); !reflect.DeepEqual(got, wantCallNames) {
		t.Fatalf("submitted calls = %v, want %v", got, wantCallNames)
	}
	if countSubmittedCall(contract.calls, "StartTransaction") != 1 {
		t.Fatalf("StartTransaction must be submitted exactly once")
	}
	if countSubmittedCall(contract.calls, "ChangeTransactionStatus") != 0 {
		t.Fatalf("approved path must not call ChangeTransactionStatus")
	}

	snapshot := cache.Snapshot()
	if len(snapshot) != 1 || snapshot[0].TransactionID != 101 {
		t.Fatalf("active transaction cache = %+v", snapshot)
	}
}

func TestTransactionLaunchRejectedRestoresPendingAsset(t *testing.T) {
	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	sessions := newSessionStore(time.Hour)
	session, err := sessions.Create("did:user:1", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	contract := &fakeTransactionContract{
		legalStatus: legalStatusNormal,
		ownerDID:    "did:user:other-owner",
		userDID:     "did:user:1",
		sellerDID:   "did:seller:1",
	}
	cache := newActiveTransactionCache()
	handler := transactionLaunchHandlerWithDependencies(contract, sessions, cache, func() time.Time { return now })

	statusCode, response := performTransactionLaunchRequest(t, handler, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	})

	if statusCode != http.StatusOK {
		t.Fatalf("status = %d, response = %+v", statusCode, response)
	}
	if !response.Success || response.Approved {
		t.Fatalf("response = %+v", response)
	}
	if response.TransactionStatus != transactionStatusRejected || response.LegalStatus != legalStatusNormal {
		t.Fatalf("unexpected rejected statuses: %+v", response)
	}
	if response.ReviewReason != "logged-in user is not the asset owner" {
		t.Fatalf("review reason = %q", response.ReviewReason)
	}

	wantCallNames := []string{
		"AddNewTransaction",
		"UpdateTransactionList",
		"UpdateStatus",
		"UpdateStatus",
		"ChangeTransactionStatus",
		"UpdateTransactionList",
	}
	if got := submittedCallNames(contract.calls); !reflect.DeepEqual(got, wantCallNames) {
		t.Fatalf("submitted calls = %v, want %v", got, wantCallNames)
	}
	if got := contract.calls[len(contract.calls)-1].Args[4]; got != "false" {
		t.Fatalf("final active status = %q, want false", got)
	}
	if len(cache.Snapshot()) != 0 {
		t.Fatalf("rejected transaction must not enter active cache")
	}
}

func TestTransactionLaunchRejectsInvalidSessionBeforeFabric(t *testing.T) {
	contract := &fakeTransactionContract{}
	handler := transactionLaunchHandlerWithDependencies(
		contract,
		newSessionStore(time.Hour),
		newActiveTransactionCache(),
		func() time.Time { return time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC) },
	)

	statusCode, _ := performTransactionLaunchRequest(t, handler, TransactionLaunchRequest{
		SessionToken:    "invalid",
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	})

	if statusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusUnauthorized)
	}
	if len(contract.calls) != 0 {
		t.Fatalf("invalid session must not submit Fabric transactions")
	}
}

func performTransactionLaunchRequest(
	t *testing.T,
	handler http.Handler,
	request TransactionLaunchRequest,
) (int, TransactionLaunchResponse) {
	t.Helper()

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	httpRequest := httptest.NewRequest(http.MethodPost, "/transactions/launch", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httpRequest)

	var response TransactionLaunchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v, body = %s", err, recorder.Body.String())
	}

	return recorder.Code, response
}

func submittedCallNames(calls []contractCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}

	return names
}

func countSubmittedCall(calls []contractCall, name string) int {
	count := 0
	for _, call := range calls {
		if call.Name == name {
			count++
		}
	}

	return count
}
