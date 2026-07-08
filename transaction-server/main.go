package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", runningHandler)
	mux.HandleFunc("/transaction", transactionHandler)

	port := "8082"
	addr := ":" + port
	log.Printf("Transaction Server PID: %d", os.Getpid())
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
