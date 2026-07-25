package main

func sellerPriceEligible(sellerCreditScore uint, targetPrice uint) bool {
	switch {
	case sellerCreditScore > 100:
		return false
	case sellerCreditScore >= sellerCreditGradeAMinimumScore:
		return true
	case sellerCreditScore >= sellerCreditGradeBMinimumScore:
		return targetPrice <= sellerCreditGradeBMaximumPrice
	case sellerCreditScore >= sellerCreditGradeCMinimumScore:
		return targetPrice <= sellerCreditGradeCMaximumPrice
	case sellerCreditScore >= sellerCreditGradeDMinimumScore:
		return targetPrice <= sellerCreditGradeDMaximumPrice
	default:
		return false
	}
}
