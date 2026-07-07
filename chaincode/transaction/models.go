package main

const (
	objectTypeTradingIdentity = "tradingIdentity"

	tradingIdentityStatusActive = "ACTIVE"

	roleTransactionService = "transactionService"
	roleVerifier           = "verifier"
)

type TradingIdentity struct {
	ObjectType  string `json:"objectType"`
	IdentityDID string `json:"identityDID"`
	BuyerDID    string `json:"buyerDID"`
	SellerDID   string `json:"sellerDID"`
	PublicKey   string `json:"publicKey,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type TradingIdentityResult struct {
	BuyerDID  string `json:"buyerDID"`
	SellerDID string `json:"sellerDID"`
}
