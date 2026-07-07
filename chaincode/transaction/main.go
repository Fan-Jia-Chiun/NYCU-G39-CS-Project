package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	chaincode, err := contractapi.NewChaincode(&TransactionIdentityRegistryContract{})
	if err != nil {
		log.Panicf("failed to create transaction identity registry chaincode: %v", err)
	}

	if err := chaincode.Start(); err != nil {
		log.Panicf("failed to start transaction identity registry chaincode: %v", err)
	}
}
