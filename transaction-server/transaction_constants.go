package main

const (
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

	transactionRoleSeller uint = 2
)
