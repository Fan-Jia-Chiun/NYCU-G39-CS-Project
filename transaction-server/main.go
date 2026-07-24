package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

type RegisterTradingIdentityRequest struct {
	UserDID   string `json:"userDID"`
	PublicKey string `json:"publicKey"`
}

type RegisterTradingIdentityResponse struct {
	Message   string `json:"message"`
	UserDID   string `json:"userDID"`
	BuyerDID  string `json:"buyerDID"`
	SellerDID string `json:"sellerDID"`
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

	ipfsEndpoint := envOrDefault("IPFS_API_ENDPOINT", defaultIPFSEndpoint)
	ipfs := newIPFSClient(ipfsEndpoint)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", runningHandler)
	mux.HandleFunc("/transaction", transactionHandler)
	mux.HandleFunc("/trading-identities", registerTradingIdentityHandler(fabricGateway))
	mux.HandleFunc("/login", loginHandler(fabricGateway, ipfs))
	mux.HandleFunc("/assets", assetRegistrationHandler(fabricGateway, ipfs))
	mux.HandleFunc("/transactions/launch", transactionLaunchHandler(fabricGateway))
	mux.Handle("/", noCacheFileServer(staticWebDir()))

	port := "8082"
	addr := ":" + port
	log.Printf("Transaction Server PID: %d", os.Getpid())
	log.Printf("Transaction Server IP: %s", localIPSummary())
	log.Printf("IPFS API endpoint: %s", ipfsEndpoint)
	log.Printf("Transaction Server listening on: %s", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("transaction server failed: %v", err)
	}
}

func runningHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Transaction Server Running\n")); err != nil {
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

func transactionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Transaction Server Running\n")); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func registerTradingIdentityHandler(fabricGateway *FabricGateway) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()

		var req RegisterTradingIdentityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		req.UserDID = strings.TrimSpace(req.UserDID)
		req.PublicKey = strings.TrimSpace(req.PublicKey)
		if req.UserDID == "" {
			http.Error(w, "userDID is required", http.StatusBadRequest)
			return
		}
		if req.PublicKey == "" {
			http.Error(w, "publicKey is required", http.StatusBadRequest)
			return
		}

		log.Printf("received trading identity registration request: %+v", req)

		result, err := fabricGateway.Contract.SubmitTransaction("RegisterTradingIdentity", req.UserDID)
		if err != nil {
			log.Printf("failed to submit RegisterTradingIdentity transaction: %v", err)
			http.Error(w, fmt.Sprintf("failed to register trading identity: %v", err), http.StatusInternalServerError)
			return
		}

		var ccResult struct {
			BuyerDID  string `json:"buyerDID"`
			SellerDID string `json:"sellerDID"`
		}
		if err := json.Unmarshal(result, &ccResult); err != nil {
			log.Printf("failed to decode RegisterTradingIdentity result: %v", err)
			http.Error(w, fmt.Sprintf("failed to decode trading identity result: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("registered trading identity for DID %s: buyer=%s seller=%s", req.UserDID, ccResult.BuyerDID, ccResult.SellerDID)

		if _, err := fabricGateway.Contract.SubmitTransaction("SetPublicKey", req.UserDID, req.PublicKey); err != nil {
			log.Printf("failed to submit SetPublicKey transaction: %v", err)
			http.Error(w, fmt.Sprintf("failed to set trading user public key: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("public key set on transaction chain for DID: %s", req.UserDID)

		writeJSON(w, http.StatusOK, RegisterTradingIdentityResponse{
			Message:   "trading identity registered",
			UserDID:   req.UserDID,
			BuyerDID:  ccResult.BuyerDID,
			SellerDID: ccResult.SellerDID,
		})
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
