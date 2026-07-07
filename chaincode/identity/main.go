package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	chaincode, err := contractapi.NewChaincode(&IdentityContract{})
	if err != nil {
		log.Panicf("failed to create identity chaincode: %v", err)
	}

	if err := chaincode.Start(); err != nil {
		log.Panicf("failed to start identity chaincode: %v", err)
	}
}
