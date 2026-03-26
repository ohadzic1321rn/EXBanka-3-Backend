//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestTransferFlow_HistoryNewestFirst(t *testing.T) {
	adminToken := adminLogin(t)
	client := createClientViaAdmin(t, adminToken, "integration.transfer")
	activateClientViaToken(t, client.SetupToken, "ClientPass12")
	clientToken := loginClientToken(t, client.Email, "ClientPass12")

	fromAccount := createCheckingAccountViaAdmin(t, adminToken, client.ID, "Transfer From", 5000)
	toAccount := createCheckingAccountViaAdmin(t, adminToken, client.ID, "Transfer To", 100)

	firstTransferID := createAndConfirmTransfer(t, clientToken, fromAccount.ID, toAccount.ID, 100, "history-first")
	time.Sleep(1100 * time.Millisecond)
	secondTransferID := createAndConfirmTransfer(t, clientToken, fromAccount.ID, toAccount.ID, 50, "history-second")

	resp, body := getWithToken(t, "/transfers/client/"+client.ID, clientToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("transfer history expected 200, got %d: %v", resp.StatusCode, body)
	}

	transfers, ok := body["transfers"].([]interface{})
	if !ok || len(transfers) < 2 {
		t.Fatalf("expected at least 2 transfers in history, got: %v", body)
	}

	first, ok := transfers[0].(map[string]interface{})
	if !ok {
		t.Fatalf("could not parse first transfer item: %v", transfers[0])
	}
	second, ok := transfers[1].(map[string]interface{})
	if !ok {
		t.Fatalf("could not parse second transfer item: %v", transfers[1])
	}

	if got := toNumericString(first["id"]); got != secondTransferID {
		t.Fatalf("expected newest transfer ID %s first, got %s", secondTransferID, got)
	}
	if got := toNumericString(second["id"]); got != firstTransferID {
		t.Fatalf("expected older transfer ID %s second, got %s", firstTransferID, got)
	}
}

func createAndConfirmTransfer(t *testing.T, clientToken, fromAccountID, toAccountID string, amount float64, purpose string) string {
	t.Helper()

	createResp, createBody := postJSONWithToken(t, "/transfers", clientToken, map[string]interface{}{
		"racunPosiljaocaId": toNumber(fromAccountID),
		"racunPrimaocaId":   toNumber(toAccountID),
		"iznos":             amount,
		"svrha":             fmt.Sprintf("%s-%d", purpose, time.Now().UnixNano()),
	})
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create transfer expected 200, got %d: %v", createResp.StatusCode, createBody)
	}

	transferObj, ok := createBody["transfer"].(map[string]interface{})
	if !ok {
		t.Fatalf("create transfer response missing transfer object: %v", createBody)
	}
	transferID := toNumericString(transferObj["id"])
	if transferID == "" {
		t.Fatalf("create transfer missing transfer ID: %v", createBody)
	}

	approveResp, approveBody := postJSONWithToken(t, "/transfers/"+transferID+"/approve", clientToken, map[string]interface{}{
		"mode": "confirm",
	})
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("approve transfer expected 200, got %d: %v", approveResp.StatusCode, approveBody)
	}

	return transferID
}
