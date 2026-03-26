//go:build integration

package integration_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestEmployeeAuthFlow_ActivationResetAndNoDefaultPermissions(t *testing.T) {
	adminToken := adminLogin(t)
	employee := createEmployeeViaAdmin(t, adminToken, "integration.employee")

	activationBody := waitForMailhogBody(t, employee.Email, "Activate Your Bank Account")
	requireBodyContains(t, activationBody, "/activate/", "24 hours")
	activationToken := extractTokenFromMailBody(t, activationBody, "/activate/")

	activateEmployeeViaToken(t, activationToken, "EmpPass12!")

	employeeToken, loginBody := loginEmployee(t, employee.Email, "EmpPass12!")
	employeeInfo, ok := loginBody["employee"].(map[string]interface{})
	if !ok {
		t.Fatalf("employee login response missing employee object: %v", loginBody)
	}

	permissions, ok := employeeInfo["permissions"].([]interface{})
	if !ok {
		t.Fatalf("employee login response missing permissions array: %v", employeeInfo)
	}
	if len(permissions) != 0 {
		t.Fatalf("new employee expected no default permissions, got %v", permissions)
	}

	forbiddenResp, forbiddenBody := getWithToken(t, "/employees", employeeToken)
	if forbiddenResp.StatusCode == http.StatusOK {
		t.Fatalf("non-admin employee unexpectedly accessed employee management: %v", forbiddenBody)
	}

	resetResp, resetBody := postJSON(t, "/auth/password-reset/request", map[string]string{
		"email": employee.Email,
	})
	if resetResp.StatusCode != http.StatusOK {
		t.Fatalf("password reset request expected 200, got %d: %v", resetResp.StatusCode, resetBody)
	}

	resetMailBody := waitForMailhogBody(t, employee.Email, "Reset Your Password")
	requireBodyContains(t, resetMailBody, "/reset-password/", "1 hour")
	if token := extractTokenFromMailBody(t, resetMailBody, "/reset-password/"); strings.TrimSpace(token) == "" {
		t.Fatal("reset email token was empty")
	}
}

func TestEmployeeManagement_UpdateAndDeactivateBlocksLogin(t *testing.T) {
	adminToken := adminLogin(t)
	employee := createEmployeeViaAdmin(t, adminToken, "integration.update")

	activationBody := waitForMailhogBody(t, employee.Email, "Activate Your Bank Account")
	activationToken := extractTokenFromMailBody(t, activationBody, "/activate/")
	activateEmployeeViaToken(t, activationToken, "EmpPass12!")

	updateResp, updateBody := putJSONWithToken(t, "/employees/"+employee.ID, adminToken, map[string]interface{}{
		"ime":           employee.Ime,
		"prezime":       employee.Prezime,
		"datumRodjenja": employee.DatumRodjenja,
		"pol":           employee.Pol,
		"email":         employee.Email,
		"brojTelefona":  "0699999999",
		"adresa":        employee.Adresa,
		"username":      employee.Username,
		"pozicija":      employee.Pozicija,
		"departman":     "Support",
		"aktivan":       true,
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("employee update expected 200, got %d: %v", updateResp.StatusCode, updateBody)
	}

	getResp, getBody := getWithToken(t, "/employees/"+employee.ID, adminToken)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get employee expected 200, got %d: %v", getResp.StatusCode, getBody)
	}

	employeeObj, ok := getBody["employee"].(map[string]interface{})
	if !ok {
		t.Fatalf("get employee response missing employee object: %v", getBody)
	}
	if phone := employeeObj["brojTelefona"]; phone != "0699999999" {
		t.Fatalf("expected updated phone, got %v", phone)
	}
	if department := employeeObj["departman"]; department != "Support" {
		t.Fatalf("expected updated department, got %v", department)
	}

	deactivateResp, deactivateBody := patchJSONWithToken(t, "/employees/"+employee.ID+"/active", adminToken, map[string]interface{}{
		"aktivan": false,
	})
	if deactivateResp.StatusCode != http.StatusOK {
		t.Fatalf("employee deactivate expected 200, got %d: %v", deactivateResp.StatusCode, deactivateBody)
	}

	loginResp, loginBody := postJSON(t, "/auth/login", map[string]string{
		"email":    employee.Email,
		"password": "EmpPass12!",
	})
	if loginResp.StatusCode == http.StatusOK {
		t.Fatalf("deactivated employee should not log in successfully: %v", loginBody)
	}
}
