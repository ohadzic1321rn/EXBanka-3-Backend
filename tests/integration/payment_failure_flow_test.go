//go:build integration

package integration_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestPaymentFlow_NonexistentReceiverAccountRejected(t *testing.T) {
	adminToken := adminLogin(t)
	client := createClientViaAdmin(t, adminToken, "integration.payment")
	activateClientViaToken(t, client.SetupToken, "ClientPass12")
	clientToken := loginClientToken(t, client.Email, "ClientPass12")
	account := createCheckingAccountViaAdmin(t, adminToken, client.ID, "Payment Sender", 5000)

	resp, body := postJSONWithToken(t, "/payments", clientToken, map[string]interface{}{
		"racunPosiljaocaId": toNumber(account.ID),
		"racunPrimaocaBroj": "333999888777666555",
		"iznos":             250,
		"sifraPlacanja":     "289",
		"pozivNaBroj":       "PAY-NOT-FOUND",
		"svrha":             "payment-missing-receiver",
	})
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("payment with nonexistent receiver should fail, got 200: %v", body)
	}

	message := strings.ToLower(toStringValue(body["message"]))
	errorValue := strings.ToLower(toStringValue(body["error"]))
	if message == "" && errorValue == "" {
		return
	}
	if !strings.Contains(message, "receiver") && !strings.Contains(errorValue, "receiver") {
		t.Fatalf("expected receiver-account error, got body: %v", body)
	}
}

func toStringValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
