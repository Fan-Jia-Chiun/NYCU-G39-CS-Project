package main

const (
	objectTypeTradingIdentity = "tradingIdentity"
	objectTypeUserAssetList   = "userAssetList"
	objectTypeAssetCert       = "assetCertificate"
	objectTypePropertyIndex   = "propertyIndex"
	objectTypeTransactionInfo = "transactionInfo"
	objectTypeUserTradeList   = "userTransactionList"
	transactionCounterKey     = "TRANSACTION_COUNTER"

	tradingIdentityStatusActive = "ACTIVE"

	roleTransactionService = "transactionService"
	roleVerifier           = "verifier"

	accountStatusAvailable   uint = 0
	defaultBuyerCreditScore  uint = 80
	defaultSellerCreditScore uint = 80

	legalStatusNormal             int = 0
	legalStatusPending            int = 1
	legalStatusSelling            int = 2
	legalStatusBidding            int = 3
	legalStatusWinnerConfirmation int = 4
	legalStatusTransiting         int = 5
	legalStatusRestricted         int = 6

	transactionStatusReviewing          uint = 0
	transactionStatusInProgress         uint = 1
	transactionStatusWinnerConfirmation uint = 2
	transactionStatusPendingShipment    uint = 3
	transactionStatusTransporting       uint = 4
	transactionStatusCompleted          uint = 5
	transactionStatusCancelled          uint = 6
	transactionStatusReturnRequest      uint = 7
	transactionStatusReturning          uint = 8
	transactionStatusReturned           uint = 9
	transactionStatusRejected           uint = 10

	transactionModeFixedPrice uint = 0
	transactionModeBidding    uint = 1
	transactionModeSealedBid  uint = 2

	transactionRoleBuyer  uint = 0
	transactionRoleBidder uint = 1
	transactionRoleSeller uint = 2
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

type TimeInfo struct {
	Year   uint `json:"year"`
	Month  uint `json:"month"`
	Day    uint `json:"day"`
	Hour   uint `json:"hour"`
	Minute uint `json:"minute"`
	Second uint `json:"second"`
}

type TransactionInfo struct {
	ObjectType          string   `json:"objectType,omitempty"`
	TransactionID       uint     `json:"transactionID,omitempty"`
	AssetID             string   `json:"assetID"`
	SellerDID           string   `json:"sellerDID"`
	TransactionStatus   uint     `json:"transactionStatus"`
	TransactionMode     uint     `json:"transactionMode"`
	StartTime           TimeInfo `json:"startTime"`
	FixedPrice          uint     `json:"fixedPrice"`
	BasicPrice          uint     `json:"basicPrice"`
	CurrentHighestBid   uint     `json:"currentHighestBid"`
	FinalizingTime      TimeInfo `json:"finalizingTime"`
	LogisticsRecordAddr string   `json:"logisticsRecordAddr,omitempty"`
}

type UserTransactionList struct {
	ObjectType          string   `json:"objectType"`
	UserDID             string   `json:"userDID"`
	TransactionIDList   []uint   `json:"transactionIDList"`
	AssetIDList         []string `json:"assetIDList,omitempty"`
	TransactionRoleList []uint   `json:"transactionRoleList"`
	IsActiveList        []bool   `json:"isActiveList"`
}

type TradeListResult struct {
	TransactionIDList   []uint   `json:"transactionIDList"`
	AssetIDList         []string `json:"assetIDList"`
	TransactionRoleList []uint   `json:"transactionRoleList"`
	IsActiveList        []bool   `json:"isActiveList"`
}
