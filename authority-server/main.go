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

type RegisterRequest struct {
	UserName     string `json:"userName"`
	IDCardNumber string `json:"idCardNumber"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	PublicKey    string `json:"publicKey"`
}

type RegisterResponse struct {
	Message string          `json:"message"`
	Request RegisterRequest `json:"request"`
	UserDID string          `json:"userDID,omitempty"`
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
	mux.HandleFunc("/register", registerHandler(fabricGateway))

	port := "8081"
	addr := ":" + port
	log.Printf("Authority Server PID: %d", os.Getpid())
	log.Printf("Authority Server IP: %s", localIPSummary())
	log.Printf("Authority Server listening on: %s", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("authority server failed: %v", err)
	}
}

func registerHandler(fabricGateway *FabricGateway) http.HandlerFunc {
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

		log.Printf("received register request: %+v", req)

		result, err := fabricGateway.Contract.SubmitTransaction("RegisterIdentity")
		if err != nil {
			log.Printf("failed to submit RegisterIdentity transaction: %v", err)
			http.Error(w, fmt.Sprintf("failed to register identity: %v", err), http.StatusInternalServerError)
			return
		}

		userDID := string(result)
		log.Printf("registered identity DID: %s", userDID)

		writeJSON(w, http.StatusOK, RegisterResponse{
			Message: "identity registered",
			Request: req,
			UserDID: userDID,
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
