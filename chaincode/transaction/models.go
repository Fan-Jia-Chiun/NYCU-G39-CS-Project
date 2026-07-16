package main

const (
	objectTypeTradingIdentity = "tradingIdentity"
	objectTypeUserAssetList   = "userAssetList"
	objectTypeAssetCert       = "assetCertificate"
	objectTypePropertyIndex   = "propertyIndex"
	objectTypeTradeInfo       = "tradeInfo"
	objectTypeUserTradeList   = "userTransactionList"

	tradingIdentityStatusActive = "ACTIVE"

	roleTransactionService = "transactionService"
	roleVerifier           = "verifier"

	accountStatusAvailable   uint = 0
	defaultBuyerCreditScore  uint = 80
	defaultSellerCreditScore uint = 80
	legalStatusNormal        int  = 0
	tradeStatusCompleted     uint = 5
	tradeStatusCancelled     uint = 6
	tradeStatusReturned      uint = 9
	tradeStatusRejected      uint = 10
)

type TradingIdentity struct {
	ObjectType        string `json:"objectType"`
	UserDID           string `json:"userDID"`
	BuyerDID          string `json:"buyerDID"`
	SellerDID         string `json:"sellerDID"`
	PublicKey         string `json:"publicKey,omitempty"`
	Status            string `json:"status"`
	AccountStatus     uint   `json:"accountStatus"`
	BuyerCreditScore  uint   `json:"buyerCreditScore"`
	SellerCreditScore uint   `json:"sellerCreditScore"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type TradingIdentityResult struct {
	BuyerDID  string `json:"buyerDID"`
	SellerDID string `json:"sellerDID"`
}

type CreditScores struct {
	BuyerCreditScore  uint `json:"buyerCreditScore"`
	SellerCreditScore uint `json:"sellerCreditScore"`
}

type UserAssetList struct {
	ObjectType string   `json:"objectType"`
	UserDID    string   `json:"userDID"`
	AssetAddrs []string `json:"assetAddrs"`
}

type AssetCertificate struct {
	ObjectType   string `json:"objectType"`
	AssetID      string `json:"assetID"`
	AssetInfoCID string `json:"assetInfoCID,omitempty"`
	LegalStatus  int    `json:"legalStatus"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type PropertyIndex struct {
	ObjectType string   `json:"objectType"`
	AssetID    string   `json:"assetID"`
	OwnerDID   string   `json:"ownerDID"`
	ChangeLog  []string `json:"changeLog"`
}

type TradeInfo struct {
	ObjectType          string `json:"objectType,omitempty"`
	TradeID             uint   `json:"tradeID,omitempty"`
	AssetID             string `json:"assetID"`
	TransactionStatus   uint   `json:"transactionStatus"`
	TransactionMode     uint   `json:"transactionMode"`
	CurrentHighestPrice uint   `json:"currentHighestPrice"`
}

type UserTransactionList struct {
	ObjectType          string   `json:"objectType"`
	UserDID             string   `json:"userDID"`
	TradeIDList         []uint   `json:"tradeIDList"`
	AssetIDList         []string `json:"assetIDList,omitempty"`
	TransactionRoleList []uint   `json:"transactionRoleList"`
	IsActiveList        []bool   `json:"isActiveList"`
}

type TradeListResult struct {
	TradeIDList         []uint `json:"tradeIDList"`
	TransactionRoleList []uint `json:"transactionRoleList"`
	IsActiveList        []bool `json:"isActiveList"`
}
