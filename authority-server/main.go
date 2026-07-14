package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type RegisterRequest struct {
	UserName     string `json:"userName"`
	IDCardNumber string `json:"idCardNumber"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	PublicKey    string `json:"publicKey"`
}

type RegisterResponse struct {
	Message   string          `json:"message"`
	Request   RegisterRequest `json:"request"`
	UserDID   string          `json:"userDID,omitempty"`
	PIMgrAddr string          `json:"pimgrAddr,omitempty"`
	BuyerDID  string          `json:"buyerDID,omitempty"`
	SellerDID string          `json:"sellerDID,omitempty"`
}

type TradingIdentityRegistrationRequest struct {
	IdentityDID string `json:"identityDID"`
	PublicKey   string `json:"publicKey"`
}

type TradingIdentityRegistrationResponse struct {
	Message     string `json:"message"`
	IdentityDID string `json:"identityDID"`
	BuyerDID    string `json:"buyerDID"`
	SellerDID   string `json:"sellerDID"`
}

func main() {
	fabricGateway, err := InitFabricGateway()
	if err != nil {
		log.Fatalf("failed to initialize Fabric Gateway: %v", err)
	}
	defer fabricGateway.Close()

	log.Println("Connected to Fabric Gateway.")
	log.Printf("Connected to channel: %s", fabricGateway.ChannelName)
	log.Printf("Loaded contract: %s", fabricGateway.ChaincodeName)

	transactionServerURL := envOrDefault("TRANSACTION_SERVER_URL", "http://localhost:8082/trading-identities")

	mux := http.NewServeMux()
	mux.HandleFunc("/register", registerHandler(fabricGateway, transactionServerURL))

	port := "8081"
	addr := ":" + port
	log.Printf("Authority Server PID: %d", os.Getpid())
	log.Printf("Authority Server IP: %s", localIPSummary())
	log.Printf("Transaction Server endpoint: %s", transactionServerURL)
	log.Printf("Authority Server listening on: %s", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("authority server failed: %v", err)
	}
}

func registerHandler(fabricGateway *FabricGateway, transactionServerURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()

		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		normalizeRegisterRequest(&req)
		if err := validateRegisterRequest(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("received register request: %+v", req)

		// Ask IDMgr to generate a novel identity DID and PIMgr for the user.
		result, err := fabricGateway.Contract.SubmitTransaction("RegisterIdentity")
		if err != nil {
			log.Printf("failed to submit RegisterIdentity transaction: %v", err)
			http.Error(w, fmt.Sprintf("failed to register identity: %v", err), http.StatusInternalServerError)
			return
		}

		userDID := string(result)
		log.Printf("registered identity DID: %s", userDID)

		// Get the PIMgr address to set the user's information.
		result, err = fabricGateway.Contract.EvaluateTransaction("GetPIMgr", userDID)
		if err != nil {
			log.Printf("failed to evaluate GetPIMgr transaction: %v", err)
			http.Error(w, fmt.Sprintf("failed to get PIMgr address: %v", err), http.StatusInternalServerError)
			return
		}

		pimgrAddr := string(result)
		if pimgrAddr == "" {
			log.Printf("empty PIMgr address for DID: %s", userDID)
			http.Error(w, "failed to get PIMgr address: empty result", http.StatusInternalServerError)
			return
		}
		log.Printf("loaded PIMgr address: %s", pimgrAddr)

		// Set the user's information into the PIMgr.
		if _, err := fabricGateway.Contract.SubmitTransaction(
			"SetProfile",
			pimgrAddr,
			req.UserName,
			req.IDCardNumber,
			req.Email,
			req.Phone,
		); err != nil {
			log.Printf("failed to submit SetProfile transaction: %v", err)
			http.Error(w, fmt.Sprintf("failed to set profile: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("profile set for DID: %s", userDID)

		// Set the user's public key into the PIMgr.
		if _, err := fabricGateway.Contract.SubmitTransaction("SetPublicKey", pimgrAddr, req.PublicKey); err != nil {
			log.Printf("failed to submit SetPublicKey transaction: %v", err)
			http.Error(w, fmt.Sprintf("failed to set public key: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("public key set for DID: %s", userDID)

		tradingIdentity, err := registerTradingIdentity(transactionServerURL, userDID, req.PublicKey)
		if err != nil {
			log.Printf("failed to register trading identity: %v", err)
			http.Error(w, fmt.Sprintf("failed to register trading identity: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("trading identity registered for DID %s: buyer=%s seller=%s", userDID, tradingIdentity.BuyerDID, tradingIdentity.SellerDID)

		writeJSON(w, http.StatusOK, RegisterResponse{
			Message:   "identity registered",
			Request:   req,
			UserDID:   userDID,
			PIMgrAddr: pimgrAddr,
			BuyerDID:  tradingIdentity.BuyerDID,
			SellerDID: tradingIdentity.SellerDID,
		})
	}
}

func registerTradingIdentity(transactionServerURL string, identityDID string, publicKey string) (*TradingIdentityRegistrationResponse, error) {
	body, err := json.Marshal(TradingIdentityRegistrationRequest{
		IdentityDID: identityDID,
		PublicKey:   publicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode trading identity request: %w", err)
	}

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(transactionServerURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to call transaction server: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transaction server response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("transaction server returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var result TradingIdentityRegistrationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode transaction server response: %w", err)
	}
	if result.BuyerDID == "" || result.SellerDID == "" {
		return nil, fmt.Errorf("transaction server returned empty buyerDID or sellerDID")
	}

	return &result, nil
}

func normalizeRegisterRequest(req *RegisterRequest) {
	req.UserName = strings.TrimSpace(req.UserName)
	req.IDCardNumber = strings.TrimSpace(req.IDCardNumber)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.PublicKey = strings.TrimSpace(req.PublicKey)
}

func validateRegisterRequest(req RegisterRequest) error {
	if req.UserName == "" {
		return fmt.Errorf("userName is required")
	}
	if req.IDCardNumber == "" {
		return fmt.Errorf("idCardNumber is required")
	}
	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if req.Phone == "" {
		return fmt.Errorf("phone is required")
	}
	if req.PublicKey == "" {
		return fmt.Errorf("publicKey is required")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func localIPSummary() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}

	var ips []string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil {
			continue
		}
		ips = append(ips, ip.String())
	}

	if len(ips) == 0 {
		return "127.0.0.1"
	}

	return strings.Join(ips, ", ")
}
