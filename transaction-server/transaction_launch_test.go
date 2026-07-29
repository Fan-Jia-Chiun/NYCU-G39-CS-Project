package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"
)

type contractCall struct {
	Name string
	Args []string
}

type fakeTransactionContract struct {
	legalStatus            int
	ownerDID               string
	userDID                string
	sellerDID              string
	sellerCreditScore      uint
	publicKeyText          string
	startTransactionResult *bool
	calls                  []contractCall
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
		return []byte(f.publicKeyText), nil
	case "CheckIdentityStatus":
		return json.Marshal(accountStatusAvailable)
	case "GetCreditScores":
		return json.Marshal(CreditScores{
			BuyerCreditScore:  80,
			SellerCreditScore: f.sellerCreditScore,
		})
	case "GetOwner":
		return []byte(f.ownerDID), nil
	case "CheckSellerEligibility":
		price, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return nil, err
		}
		qualification := evaluateSellerPriceQualification(
			f.sellerCreditScore,
			uint(price),
			transactionModeFixedPrice,
		)
		return json.Marshal(qualification.Eligible)
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
	case "StartTransaction":
		if f.startTransactionResult != nil {
			return json.Marshal(*f.startTransactionResult)
		}
		return []byte("true"), nil
	case "UpdateTransactionList", "UpdateStatus", "ChangeTransactionStatus":
		return []byte("true"), nil
	default:
		return nil, fmt.Errorf("unexpected submit call: %s", name)
	}
}

func TestBuildTransactionLaunchCredential(t *testing.T) {
	got := buildTransactionLaunchCredential(
		"did:nycu-g39:seller:abc",
		"asset-123",
		1,
		300000,
		"2026-07-30T08:30:00Z",
		"2026-07-29T08:30:00Z",
	)
	want := "TRANSACTION_LAUNCH|did:nycu-g39:seller:abc|asset-123|1|300000|2026-07-30T08:30:00Z|2026-07-29T08:30:00Z"
	if got != want {
		t.Fatalf("credential = %q, want %q", got, want)
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
		legalStatus:       legalStatusNormal,
		ownerDID:          "did:user:1",
		userDID:           "did:user:1",
		sellerDID:         "did:seller:1",
		sellerCreditScore: 80,
	}
	cache := newActiveTransactionCache()
	handler := transactionLaunchHandlerWithDependencies(contract, sessions, cache, func() time.Time { return now })

	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	}, now)
	statusCode, response := performTransactionLaunchRequest(t, handler, request)

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
		legalStatus:       legalStatusNormal,
		ownerDID:          "did:user:other-owner",
		userDID:           "did:user:1",
		sellerDID:         "did:seller:1",
		sellerCreditScore: 80,
	}
	cache := newActiveTransactionCache()
	handler := transactionLaunchHandlerWithDependencies(contract, sessions, cache, func() time.Time { return now })

	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	}, now)
	statusCode, response := performTransactionLaunchRequest(t, handler, request)

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
	changeCall := findSubmittedCall(contract.calls, "ChangeTransactionStatus")
	if changeCall == nil || !reflect.DeepEqual(changeCall.Args, []string{"101", "0", "10"}) {
		t.Fatalf("ChangeTransactionStatus args = %+v", changeCall)
	}
	if len(cache.Snapshot()) != 0 {
		t.Fatalf("rejected transaction must not enter active cache")
	}
}

func TestTransactionLaunchRejectsSellerBelowCreditThreshold(t *testing.T) {
	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	sessions := newSessionStore(time.Hour)
	session, err := sessions.Create("did:user:1", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	contract := &fakeTransactionContract{
		legalStatus:       legalStatusNormal,
		ownerDID:          "did:user:1",
		userDID:           "did:user:1",
		sellerDID:         "did:seller:1",
		sellerCreditScore: 59,
	}
	cache := newActiveTransactionCache()
	handler := transactionLaunchHandlerWithDependencies(contract, sessions, cache, func() time.Time { return now })

	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      1,
	}, now)
	statusCode, response := performTransactionLaunchRequest(t, handler, request)

	if statusCode != http.StatusOK || response.Approved {
		t.Fatalf("status = %d, response = %+v", statusCode, response)
	}
	if response.SellerPriceQualification == nil || response.SellerPriceQualification.Eligible {
		t.Fatalf("seller qualification = %+v", response.SellerPriceQualification)
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
	if countSubmittedCall(contract.calls, "StartTransaction") != 0 {
		t.Fatalf("ineligible seller must not start a transaction")
	}
}

func TestTransactionLaunchHandlesFinalEligibilityRecheck(t *testing.T) {
	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	sessions := newSessionStore(time.Hour)
	session, err := sessions.Create("did:user:1", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	startResult := false
	contract := &fakeTransactionContract{
		legalStatus:            legalStatusNormal,
		ownerDID:               "did:user:1",
		userDID:                "did:user:1",
		sellerDID:              "did:seller:1",
		sellerCreditScore:      80,
		startTransactionResult: &startResult,
	}
	cache := newActiveTransactionCache()
	handler := transactionLaunchHandlerWithDependencies(contract, sessions, cache, func() time.Time { return now })

	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	}, now)
	statusCode, response := performTransactionLaunchRequest(t, handler, request)

	if statusCode != http.StatusOK || response.Approved {
		t.Fatalf("status = %d, response = %+v", statusCode, response)
	}
	wantCallNames := []string{
		"AddNewTransaction",
		"UpdateTransactionList",
		"UpdateStatus",
		"UpdateStatus",
		"StartTransaction",
		"UpdateStatus",
		"ChangeTransactionStatus",
		"UpdateTransactionList",
	}
	if got := submittedCallNames(contract.calls); !reflect.DeepEqual(got, wantCallNames) {
		t.Fatalf("submitted calls = %v, want %v", got, wantCallNames)
	}
	if len(cache.Snapshot()) != 0 {
		t.Fatalf("final eligibility rejection must not enter active cache")
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

	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    "invalid",
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	}, now)
	statusCode, _ := performTransactionLaunchRequest(t, handler, request)

	if statusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusUnauthorized)
	}
	if len(contract.calls) != 0 {
		t.Fatalf("invalid session must not submit Fabric transactions")
	}
}

func TestTransactionLaunchRejectsModifiedSignedPriceBeforeFabric(t *testing.T) {
	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	sessions := newSessionStore(time.Hour)
	session, err := sessions.Create("did:user:1", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	contract := &fakeTransactionContract{
		userDID:   "did:user:1",
		sellerDID: "did:seller:1",
	}
	handler := transactionLaunchHandlerWithDependencies(
		contract,
		sessions,
		newActiveTransactionCache(),
		func() time.Time { return now },
	)
	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	}, now)
	request.BasicPrice++

	statusCode, response := performTransactionLaunchRequest(t, handler, request)
	if statusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, response = %+v", statusCode, response)
	}
	if response.Message != "signature verification failed" {
		t.Fatalf("message = %q", response.Message)
	}
	if len(contract.calls) != 0 {
		t.Fatalf("invalid signature must not submit Fabric transactions")
	}
}

func TestTransactionLaunchRejectsExpiredTimestampBeforeFabric(t *testing.T) {
	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	sessions := newSessionStore(time.Hour)
	session, err := sessions.Create("did:user:1", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	contract := &fakeTransactionContract{
		userDID:   "did:user:1",
		sellerDID: "did:seller:1",
	}
	handler := transactionLaunchHandlerWithDependencies(
		contract,
		sessions,
		newActiveTransactionCache(),
		func() time.Time { return now },
	)
	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	}, now.Add(-61*time.Second))

	statusCode, response := performTransactionLaunchRequest(t, handler, request)
	if statusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, response = %+v", statusCode, response)
	}
	if response.Message != "timestamp is outside the allowed window" {
		t.Fatalf("message = %q", response.Message)
	}
	if len(contract.calls) != 0 {
		t.Fatalf("expired request must not submit Fabric transactions")
	}
}

func TestTransactionLaunchRejectsInvalidSignatureEncodingBeforeFabric(t *testing.T) {
	now := time.Date(2026, 7, 24, 8, 30, 0, 0, time.UTC)
	sessions := newSessionStore(time.Hour)
	session, err := sessions.Create("did:user:1", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	contract := &fakeTransactionContract{
		userDID:   "did:user:1",
		sellerDID: "did:seller:1",
	}
	handler := transactionLaunchHandlerWithDependencies(
		contract,
		sessions,
		newActiveTransactionCache(),
		func() time.Time { return now },
	)
	request := signedTransactionLaunchRequest(t, contract, TransactionLaunchRequest{
		SessionToken:    session.Token,
		UserDID:         "did:user:1",
		SellerDID:       "did:seller:1",
		AssetID:         "asset-1",
		TransactionMode: transactionModeFixedPrice,
		BasicPrice:      500,
	}, now)
	request.Signature = "not-base64!"

	statusCode, response := performTransactionLaunchRequest(t, handler, request)
	if statusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, response = %+v", statusCode, response)
	}
	if response.Message != "signature is invalid base64" {
		t.Fatalf("message = %q", response.Message)
	}
	if len(contract.calls) != 0 {
		t.Fatalf("malformed signature must not submit Fabric transactions")
	}
}

func signedTransactionLaunchRequest(
	t *testing.T,
	contract *fakeTransactionContract,
	request TransactionLaunchRequest,
	now time.Time,
) TransactionLaunchRequest {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}
	contract.publicKeyText = base64.StdEncoding.EncodeToString(publicKeyDER)

	request.Timestamp = now.UTC().Format(time.RFC3339)
	credential := buildTransactionLaunchCredential(
		request.SellerDID,
		request.AssetID,
		request.TransactionMode,
		request.BasicPrice,
		request.FinalizingTime,
		request.Timestamp,
	)
	digest := sha256.Sum256([]byte(credential))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatalf("SignASN1() error = %v", err)
	}
	request.Signature = base64.StdEncoding.EncodeToString(signature)

	return request
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

func findSubmittedCall(calls []contractCall, name string) *contractCall {
	for i := range calls {
		if calls[i].Name == name {
			return &calls[i]
		}
	}

	return nil
}
