//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

type clientFixture struct {
	ID            string
	Ime           string
	Prezime       string
	Email         string
	BrojTelefona  string
	Adresa        string
	SetupToken    string
	DatumRodjenja int64
	Pol           string
}

type employeeFixture struct {
	ID            string
	Ime           string
	Prezime       string
	Email         string
	BrojTelefona  string
	Adresa        string
	Username      string
	Pozicija      string
	Departman     string
	DatumRodjenja int64
	Pol           string
}

type accountFixture struct {
	ID         string
	Naziv      string
	BrojRacuna string
	CurrencyID int
}

func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func createClientViaAdmin(t *testing.T, adminToken, label string) clientFixture {
	t.Helper()

	suffix := uniqueSuffix()
	fixture := clientFixture{
		Ime:           "Test",
		Prezime:       "Client",
		DatumRodjenja: 946684800,
		Pol:           "M",
		Email:         fmt.Sprintf("%s.%s@example.com", label, suffix),
		BrojTelefona:  "0611234567",
		Adresa:        "Integration Test 1",
	}

	resp, body := postJSONWithToken(t, "/clients", adminToken, map[string]interface{}{
		"ime":           fixture.Ime,
		"prezime":       fixture.Prezime,
		"datumRodjenja": fixture.DatumRodjenja,
		"pol":           fixture.Pol,
		"email":         fixture.Email,
		"brojTelefona":  fixture.BrojTelefona,
		"adresa":        fixture.Adresa,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create client expected 200, got %d: %v", resp.StatusCode, body)
	}

	clientObj, ok := body["client"].(map[string]interface{})
	if !ok {
		t.Fatalf("create client missing client object: %v", body)
	}
	fixture.ID = toNumericString(clientObj["id"])
	if fixture.ID == "" {
		t.Fatalf("create client missing client ID: %v", body)
	}

	message, _ := body["message"].(string)
	fixture.SetupToken = extractSetupToken(message)
	if fixture.SetupToken == "" {
		t.Fatalf("create client missing setup token: %v", body)
	}

	return fixture
}

func activateClientViaToken(t *testing.T, setupToken, password string) {
	t.Helper()

	resp, body := postJSON(t, "/auth/client/activate", map[string]string{
		"token":           setupToken,
		"password":        password,
		"passwordConfirm": password,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client activate expected 200, got %d: %v", resp.StatusCode, body)
	}
}

func loginClientToken(t *testing.T, email, password string) string {
	t.Helper()

	resp, body := postJSON(t, "/auth/client/login", map[string]string{
		"email":    email,
		"password": password,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client login expected 200, got %d: %v", resp.StatusCode, body)
	}
	token, _ := body["accessToken"].(string)
	if token == "" {
		t.Fatalf("client login missing access token: %v", body)
	}
	return token
}

func createCheckingAccountViaAdmin(t *testing.T, adminToken, clientID, naziv string, initialBalance float64) accountFixture {
	t.Helper()

	resp, body := postJSONWithToken(t, "/accounts/create", adminToken, map[string]interface{}{
		"clientId":      toNumber(clientID),
		"currencyId":    1,
		"tip":           "tekuci",
		"vrsta":         "licni",
		"podvrsta":      "standardni",
		"naziv":         naziv,
		"pocetnoStanje": initialBalance,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create account expected 200, got %d: %v", resp.StatusCode, body)
	}

	accountObj, ok := body["account"].(map[string]interface{})
	if !ok {
		t.Fatalf("create account missing account object: %v", body)
	}

	fixture := accountFixture{
		ID:         toNumericString(accountObj["id"]),
		Naziv:      naziv,
		BrojRacuna: fmt.Sprintf("%v", accountObj["brojRacuna"]),
		CurrencyID: 1,
	}
	if fixture.ID == "" || fixture.BrojRacuna == "" {
		t.Fatalf("create account missing account fields: %v", body)
	}

	return fixture
}

func createEmployeeViaAdmin(t *testing.T, adminToken, label string) employeeFixture {
	t.Helper()

	suffix := uniqueSuffix()
	fixture := employeeFixture{
		Ime:           "Test",
		Prezime:       "Employee",
		DatumRodjenja: 946684800,
		Pol:           "M",
		Email:         fmt.Sprintf("%s.%s@bank.com", label, suffix),
		BrojTelefona:  "0621234567",
		Adresa:        "Employee Street 1",
		Username:      fmt.Sprintf("emp%s", suffix),
		Pozicija:      "Developer",
		Departman:     "IT",
	}

	resp, body := postJSONWithToken(t, "/employees", adminToken, map[string]interface{}{
		"ime":           fixture.Ime,
		"prezime":       fixture.Prezime,
		"datumRodjenja": fixture.DatumRodjenja,
		"pol":           fixture.Pol,
		"email":         fixture.Email,
		"brojTelefona":  fixture.BrojTelefona,
		"adresa":        fixture.Adresa,
		"username":      fixture.Username,
		"pozicija":      fixture.Pozicija,
		"departman":     fixture.Departman,
		"aktivan":       false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create employee expected 200, got %d: %v", resp.StatusCode, body)
	}

	employeeObj, ok := body["employee"].(map[string]interface{})
	if !ok {
		t.Fatalf("create employee missing employee object: %v", body)
	}
	fixture.ID = toNumericString(employeeObj["id"])
	if fixture.ID == "" {
		t.Fatalf("create employee missing employee ID: %v", body)
	}

	return fixture
}

func activateEmployeeViaToken(t *testing.T, token, password string) {
	t.Helper()

	resp, body := postJSON(t, "/auth/activate", map[string]string{
		"token":           token,
		"password":        password,
		"passwordConfirm": password,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("employee activate expected 200, got %d: %v", resp.StatusCode, body)
	}
}

func loginEmployee(t *testing.T, email, password string) (string, map[string]interface{}) {
	t.Helper()

	resp, body := postJSON(t, "/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("employee login expected 200, got %d: %v", resp.StatusCode, body)
	}
	token, _ := body["accessToken"].(string)
	if token == "" {
		t.Fatalf("employee login missing access token: %v", body)
	}
	return token, body
}
