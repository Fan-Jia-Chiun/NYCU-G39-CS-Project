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
	IdentityDID string `json:"identityDID"`
}

type RegisterTradingIdentityResponse struct {
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

	mux := http.NewServeMux()
	mux.HandleFunc("/health", runningHandler)
	mux.HandleFunc("/transaction", transactionHandler)
	mux.HandleFunc("/trading-identities", registerTradingIdentityHandler(fabricGateway))

	port := "8082"
	addr := ":" + port
	log.Printf("Transaction Server PID: %d", os.Getpid())
	log.Printf("Transaction Server IP: %s", localIPSummary())
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

		req.IdentityDID = strings.TrimSpace(req.IdentityDID)
		if req.IdentityDID == "" {
			http.Error(w, "identityDID is required", http.StatusBadRequest)
			return
		}

		log.Printf("received trading identity registration request: %+v", req)

		result, err := fabricGateway.Contract.SubmitTransaction("RegisterTradingIdentity", req.IdentityDID)
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

		log.Printf("registered trading identity for DID %s: buyer=%s seller=%s", req.IdentityDID, ccResult.BuyerDID, ccResult.SellerDID)

		writeJSON(w, http.StatusOK, RegisterTradingIdentityResponse{
			Message:     "trading identity registered",
			IdentityDID: req.IdentityDID,
			BuyerDID:    ccResult.BuyerDID,
			SellerDID:   ccResult.SellerDID,
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
