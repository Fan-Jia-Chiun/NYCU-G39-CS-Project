package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	msppb "github.com/hyperledger/fabric-protos-go/msp"
)

func TestTransactionLaunchContractLifecycle(t *testing.T) {
	chaincode, err := contractapi.NewChaincode(&TransactionIdentityRegistryContract{})
	if err != nil {
		t.Fatalf("NewChaincode() error = %v", err)
	}

	stub := shimtest.NewMockStub("transaction", chaincode)
	stub.Creator = transactionServiceCreator(t)

	userDID := "did:nycu-g39:identity:test"
	sellerDID := "did:nycu-g39:seller:test"
	putMockJSON(t, stub, "setup-identity", tradingIdentityKey(userDID), TradingIdentity{
		ObjectType:        objectTypeTradingIdentity,
		UserDID:           userDID,
		SellerDID:         sellerDID,
		AccountStatus:     accountStatusAvailable,
		SellerCreditScore: 80,
	})
	putMockState(t, stub, "setup-seller-index", sellerDIDKey(sellerDID), []byte(userDID))

	assetID := "asset:test"
	assetAddr := assetCertificateKey(assetID)
	putMockState(t, stub, "setup-asset", assetCertAddrKey(assetID), []byte(assetAddr))
	putMockJSON(t, stub, "setup-cert", assetAddr, AssetCertificate{
		ObjectType:  objectTypeAssetCert,
		AssetID:     assetID,
		LegalStatus: legalStatusNormal,
	})

	response := stub.MockInvoke("add-transaction", byteArgs(
		"AddNewTransaction",
		assetID,
		sellerDID,
	))
	if response.Status != 200 {
		t.Fatalf("AddNewTransaction failed: %s", response.Message)
	}

	transactionID := uint(1)
	info := readTransactionInfoState(t, stub, transactionID)
	if info.TransactionStatus != transactionStatusReviewing {
		t.Fatalf("transaction status = %d, want Reviewing", info.TransactionStatus)
	}
	if info.AssetID != assetID || info.SellerDID != sellerDID {
		t.Fatalf("transaction info = %+v", info)
	}

	response = stub.MockInvoke("get-reviewing-transaction", byteArgs(
		"GetTransactionInfo",
		"1",
	))
	if response.Status != 200 {
		t.Fatalf("GetTransactionInfo for Reviewing transaction failed: %s", response.Message)
	}

	response = stub.MockInvoke("invalid-direct-selling", byteArgs(
		"UpdateStatus",
		assetAddr,
		"2",
	))
	if response.Status == 200 {
		t.Fatalf("UpdateStatus must reject a direct Normal-to-Selling transition")
	}
	var unchangedCert AssetCertificate
	readMockJSON(t, stub, assetAddr, &unchangedCert)
	if unchangedCert.LegalStatus != legalStatusNormal {
		t.Fatalf("legal status = %d, want Normal after rejected transition", unchangedCert.LegalStatus)
	}

	response = stub.MockInvoke("index-transaction", byteArgs(
		"UpdateTransactionList",
		userDID,
		"1",
		assetID,
		"2",
		"true",
	))
	if response.Status != 200 {
		t.Fatalf("UpdateTransactionList insert failed: %s", response.Message)
	}

	response = stub.MockInvoke("set-pending", byteArgs(
		"UpdateStatus",
		assetAddr,
		"1",
	))
	if response.Status != 200 {
		t.Fatalf("UpdateStatus Pending failed: %s", response.Message)
	}
	response = stub.MockInvoke("set-bidding", byteArgs(
		"UpdateStatus",
		assetAddr,
		"3",
	))
	if response.Status != 200 {
		t.Fatalf("UpdateStatus Bidding failed: %s", response.Message)
	}

	finalizingTime := TimeInfo{Year: 2099, Month: 7, Day: 24, Hour: 8, Minute: 30}
	finalizingTimeJSON, err := json.Marshal(finalizingTime)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	response = stub.MockInvoke("ineligible-start", byteArgs(
		"StartTransaction",
		"1",
		"1000001",
		"1",
		string(finalizingTimeJSON),
	))
	if response.Status != 200 || string(response.Payload) != "false" {
		t.Fatalf("StartTransaction must reject a price above the seller limit: status %d payload %s message %s", response.Status, response.Payload, response.Message)
	}
	info = readTransactionInfoState(t, stub, transactionID)
	if info.TransactionStatus != transactionStatusReviewing {
		t.Fatalf("ineligible transaction status = %d, want Reviewing", info.TransactionStatus)
	}

	response = stub.MockInvoke("start-transaction", byteArgs(
		"StartTransaction",
		"1",
		"500",
		"1",
		string(finalizingTimeJSON),
	))
	if response.Status != 200 {
		t.Fatalf("StartTransaction failed: %s", response.Message)
	}

	info = readTransactionInfoState(t, stub, transactionID)
	if info.TransactionStatus != transactionStatusInProgress {
		t.Fatalf("transaction status = %d, want In Progress", info.TransactionStatus)
	}
	if info.TransactionMode != transactionModeBidding || info.BasicPrice != 500 || info.FixedPrice != 0 {
		t.Fatalf("mode-specific transaction info = %+v", info)
	}
	if info.StartTime.Year == 0 || info.FinalizingTime != finalizingTime {
		t.Fatalf("transaction times = start %+v, finalizing %+v", info.StartTime, info.FinalizingTime)
	}

	response = stub.MockInvoke("invalid-status-change", byteArgs(
		"ChangeTransactionStatus",
		"1",
		"1",
		"1",
	))
	if response.Status != 200 || string(response.Payload) != "false" {
		t.Fatalf("ChangeTransactionStatus must return false for In Progress: status %d payload %s", response.Status, response.Payload)
	}

	response = stub.MockInvoke("stale-status-change", byteArgs(
		"ChangeTransactionStatus",
		"1",
		"0",
		"10",
	))
	if response.Status != 200 || string(response.Payload) != "false" {
		t.Fatalf("ChangeTransactionStatus must reject a stale originalStatus: status %d payload %s", response.Status, response.Payload)
	}

	response = stub.MockInvoke("reject-transaction", byteArgs(
		"ChangeTransactionStatus",
		"1",
		"1",
		"10",
	))
	if response.Status != 200 {
		t.Fatalf("ChangeTransactionStatus reject failed: %s", response.Message)
	}
	response = stub.MockInvoke("deactivate-transaction", byteArgs(
		"UpdateTransactionList",
		userDID,
		"1",
		assetID,
		"2",
		"false",
	))
	if response.Status != 200 {
		t.Fatalf("UpdateTransactionList deactivate failed: %s", response.Message)
	}

	var list UserTransactionList
	readMockJSON(t, stub, userTransactionListKey(userDID), &list)
	if len(list.TransactionIDList) != 1 || list.TransactionIDList[0] != transactionID {
		t.Fatalf("transaction history was not preserved: %+v", list)
	}
	if list.IsActiveList[0] {
		t.Fatalf("rejected transaction must be inactive: %+v", list)
	}
}

func TestSellerPriceEligibilityBoundaries(t *testing.T) {
	tests := []struct {
		score uint
		price uint
		want  bool
	}{
		{score: 59, price: 1, want: false},
		{score: 60, price: 100_000, want: true},
		{score: 60, price: 100_001, want: false},
		{score: 69, price: 100_000, want: true},
		{score: 70, price: 300_000, want: true},
		{score: 70, price: 300_001, want: false},
		{score: 79, price: 300_000, want: true},
		{score: 80, price: 1_000_000, want: true},
		{score: 80, price: 1_000_001, want: false},
		{score: 89, price: 1_000_000, want: true},
		{score: 90, price: 100_000_000, want: true},
		{score: 100, price: 100_000_000, want: true},
		{score: 101, price: 1, want: false},
	}

	for _, tt := range tests {
		if got := sellerPriceEligible(tt.score, tt.price); got != tt.want {
			t.Fatalf("sellerPriceEligible(%d, %d) = %t, want %t", tt.score, tt.price, got, tt.want)
		}
	}
}

func TestTradingIdentityInitialCreditScores(t *testing.T) {
	chaincode, err := contractapi.NewChaincode(&TransactionIdentityRegistryContract{})
	if err != nil {
		t.Fatalf("NewChaincode() error = %v", err)
	}

	stub := shimtest.NewMockStub("transaction", chaincode)
	stub.Creator = transactionServiceCreator(t)

	userDID := "did:nycu-g39:identity:initial-credit"
	response := stub.MockInvoke("register-trading-identity", byteArgs(
		"RegisterTradingIdentity",
		userDID,
	))
	if response.Status != 200 {
		t.Fatalf("RegisterTradingIdentity failed: %s", response.Message)
	}

	var record TradingIdentity
	readMockJSON(t, stub, tradingIdentityKey(userDID), &record)
	if record.BuyerCreditScore != 60 || record.SellerCreditScore != 60 {
		t.Fatalf("initial credit scores = buyer %d, seller %d", record.BuyerCreditScore, record.SellerCreditScore)
	}
}

func transactionServiceCreator(t *testing.T) []byte {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "transactionUser"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{
			{
				Id:    asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 7, 8, 1},
				Value: []byte(`{"attrs":{"role":"transactionService"}}`),
			},
		},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	serializedIdentity, err := proto.Marshal(&msppb.SerializedIdentity{
		Mspid: "Org1MSP",
		IdBytes: pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certificateDER,
		}),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	return serializedIdentity
}

func byteArgs(values ...string) [][]byte {
	args := make([][]byte, 0, len(values))
	for _, value := range values {
		args = append(args, []byte(value))
	}

	return args
}

func putMockState(t *testing.T, stub *shimtest.MockStub, transactionID string, key string, value []byte) {
	t.Helper()

	stub.MockTransactionStart(transactionID)
	if err := stub.PutState(key, value); err != nil {
		stub.MockTransactionEnd(transactionID)
		t.Fatalf("PutState(%s) error = %v", key, err)
	}
	stub.MockTransactionEnd(transactionID)
}

func putMockJSON(t *testing.T, stub *shimtest.MockStub, transactionID string, key string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	putMockState(t, stub, transactionID, key, data)
}

func readMockJSON(t *testing.T, stub *shimtest.MockStub, key string, target any) {
	t.Helper()

	data := stub.State[key]
	if len(data) == 0 {
		t.Fatalf("state not found: %s", key)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", key, err)
	}
}

func readTransactionInfoState(t *testing.T, stub *shimtest.MockStub, transactionID uint) TransactionInfo {
	t.Helper()

	var info TransactionInfo
	readMockJSON(t, stub, transactionInfoKey(transactionID), &info)

	return info
}
