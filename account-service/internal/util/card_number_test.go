package util_test

import (
	"strings"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/account-service/internal/util"
)

// --- ValidateLuhn ---

func TestValidateLuhn_KnownValid(t *testing.T) {
	// Standard test vectors for Luhn algorithm
	valid := []string{
		"4532015112830366", // Visa test number
		"5425233430109903", // Mastercard test number
		"4111111111111111", // Classic Visa test
		"79927398713",      // IETF example
	}
	for _, n := range valid {
		if !util.ValidateLuhn(n) {
			t.Errorf("expected %s to pass Luhn check", n)
		}
	}
}

func TestValidateLuhn_KnownInvalid(t *testing.T) {
	invalid := []string{
		"4532015112830367", // off by 1
		"1234567890123456",
	}
	for _, n := range invalid {
		if util.ValidateLuhn(n) {
			t.Errorf("expected %s to fail Luhn check", n)
		}
	}
}

func TestValidateLuhn_NonDigits_ReturnsFalse(t *testing.T) {
	if util.ValidateLuhn("4532-0151-1283-0366") {
		t.Error("expected false for non-digit characters")
	}
}

// --- GenerateCardNumber ---

func TestGenerateCardNumber_Is16Digits(t *testing.T) {
	n := util.GenerateCardNumber("visa")
	if len(n) != 16 {
		t.Errorf("expected 16-digit card number, got %d: %s", len(n), n)
	}
	for _, c := range n {
		if c < '0' || c > '9' {
			t.Errorf("expected all digits, got non-digit in %s", n)
		}
	}
}

func TestGenerateCardNumber_PassesLuhn(t *testing.T) {
	for _, vrsta := range []string{"visa", "mastercard", "dinacard", "amex"} {
		n := util.GenerateCardNumber(vrsta)
		if !util.ValidateLuhn(n) {
			t.Errorf("GenerateCardNumber(%s) = %s, failed Luhn check", vrsta, n)
		}
	}
}

func TestGenerateCardNumber_VisaStartsWith4(t *testing.T) {
	n := util.GenerateCardNumber("visa")
	if !strings.HasPrefix(n, "4") {
		t.Errorf("visa card should start with 4, got %s", n)
	}
}

func TestGenerateCardNumber_MastercardStartsWith5(t *testing.T) {
	n := util.GenerateCardNumber("mastercard")
	if !strings.HasPrefix(n, "5") {
		t.Errorf("mastercard should start with 5, got %s", n)
	}
}

func TestGenerateCardNumber_IsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		n := util.GenerateCardNumber("visa")
		if seen[n] {
			t.Errorf("duplicate card number generated: %s", n)
		}
		seen[n] = true
	}
}

// --- GenerateCVV ---

func TestGenerateCVV_Is3Digits(t *testing.T) {
	cvv := util.GenerateCVV()
	if len(cvv) != 3 {
		t.Errorf("expected 3-digit CVV, got %q", cvv)
	}
	for _, c := range cvv {
		if c < '0' || c > '9' {
			t.Errorf("expected all digits in CVV, got %q", cvv)
		}
	}
}

func TestGenerateCVV_IsRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		seen[util.GenerateCVV()] = true
	}
	if len(seen) < 5 {
		t.Error("GenerateCVV appears non-random — too few unique values in 50 calls")
	}
}
