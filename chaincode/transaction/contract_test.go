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
		"did:nycu-g39:seller:test",
	))
	if response.Status != 200 {
		t.Fatalf("AddNewTransaction failed: %s", response.Message)
	}

	transactionID := uint(1)
	info := readTransactionInfoState(t, stub, transactionID)
	if info.TransactionStatus != transactionStatusReviewing {
		t.Fatalf("transaction status = %d, want Reviewing", info.TransactionStatus)
	}
	if info.AssetID != assetID || info.SellerDID != "did:nycu-g39:seller:test" {
		t.Fatalf("transaction info = %+v", info)
	}

	response = stub.MockInvoke("index-transaction", byteArgs(
		"UpdateTransactionList",
		"did:nycu-g39:identity:test",
		"1",
		assetID,
		"2",
		"true",
	))
	if response.Status != 200 {
		t.Fatalf("UpdateTransactionList insert failed: %s", response.Message)
	}

	finalizingTime := TimeInfo{Year: 2099, Month: 7, Day: 24, Hour: 8, Minute: 30}
	finalizingTimeJSON, err := json.Marshal(finalizingTime)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
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
	))
	if response.Status == 200 {
		t.Fatalf("ChangeTransactionStatus must reject In Progress")
	}

	response = stub.MockInvoke("reject-transaction", byteArgs(
		"ChangeTransactionStatus",
		"1",
		"10",
	))
	if response.Status != 200 {
		t.Fatalf("ChangeTransactionStatus reject failed: %s", response.Message)
	}
	response = stub.MockInvoke("deactivate-transaction", byteArgs(
		"UpdateTransactionList",
		"did:nycu-g39:identity:test",
		"1",
		assetID,
		"2",
		"false",
	))
	if response.Status != 200 {
		t.Fatalf("UpdateTransactionList deactivate failed: %s", response.Message)
	}

	var list UserTransactionList
	readMockJSON(t, stub, userTransactionListKey("did:nycu-g39:identity:test"), &list)
	if len(list.TransactionIDList) != 1 || list.TransactionIDList[0] != transactionID {
		t.Fatalf("transaction history was not preserved: %+v", list)
	}
	if list.IsActiveList[0] {
		t.Fatalf("rejected transaction must be inactive: %+v", list)
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
