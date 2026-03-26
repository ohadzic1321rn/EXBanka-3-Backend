//go:build integration

package integration_test

import (
	"net/http"
	"net/url"
	"testing"
)

func TestClientManagement_SearchAndUpdate(t *testing.T) {
	adminToken := adminLogin(t)
	client := createClientViaAdmin(t, adminToken, "integration.client")

	searchResp, searchBody := getWithToken(t, "/clients?emailFilter="+url.QueryEscape(client.Email), adminToken)
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("client search by email expected 200, got %d: %v", searchResp.StatusCode, searchBody)
	}
	clients, ok := searchBody["clients"].([]interface{})
	if !ok || len(clients) == 0 {
		t.Fatalf("client search by email returned no results: %v", searchBody)
	}

	nameResp, nameBody := getWithToken(t, "/clients?nameFilter="+url.QueryEscape(client.Prezime), adminToken)
	if nameResp.StatusCode != http.StatusOK {
		t.Fatalf("client search by name expected 200, got %d: %v", nameResp.StatusCode, nameBody)
	}
	nameClients, ok := nameBody["clients"].([]interface{})
	if !ok || len(nameClients) == 0 {
		t.Fatalf("client search by name returned no results: %v", nameBody)
	}

	updateResp, updateBody := putJSONWithToken(t, "/clients/"+client.ID, adminToken, map[string]interface{}{
		"ime":            client.Ime,
		"prezime":        client.Prezime,
		"datumRodjenja":  client.DatumRodjenja,
		"pol":            client.Pol,
		"email":          client.Email,
		"brojTelefona":   "0605554444",
		"adresa":         "Updated Client Address 99",
		"povezaniRacuni": "",
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("client update expected 200, got %d: %v", updateResp.StatusCode, updateBody)
	}

	getResp, getBody := getWithToken(t, "/clients/"+client.ID, adminToken)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get client expected 200, got %d: %v", getResp.StatusCode, getBody)
	}

	clientObj, ok := getBody["client"].(map[string]interface{})
	if !ok {
		t.Fatalf("get client response missing client object: %v", getBody)
	}
	if phone := clientObj["brojTelefona"]; phone != "0605554444" {
		t.Fatalf("expected updated client phone, got %v", phone)
	}
	if address := clientObj["adresa"]; address != "Updated Client Address 99" {
		t.Fatalf("expected updated client address, got %v", address)
	}
}
