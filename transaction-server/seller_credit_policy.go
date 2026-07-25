package main

import "fmt"

const (
	sellerCreditGradeAMinimumScore uint = 90
	sellerCreditGradeBMinimumScore uint = 80
	sellerCreditGradeCMinimumScore uint = 70
	sellerCreditGradeDMinimumScore uint = 60

	sellerCreditGradeBMaximumPrice uint = 1_000_000
	sellerCreditGradeCMaximumPrice uint = 300_000
	sellerCreditGradeDMaximumPrice uint = 100_000
)

type SellerCreditRule struct {
	Grade        string `json:"grade"`
	MinimumScore uint   `json:"minimumScore"`
	MaximumScore uint   `json:"maximumScore"`
	CanLaunch    bool   `json:"canLaunch"`
	Unlimited    bool   `json:"unlimited"`
	MaximumPrice uint   `json:"maximumPrice,omitempty"`
}

type SellerCreditStanding struct {
	Score        uint   `json:"score"`
	Grade        string `json:"grade"`
	CanLaunch    bool   `json:"canLaunch"`
	Unlimited    bool   `json:"unlimited"`
	MaximumPrice uint   `json:"maximumPrice,omitempty"`
	Message      string `json:"message"`
}

type SellerCreditPolicy struct {
	Currency string               `json:"currency"`
	Current  SellerCreditStanding `json:"current"`
	Rules    []SellerCreditRule   `json:"rules"`
}

type SellerPriceQualification struct {
	Eligible     bool   `json:"eligible"`
	Price        uint   `json:"price"`
	PriceField   string `json:"priceField"`
	Grade        string `json:"grade"`
	Unlimited    bool   `json:"unlimited"`
	MaximumPrice uint   `json:"maximumPrice,omitempty"`
	Message      string `json:"message"`
}

func sellerCreditPolicy(score uint) SellerCreditPolicy {
	return SellerCreditPolicy{
		Currency: "TWD",
		Current:  sellerCreditStanding(score),
		Rules: []SellerCreditRule{
			{Grade: "A", MinimumScore: 90, MaximumScore: 100, CanLaunch: true, Unlimited: true},
			{Grade: "B", MinimumScore: 80, MaximumScore: 89, CanLaunch: true, MaximumPrice: sellerCreditGradeBMaximumPrice},
			{Grade: "C", MinimumScore: 70, MaximumScore: 79, CanLaunch: true, MaximumPrice: sellerCreditGradeCMaximumPrice},
			{Grade: "D", MinimumScore: 60, MaximumScore: 69, CanLaunch: true, MaximumPrice: sellerCreditGradeDMaximumPrice},
			{Grade: "E", MinimumScore: 0, MaximumScore: 59, CanLaunch: false},
		},
	}
}

func sellerCreditStanding(score uint) SellerCreditStanding {
	switch {
	case score > 100:
		return SellerCreditStanding{
			Score:   score,
			Grade:   "Invalid",
			Message: "seller credit score is outside the supported range",
		}
	case score >= sellerCreditGradeAMinimumScore:
		return SellerCreditStanding{
			Score:     score,
			Grade:     "A",
			CanLaunch: true,
			Unlimited: true,
			Message:   "Grade A has no transaction launch price limit.",
		}
	case score >= sellerCreditGradeBMinimumScore:
		return limitedSellerCreditStanding(score, "B", sellerCreditGradeBMaximumPrice)
	case score >= sellerCreditGradeCMinimumScore:
		return limitedSellerCreditStanding(score, "C", sellerCreditGradeCMaximumPrice)
	case score >= sellerCreditGradeDMinimumScore:
		return limitedSellerCreditStanding(score, "D", sellerCreditGradeDMaximumPrice)
	default:
		return SellerCreditStanding{
			Score:   score,
			Grade:   "E",
			Message: "The current seller credit score is below the transaction launch threshold.",
		}
	}
}

func limitedSellerCreditStanding(score uint, grade string, maximumPrice uint) SellerCreditStanding {
	return SellerCreditStanding{
		Score:        score,
		Grade:        grade,
		CanLaunch:    true,
		MaximumPrice: maximumPrice,
		Message: fmt.Sprintf(
			"Grade %s may launch transactions priced up to TWD %d.",
			grade,
			maximumPrice,
		),
	}
}

func evaluateSellerPriceQualification(score uint, price uint, transactionMode uint) SellerPriceQualification {
	standing := sellerCreditStanding(score)
	priceField := "sale price"
	if transactionMode != transactionModeFixedPrice {
		priceField = "starting price"
	}

	qualification := SellerPriceQualification{
		Price:        price,
		PriceField:   priceField,
		Grade:        standing.Grade,
		Unlimited:    standing.Unlimited,
		MaximumPrice: standing.MaximumPrice,
	}
	switch {
	case !standing.CanLaunch:
		qualification.Message = standing.Message
	case price == 0:
		qualification.Message = priceField + " must be greater than zero"
	case !standing.Unlimited && price > standing.MaximumPrice:
		qualification.Message = fmt.Sprintf(
			"%s exceeds the maximum price allowed for seller credit grade %s",
			priceField,
			standing.Grade,
		)
	default:
		qualification.Eligible = true
		qualification.Message = fmt.Sprintf(
			"%s is within the limit for seller credit grade %s",
			priceField,
			standing.Grade,
		)
	}

	return qualification
}
