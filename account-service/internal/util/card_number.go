package util

import (
	"fmt"
	"math/rand"
)

// IIN prefixes per card type (Bank Identification Number)
var cardIIN = map[string]string{
	"visa":       "453201", // Visa IIN starting with 4
	"mastercard": "542523", // Mastercard IIN starting with 5
	"dinacard":   "655200", // Dinacard IIN
	"amex":       "371449", // Amex IIN starting with 37
}

// GenerateCardNumber generates a 16-digit card number with a valid Luhn check digit.
// Format: IIN (6) + Account Number (9) + Check Digit (1) = 16 digits
func GenerateCardNumber(vrstaKartice string) string {
	iin, ok := cardIIN[vrstaKartice]
	if !ok {
		iin = "453201" // default to Visa-style
	}
	// 9 random middle digits
	middle := fmt.Sprintf("%09d", rand.Int63n(1_000_000_000))
	// 15 digits without check digit
	partial := iin + middle
	check := luhnCheckDigit(partial)
	return partial + fmt.Sprintf("%d", check)
}

// ValidateLuhn returns true if the number passes the Luhn algorithm.
func ValidateLuhn(number string) bool {
	sum := 0
	nDigits := len(number)
	parity := nDigits % 2
	for i, ch := range number {
		if ch < '0' || ch > '9' {
			return false
		}
		digit := int(ch - '0')
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

// GenerateCVV generates a random 3-digit CVV string.
func GenerateCVV() string {
	return fmt.Sprintf("%03d", rand.Intn(1000))
}

// luhnCheckDigit computes the Luhn check digit for a partial number (without check digit).
func luhnCheckDigit(partial string) int {
	// Append a placeholder 0 and compute
	sum := 0
	n := len(partial) + 1
	parity := n % 2
	for i, ch := range partial {
		digit := int(ch - '0')
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	check := (10 - (sum % 10)) % 10
	return check
}
