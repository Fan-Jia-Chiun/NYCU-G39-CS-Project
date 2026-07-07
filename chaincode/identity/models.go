package main

const (
	objectTypePIMgr = "pimgr"

	didStatusActive   = "ACTIVE"
	didStatusDisabled = "DISABLED"
	didStatusRevoked  = "REVOKED"

	roleAuthority          = "authority"
	roleTransactionService = "transactionService"
	roleVerifier           = "verifier"
)

type PIMgr struct {
	ObjectType string       `json:"objectType"`
	UserDID    string       `json:"userDID"`
	PIMgrAddr  string       `json:"pimgrAddr"`
	Status     string       `json:"status"`
	Profile    *UserProfile `json:"profile,omitempty"`
	PublicKey  string       `json:"publicKey,omitempty"`
	CreatedAt  string       `json:"createdAt"`
	UpdatedAt  string       `json:"updatedAt"`
}

type UserProfile struct {
	UserName     string `json:"userName"`
	IDCardNumber string `json:"idCardNumber"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
}
