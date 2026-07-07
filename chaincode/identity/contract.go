package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type IdentityContract struct {
	contractapi.Contract
}

// RegisterIdentity creates a user DID and binds it to a PIMgr record.
// Input: none. Output: generated identity DID.
func (c *IdentityContract) RegisterIdentity(ctx contractapi.TransactionContextInterface) (string, error) {
	if err := requireAnyRole(ctx, roleAuthority); err != nil {
		return "", err
	}

	userDID := "did:nycu-g39:identity:" + ctx.GetStub().GetTxID()
	pimgrAddr := pimgrKey(userDID)

	existing, err := ctx.GetStub().GetState(pimgrAddr)
	if err != nil {
		return "", fmt.Errorf("failed to check PIMgr state: %w", err)
	}
	if existing != nil {
		return "", fmt.Errorf("identity already exists: %s", userDID)
	}

	now, err := txTimestamp(ctx)
	if err != nil {
		return "", err
	}

	pimgr := PIMgr{
		ObjectType: objectTypePIMgr,
		UserDID:    userDID,
		PIMgrAddr:  pimgrAddr,
		Status:     didStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := putPIMgr(ctx, &pimgr); err != nil {
		return "", err
	}

	return userDID, nil
}

// GetPIMgr returns the PIMgr ledger address for a target user DID.
// Input: userDID. Output: PIMgrAddr, or empty string when not found.
func (c *IdentityContract) GetPIMgr(ctx contractapi.TransactionContextInterface, userDID string) (string, error) {
	if err := requireAnyRole(ctx, roleAuthority, roleTransactionService, roleVerifier); err != nil {
		return "", err
	}
	if err := validateDID(userDID); err != nil {
		return "", err
	}

	pimgrAddr := pimgrKey(userDID)
	existing, err := ctx.GetStub().GetState(pimgrAddr)
	if err != nil {
		return "", fmt.Errorf("failed to read PIMgr state: %w", err)
	}
	if existing == nil {
		return "", nil
	}

	return pimgrAddr, nil
}

// GetPublicKey returns the public key stored in the user's PIMgr.
// Input: userDID. Output: public key.
func (c *IdentityContract) GetPublicKey(ctx contractapi.TransactionContextInterface, userDID string) (string, error) {
	if err := requireAnyRole(ctx, roleAuthority, roleTransactionService, roleVerifier); err != nil {
		return "", err
	}

	pimgr, err := getPIMgr(ctx, userDID)
	if err != nil {
		return "", err
	}
	if pimgr.PublicKey == "" {
		return "", fmt.Errorf("public key is not set for DID: %s", userDID)
	}

	return pimgr.PublicKey, nil
}

// SetProfile stores the verified user profile in the target PIMgr.
// Input: PIMgrAddr, user name, ID card number, email, phone. Output: success.
func (c *IdentityContract) SetProfile(ctx contractapi.TransactionContextInterface, pimgrAddr string, userName string, idCardNumber string, email string, phone string) (bool, error) {
	if err := requireAnyRole(ctx, roleAuthority); err != nil {
		return false, err
	}
	if err := validateRequired("userName", userName); err != nil {
		return false, err
	}
	if err := validateRequired("idCardNumber", idCardNumber); err != nil {
		return false, err
	}
	if err := validateRequired("email", email); err != nil {
		return false, err
	}
	if err := validateRequired("phone", phone); err != nil {
		return false, err
	}

	pimgr, err := getPIMgrByAddr(ctx, pimgrAddr)
	if err != nil {
		return false, err
	}

	now, err := txTimestamp(ctx)
	if err != nil {
		return false, err
	}

	pimgr.Profile = &UserProfile{
		UserName:     strings.TrimSpace(userName),
		IDCardNumber: strings.TrimSpace(idCardNumber),
		Email:        strings.TrimSpace(email),
		Phone:        strings.TrimSpace(phone),
	}
	pimgr.UpdatedAt = now

	if err := putPIMgr(ctx, pimgr); err != nil {
		return false, err
	}

	return true, nil
}

// SetPublicKey stores the user's public key in the target PIMgr.
// Input: PIMgrAddr, public key. Output: success.
func (c *IdentityContract) SetPublicKey(ctx contractapi.TransactionContextInterface, pimgrAddr string, publicKey string) (bool, error) {
	if err := requireAnyRole(ctx, roleAuthority); err != nil {
		return false, err
	}
	if err := validateRequired("publicKey", publicKey); err != nil {
		return false, err
	}

	pimgr, err := getPIMgrByAddr(ctx, pimgrAddr)
	if err != nil {
		return false, err
	}

	now, err := txTimestamp(ctx)
	if err != nil {
		return false, err
	}

	pimgr.PublicKey = strings.TrimSpace(publicKey)
	pimgr.UpdatedAt = now

	if err := putPIMgr(ctx, pimgr); err != nil {
		return false, err
	}

	return true, nil
}

// ReadPIMgr returns the full PIMgr record for authorized inspection.
// Input: userDID. Output: PIMgr record.
func (c *IdentityContract) ReadPIMgr(ctx contractapi.TransactionContextInterface, userDID string) (*PIMgr, error) {
	if err := requireAnyRole(ctx, roleAuthority, roleVerifier); err != nil {
		return nil, err
	}

	return getPIMgr(ctx, userDID)
}

// pimgrKey builds the ledger key used to store a user's PIMgr record.
func pimgrKey(userDID string) string {
	return "PIMGR:" + userDID
}

// getPIMgr reads and decodes a PIMgr record by user DID.
func getPIMgr(ctx contractapi.TransactionContextInterface, userDID string) (*PIMgr, error) {
	if err := validateDID(userDID); err != nil {
		return nil, err
	}

	data, err := ctx.GetStub().GetState(pimgrKey(userDID))
	if err != nil {
		return nil, fmt.Errorf("failed to read PIMgr state: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("PIMgr not found for DID: %s", userDID)
	}

	var pimgr PIMgr
	if err := json.Unmarshal(data, &pimgr); err != nil {
		return nil, fmt.Errorf("failed to decode PIMgr state: %w", err)
	}

	return &pimgr, nil
}

// getPIMgrByAddr reads and decodes a PIMgr record by ledger address.
func getPIMgrByAddr(ctx contractapi.TransactionContextInterface, pimgrAddr string) (*PIMgr, error) {
	if err := validateRequired("pimgrAddr", pimgrAddr); err != nil {
		return nil, err
	}

	pimgrAddr = strings.TrimSpace(pimgrAddr)
	data, err := ctx.GetStub().GetState(pimgrAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to read PIMgr state: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("PIMgr not found at address: %s", pimgrAddr)
	}

	var pimgr PIMgr
	if err := json.Unmarshal(data, &pimgr); err != nil {
		return nil, fmt.Errorf("failed to decode PIMgr state: %w", err)
	}

	return &pimgr, nil
}

// putPIMgr encodes and writes a PIMgr record back to world state.
func putPIMgr(ctx contractapi.TransactionContextInterface, pimgr *PIMgr) error {
	data, err := json.Marshal(pimgr)
	if err != nil {
		return fmt.Errorf("failed to encode PIMgr state: %w", err)
	}

	if err := ctx.GetStub().PutState(pimgr.PIMgrAddr, data); err != nil {
		return fmt.Errorf("failed to write PIMgr state: %w", err)
	}

	return nil
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

// validateDID checks that the user DID argument is present.
func validateDID(userDID string) error {
	return validateRequired("userDID", userDID)
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
