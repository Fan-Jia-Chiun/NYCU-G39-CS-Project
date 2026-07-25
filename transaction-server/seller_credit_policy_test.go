package main

import "testing"

func TestSellerCreditStandingBoundaries(t *testing.T) {
	tests := []struct {
		score        uint
		grade        string
		canLaunch    bool
		unlimited    bool
		maximumPrice uint
	}{
		{score: 59, grade: "E", canLaunch: false},
		{score: 60, grade: "D", canLaunch: true, maximumPrice: 100_000},
		{score: 69, grade: "D", canLaunch: true, maximumPrice: 100_000},
		{score: 70, grade: "C", canLaunch: true, maximumPrice: 300_000},
		{score: 79, grade: "C", canLaunch: true, maximumPrice: 300_000},
		{score: 80, grade: "B", canLaunch: true, maximumPrice: 1_000_000},
		{score: 89, grade: "B", canLaunch: true, maximumPrice: 1_000_000},
		{score: 90, grade: "A", canLaunch: true, unlimited: true},
		{score: 100, grade: "A", canLaunch: true, unlimited: true},
	}

	for _, tt := range tests {
		got := sellerCreditStanding(tt.score)
		if got.Grade != tt.grade ||
			got.CanLaunch != tt.canLaunch ||
			got.Unlimited != tt.unlimited ||
			got.MaximumPrice != tt.maximumPrice {
			t.Fatalf("sellerCreditStanding(%d) = %+v", tt.score, got)
		}
	}
}

func TestSellerPriceQualificationAtAndAboveLimits(t *testing.T) {
	tests := []struct {
		score uint
		price uint
		mode  uint
		want  bool
	}{
		{score: 59, price: 1, mode: transactionModeFixedPrice, want: false},
		{score: 60, price: 100_000, mode: transactionModeFixedPrice, want: true},
		{score: 60, price: 100_001, mode: transactionModeFixedPrice, want: false},
		{score: 70, price: 300_000, mode: transactionModeBidding, want: true},
		{score: 70, price: 300_001, mode: transactionModeBidding, want: false},
		{score: 80, price: 1_000_000, mode: transactionModeSealedBid, want: true},
		{score: 80, price: 1_000_001, mode: transactionModeSealedBid, want: false},
		{score: 90, price: 100_000_000, mode: transactionModeFixedPrice, want: true},
	}

	for _, tt := range tests {
		got := evaluateSellerPriceQualification(tt.score, tt.price, tt.mode)
		if got.Eligible != tt.want {
			t.Fatalf("qualification for score %d price %d mode %d = %+v", tt.score, tt.price, tt.mode, got)
		}
	}
}
