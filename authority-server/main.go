package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", registerHandler)

	port := "8081"
	addr := ":" + port
	log.Printf("Authority Server PID: %d", os.Getpid())
	log.Printf("Authority Server listening on: %s", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("authority server failed: %v", err)
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, RegisterResponse{
		Message: "register request received",
		Request: req,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
