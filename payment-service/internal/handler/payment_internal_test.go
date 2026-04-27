package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	paymentv1 "github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/gen/proto/payment/v1"
	prv1 "github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/gen/proto/payment_recipient/v1"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/config"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/middleware"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/service"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/util"
)

// =====================
// mock services
// =====================

type mockCreateSvc struct {
	result *models.Payment
	err    error
}

func (m *mockCreateSvc) CreatePayment(_ service.CreatePaymentInput) (*models.Payment, error) {
	return m.result, m.err
}

type mockMobileSvc struct {
	approveResult  *models.Payment
	approveCode    string
	approveExpiry  *time.Time
	approveErr     error
	rejectResult   *models.Payment
	rejectErr      error
}

func (m *mockMobileSvc) ApprovePaymentMobile(_ uint, _ string) (*models.Payment, string, *time.Time, error) {
	return m.approveResult, m.approveCode, m.approveExpiry, m.approveErr
}

func (m *mockMobileSvc) RejectPayment(_ uint) (*models.Payment, error) {
	return m.rejectResult, m.rejectErr
}

type mockPrenosSvc struct {
	createResult *models.Payment
	createErr    error
	verifyResult *models.Payment
	verifyErr    error
}

func (m *mockPrenosSvc) CreatePrenos(_ service.CreatePrenosInput) (*models.Payment, error) {
	return m.createResult, m.createErr
}

func (m *mockPrenosSvc) VerifyPrenos(_ uint, _ string) (*models.Payment, error) {
	return m.verifyResult, m.verifyErr
}

// =====================
// helpers
// =====================

func makePaymentModel(id uint) *models.Payment {
	return &models.Payment{
		ID:                id,
		RacunPosiljaocaID: 1,
		RacunPrimaocaBroj: "000000000000000001",
		Iznos:             500,
		Status:            "u_obradi",
	}
}

func newCreateHandler(svc paymentCreateService) *CreatePaymentHTTPHandler {
	return &CreatePaymentHTTPHandler{svc: svc}
}

func newMobileHandler(svc paymentMobileVerificationService) *PaymentMobileVerificationHandler {
	return &PaymentMobileVerificationHandler{svc: svc}
}

func newPrenosHandler(svc prenosService) *PrenosHTTPHandler {
	return &PrenosHTTPHandler{svc: svc}
}

// =====================
// http_auth helpers
// =====================

func TestWriteAuthError_SetsStatusAndBody(t *testing.T) {
	w := httptest.NewRecorder()
	writeAuthError(w, http.StatusForbidden, "access denied")
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "access denied") {
		t.Errorf("body missing message: %s", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestParseHTTPClaims_NilConfig_ReturnsNilTrue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	claims, ok := parseHTTPClaims(w, r, nil)
	if !ok {
		t.Error("expected ok=true for nil config")
	}
	if claims != nil {
		t.Error("expected nil claims for nil config")
	}
}

func TestParseHTTPClaims_MissingHeader_Returns401(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	_, ok := parseHTTPClaims(w, r, cfg)
	if ok {
		t.Error("expected ok=false for missing header")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestParseHTTPClaims_InvalidBearerPrefix_Returns401(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Basic abc")
	w := httptest.NewRecorder()
	_, ok := parseHTTPClaims(w, r, cfg)
	if ok {
		t.Error("expected ok=false for non-bearer scheme")
	}
}

func TestParseHTTPClaims_EmptyTokenAfterBearer_Returns401(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer   ")
	w := httptest.NewRecorder()
	_, ok := parseHTTPClaims(w, r, cfg)
	if ok {
		t.Error("expected ok=false for empty token")
	}
}

func TestParseHTTPClaims_InvalidToken_Returns401(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer notavalidtoken")
	w := httptest.NewRecorder()
	_, ok := parseHTTPClaims(w, r, cfg)
	if ok {
		t.Error("expected ok=false for invalid token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireClientPermissionHTTP_NilClaims_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	if !requireClientPermissionHTTP(w, nil, models.PermClientBasic) {
		t.Error("expected true for nil claims")
	}
}

func TestRequireClientPermissionHTTP_NonClientSource_ReturnsFalse(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "employee", ClientID: 1}
	if requireClientPermissionHTTP(w, claims, models.PermClientBasic) {
		t.Error("expected false for non-client source")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireClientPermissionHTTP_ZeroClientID_ReturnsFalse(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 0}
	if requireClientPermissionHTTP(w, claims, models.PermClientBasic) {
		t.Error("expected false for zero client ID")
	}
}

func TestRequireClientPermissionHTTP_MissingPerm_ReturnsFalse(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 1, Permissions: []string{}}
	if requireClientPermissionHTTP(w, claims, models.PermClientBasic) {
		t.Error("expected false when permission missing")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireClientPermissionHTTP_HasPerm_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 1, Permissions: []string{models.PermClientBasic}}
	if !requireClientPermissionHTTP(w, claims, models.PermClientBasic) {
		t.Error("expected true when client has permission")
	}
}

func TestRequireClientOwnershipHTTP_NilClaims_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	if !requireClientOwnershipHTTP(w, nil, 5) {
		t.Error("expected true for nil claims")
	}
}

func TestRequireClientOwnershipHTTP_MatchingID_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{ClientID: 5}
	if !requireClientOwnershipHTTP(w, claims, 5) {
		t.Error("expected true for matching client ID")
	}
}

func TestRequireClientOwnershipHTTP_MismatchID_ReturnsFalse(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{ClientID: 5}
	if requireClientOwnershipHTTP(w, claims, 99) {
		t.Error("expected false for mismatched client ID")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRequireClientBasicHTTP_NilClaims_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	if !requireClientBasicHTTP(w, nil, 5) {
		t.Error("expected true for nil claims")
	}
}

func TestRequireClientBasicHTTP_ValidClient_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 5, Permissions: []string{models.PermClientBasic}}
	if !requireClientBasicHTTP(w, claims, 5) {
		t.Error("expected true for valid client")
	}
}

func TestRequireClientBasicHTTP_WrongClient_ReturnsFalse(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 5, Permissions: []string{models.PermClientBasic}}
	if requireClientBasicHTTP(w, claims, 99) {
		t.Error("expected false for wrong client ID")
	}
}


// =====================
// writeJSON
// =====================

func TestWriteJSON_SetsStatusAndBody(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]interface{}{"key": "value"})
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("expected key=value, got %v", body["key"])
	}
}

// =====================
// toPaymentHTTPJSON
// =====================

func TestToPaymentHTTPJSON_WithRecipient(t *testing.T) {
	rid := uint(7)
	p := makePaymentModel(3)
	p.RecipientID = &rid
	j := toPaymentHTTPJSON(p)
	if j.ID != "3" {
		t.Errorf("expected ID=3, got %s", j.ID)
	}
	if j.RecipientID != "7" {
		t.Errorf("expected RecipientID=7, got %s", j.RecipientID)
	}
}

func TestToPaymentHTTPJSON_WithoutRecipient(t *testing.T) {
	p := makePaymentModel(5)
	j := toPaymentHTTPJSON(p)
	if j.RecipientID != "0" {
		t.Errorf("expected RecipientID=0, got %s", j.RecipientID)
	}
}

// =====================
// defaultString
// =====================

func TestDefaultString_ReturnsValue_WhenNonEmpty(t *testing.T) {
	if got := defaultString("hello", "fallback"); got != "hello" {
		t.Errorf("expected 'hello', got '%s'", got)
	}
}

func TestDefaultString_ReturnsFallback_WhenEmpty(t *testing.T) {
	if got := defaultString("", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", got)
	}
}

func TestDefaultString_ReturnsFallback_WhenWhitespace(t *testing.T) {
	if got := defaultString("   ", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", got)
	}
}

// =====================
// extractPaymentID
// =====================

func TestExtractPaymentID_ValidPath(t *testing.T) {
	id, err := extractPaymentID("/api/v1/payment/42/approve")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected 42, got %d", id)
	}
}

func TestExtractPaymentID_ShortPath_ReturnsError(t *testing.T) {
	if _, err := extractPaymentID("/api/v1"); err == nil {
		t.Error("expected error for short path")
	}
}

func TestExtractPaymentID_NonNumericID_ReturnsError(t *testing.T) {
	if _, err := extractPaymentID("/api/v1/payment/notanumber/approve"); err == nil {
		t.Error("expected error for non-numeric ID")
	}
}

// =====================
// extractPrenosID
// =====================

func TestExtractPrenosID_ValidPath(t *testing.T) {
	id, err := extractPrenosID("/api/v1/prenos/99/verify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 99 {
		t.Errorf("expected 99, got %d", id)
	}
}

func TestExtractPrenosID_ShortPath_ReturnsError(t *testing.T) {
	if _, err := extractPrenosID("/short"); err == nil {
		t.Error("expected error for short path")
	}
}

func TestExtractPrenosID_NonNumericID_ReturnsError(t *testing.T) {
	if _, err := extractPrenosID("/api/v1/prenos/abc/verify"); err == nil {
		t.Error("expected error for non-numeric ID")
	}
}

// =====================
// CreatePaymentHTTPHandler
// =====================

func TestCreateHTTPHandler_WrongMethod_Returns405(t *testing.T) {
	h := newCreateHandler(&mockCreateSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/payments", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestCreateHTTPHandler_InvalidJSON_Returns400(t *testing.T) {
	h := newCreateHandler(&mockCreateSvc{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateHTTPHandler_ServiceError_Returns400(t *testing.T) {
	h := newCreateHandler(&mockCreateSvc{err: errors.New("insufficient funds")})
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"000001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateHTTPHandler_Success_Returns200(t *testing.T) {
	h := newCreateHandler(&mockCreateSvc{result: makePaymentModel(1)})
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"000001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateHTTPHandler_CamelCaseFields_Accepted(t *testing.T) {
	h := newCreateHandler(&mockCreateSvc{result: makePaymentModel(2)})
	body := `{"racunPosiljaocaId":1,"racunPrimaocaBroj":"000001","iznos":200,"sifraPlacanja":"289","pozivNaBroj":"99-001","addRecipient":true,"recipientNaziv":"Test"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// =====================
// PaymentMobileVerificationHandler
// =====================

func TestMobileHandler_Approve_WrongMethod_Returns405(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/payment/1/approve", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestMobileHandler_Approve_BadPath_Returns400(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	r := httptest.NewRequest(http.MethodPost, "/bad", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMobileHandler_Approve_Success_Returns200(t *testing.T) {
	svc := &mockMobileSvc{approveResult: makePaymentModel(5)}
	h := newMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/5/approve", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMobileHandler_Approve_WithVerificationCode_Returns200(t *testing.T) {
	expiry := time.Now().Add(10 * time.Minute)
	svc := &mockMobileSvc{approveResult: makePaymentModel(5), approveCode: "ABC123", approveExpiry: &expiry}
	h := newMobileHandler(svc)
	body := `{"mode":"code"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/5/approve", strings.NewReader(body))
	r.ContentLength = int64(len(body))
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ABC123") {
		t.Error("expected verificationCode in response")
	}
}

func TestMobileHandler_Approve_ServiceError_Returns400(t *testing.T) {
	svc := &mockMobileSvc{approveErr: errors.New("payment not found")}
	h := newMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/5/approve", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMobileHandler_Approve_InvalidBody_Returns400(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{approveResult: makePaymentModel(1)})
	body := "not json"
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/1/approve", strings.NewReader(body))
	r.ContentLength = int64(len(body))
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMobileHandler_Reject_WrongMethod_Returns405(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/payment/1/reject", nil)
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestMobileHandler_Reject_BadPath_Returns400(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	r := httptest.NewRequest(http.MethodPost, "/bad", nil)
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMobileHandler_Reject_Success_Returns200(t *testing.T) {
	svc := &mockMobileSvc{rejectResult: makePaymentModel(3)}
	h := newMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/3/reject", nil)
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMobileHandler_Reject_ServiceError_Returns400(t *testing.T) {
	svc := &mockMobileSvc{rejectErr: errors.New("already settled")}
	h := newMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/3/reject", nil)
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMobileHandler_WriteVerificationError_PaymentNotPending_Returns409(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	verErr := &service.PaymentVerificationError{
		Code:    "payment_not_pending",
		Message: "not pending",
		Status:  "uspesno",
	}
	h.writeVerificationError(w, verErr)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["code"] != "payment_not_pending" {
		t.Errorf("expected code=payment_not_pending, got %v", body["code"])
	}
}

func TestMobileHandler_WriteVerificationError_WithAttemptsRemaining(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	verErr := &service.PaymentVerificationError{
		Code:              "insufficient_balance",
		Message:           "not enough",
		AttemptsRemaining: 2,
	}
	h.writeVerificationError(w, verErr)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if _, ok := body["attemptsRemaining"]; !ok {
		t.Error("expected attemptsRemaining in response")
	}
}

func TestMobileHandler_WriteVerificationError_UnsupportedMode(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	h.writeVerificationError(w, errors.New("unsupported approval mode"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["code"] != "unsupported_approval_mode" {
		t.Errorf("expected unsupported_approval_mode, got %v", body["code"])
	}
}

func TestMobileHandler_WriteVerificationError_GenericError(t *testing.T) {
	h := newMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	h.writeVerificationError(w, errors.New("something went wrong"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// =====================
// CombinedPaymentHandler
// =====================

func TestCombinedHandler_RoutesToCreate(t *testing.T) {
	createH := newCreateHandler(&mockCreateSvc{result: makePaymentModel(1)})
	mobileH := newMobileHandler(&mockMobileSvc{})
	combined := NewCombinedPaymentHandler(createH, mobileH, http.NotFoundHandler())

	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(body))
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_RoutesToApprove(t *testing.T) {
	mobileH := newMobileHandler(&mockMobileSvc{approveResult: makePaymentModel(1)})
	combined := NewCombinedPaymentHandler(newCreateHandler(&mockCreateSvc{}), mobileH, http.NotFoundHandler())

	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/1/approve", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_RoutesToReject(t *testing.T) {
	mobileH := newMobileHandler(&mockMobileSvc{rejectResult: makePaymentModel(1)})
	combined := NewCombinedPaymentHandler(newCreateHandler(&mockCreateSvc{}), mobileH, http.NotFoundHandler())

	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/1/reject", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_UnknownRoute_UsesFallback(t *testing.T) {
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	combined := NewCombinedPaymentHandler(newCreateHandler(&mockCreateSvc{}), newMobileHandler(&mockMobileSvc{}), fallback)

	r := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusTeapot {
		t.Errorf("expected 418, got %d", w.Code)
	}
}

// =====================
// PrenosHTTPHandler
// =====================

func TestPrenosHandler_ServeHTTP_CreateRoute(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{createResult: makePaymentModel(1)})
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPrenosHandler_ServeHTTP_VerifyRoute(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{verifyResult: makePaymentModel(1)})
	body := `{"verification_code":"ABC123"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos/1/verify", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPrenosHandler_ServeHTTP_UnknownRoute_Returns404(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPrenosHandler_Create_InvalidJSON_Returns400(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPrenosHandler_Create_ServiceError_Returns400(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{createErr: errors.New("insufficient funds")})
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPrenosHandler_Create_CamelCase_Accepted(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{createResult: makePaymentModel(2)})
	body := `{"racunPosiljaocaId":2,"racunPrimaocaBroj":"002","iznos":200}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPrenosHandler_Verify_BadPath_Returns400(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{})
	body := `{"verification_code":"ABC"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos/bad/verify", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPrenosHandler_Verify_MissingCode_Returns400(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{})
	body := `{"verification_code":""}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos/1/verify", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPrenosHandler_Verify_ServiceError_Returns400(t *testing.T) {
	h := newPrenosHandler(&mockPrenosSvc{verifyErr: errors.New("invalid code")})
	body := `{"verification_code":"WRONG"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos/1/verify", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPrenosHandler_Verify_ServiceVerificationError_Returns409(t *testing.T) {
	verErr := &service.PaymentVerificationError{
		Code:              "payment_not_pending",
		Message:           "payment is not pending",
		AttemptsRemaining: 2,
	}
	h := newPrenosHandler(&mockPrenosSvc{verifyErr: verErr})
	body := `{"verification_code":"WRONG"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos/1/verify", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// =====================
// gRPC PaymentHandler mocks
// =====================

type mockPaymentGRPCSvc struct {
	createResult        *models.Payment
	createErr           error
	verifyResult        *models.Payment
	verifyErr           error
	getResult           *models.Payment
	getErr              error
	listByAccountResult []models.Payment
	listByAccountErr    error
	listByClientResult  []models.Payment
	listByClientErr     error
}

func (m *mockPaymentGRPCSvc) CreatePayment(_ service.CreatePaymentInput) (*models.Payment, error) {
	return m.createResult, m.createErr
}
func (m *mockPaymentGRPCSvc) VerifyPayment(_ uint, _ string) (*models.Payment, error) {
	return m.verifyResult, m.verifyErr
}
func (m *mockPaymentGRPCSvc) GetPayment(_ uint) (*models.Payment, error) {
	return m.getResult, m.getErr
}
func (m *mockPaymentGRPCSvc) ListPaymentsByAccount(_ uint, _ models.PaymentFilter) ([]models.Payment, int64, error) {
	return m.listByAccountResult, int64(len(m.listByAccountResult)), m.listByAccountErr
}
func (m *mockPaymentGRPCSvc) ListPaymentsByClient(_ uint, _ models.PaymentFilter) ([]models.Payment, int64, error) {
	return m.listByClientResult, int64(len(m.listByClientResult)), m.listByClientErr
}

func newPaymentGRPCHandler(svc PaymentServiceInterface) *PaymentHandler {
	return NewPaymentHandlerWithService(svc)
}

func ctxWithClaims(claims *util.Claims) context.Context {
	return context.WithValue(context.Background(), middleware.ClaimsKey, claims)
}

func clientClaims(clientID uint) *util.Claims {
	return &util.Claims{
		ClientID:    clientID,
		TokenSource: "client",
		Permissions: []string{models.PermClientBasic},
	}
}

// =====================
// PaymentHandler.CreatePayment (gRPC)
// =====================

func TestPaymentHandler_CreatePayment_MissingPosiljaocaID_ReturnsError(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	_, err := h.CreatePayment(context.Background(), &paymentv1.CreatePaymentRequest{RacunPrimaocaBroj: "001"})
	if err == nil {
		t.Error("expected error for missing racun_posiljaoca_id")
	}
}

func TestPaymentHandler_CreatePayment_MissingPrimaocaBroj_ReturnsError(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	_, err := h.CreatePayment(context.Background(), &paymentv1.CreatePaymentRequest{RacunPosiljaocaId: 1})
	if err == nil {
		t.Error("expected error for missing racun_primaoca_broj")
	}
}

func TestPaymentHandler_CreatePayment_Success(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{createResult: makePaymentModel(10)})
	resp, err := h.CreatePayment(context.Background(), &paymentv1.CreatePaymentRequest{
		RacunPosiljaocaId: 1,
		RacunPrimaocaBroj: "000111000111000111",
		Iznos:             100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Payment.Id != 10 {
		t.Errorf("expected payment ID=10, got %d", resp.Payment.Id)
	}
}

func TestPaymentHandler_CreatePayment_ServiceError_ReturnsInvalidArgument(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{createErr: errors.New("insufficient funds")})
	_, err := h.CreatePayment(context.Background(), &paymentv1.CreatePaymentRequest{
		RacunPosiljaocaId: 1,
		RacunPrimaocaBroj: "001",
	})
	if err == nil {
		t.Error("expected error from service")
	}
}

func TestPaymentHandler_CreatePayment_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{
		RacunPosiljaocaId: 1,
		RacunPrimaocaBroj: "001",
	})
	if err == nil {
		t.Error("expected error for zero client ID")
	}
}

func TestPaymentHandler_CreatePayment_WithClaims_NilDB_Success(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{createResult: makePaymentModel(11)})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{
		RacunPosiljaocaId: 1,
		RacunPrimaocaBroj: "001",
		Iznos:             50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentHandler_CreatePayment_WithClaims_WithRecipientID(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{createResult: makePaymentModel(12)})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{
		RacunPosiljaocaId: 1,
		RacunPrimaocaBroj: "001",
		Iznos:             50,
		RecipientId:       3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// PaymentHandler.VerifyPayment (gRPC)
// =====================

func TestPaymentHandler_VerifyPayment_Success(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{verifyResult: makePaymentModel(5)})
	resp, err := h.VerifyPayment(context.Background(), &paymentv1.VerifyPaymentRequest{
		Id:               5,
		VerificationCode: "ABC123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Payment.Id != 5 {
		t.Errorf("expected ID=5, got %d", resp.Payment.Id)
	}
}

func TestPaymentHandler_VerifyPayment_ServiceError_ReturnsInvalidArgument(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{verifyErr: errors.New("bad code")})
	_, err := h.VerifyPayment(context.Background(), &paymentv1.VerifyPaymentRequest{Id: 1, VerificationCode: "X"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPaymentHandler_VerifyPayment_VerificationError_NotPending(t *testing.T) {
	verErr := &service.PaymentVerificationError{Code: "payment_not_pending", Message: "not pending"}
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{verifyErr: verErr})
	_, err := h.VerifyPayment(context.Background(), &paymentv1.VerifyPaymentRequest{Id: 1, VerificationCode: "X"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPaymentHandler_VerifyPayment_VerificationError_Default(t *testing.T) {
	verErr := &service.PaymentVerificationError{Code: "invalid_code", Message: "invalid"}
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{verifyErr: verErr})
	_, err := h.VerifyPayment(context.Background(), &paymentv1.VerifyPaymentRequest{Id: 1, VerificationCode: "X"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPaymentHandler_VerifyPayment_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.VerifyPayment(ctx, &paymentv1.VerifyPaymentRequest{Id: 1, VerificationCode: "X"})
	if err == nil {
		t.Error("expected error for zero client ID")
	}
}

func TestPaymentHandler_VerifyPayment_WithClaims_NilDB_Success(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{verifyResult: makePaymentModel(3)})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.VerifyPayment(ctx, &paymentv1.VerifyPaymentRequest{Id: 3, VerificationCode: "ABC"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// PaymentHandler.GetPayment (gRPC)
// =====================

func TestPaymentHandler_GetPayment_Success(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{getResult: makePaymentModel(7)})
	resp, err := h.GetPayment(context.Background(), &paymentv1.GetPaymentRequest{Id: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Payment.Id != 7 {
		t.Errorf("expected ID=7, got %d", resp.Payment.Id)
	}
}

func TestPaymentHandler_GetPayment_ServiceError_ReturnsNotFound(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{getErr: errors.New("not found")})
	_, err := h.GetPayment(context.Background(), &paymentv1.GetPaymentRequest{Id: 99})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPaymentHandler_GetPayment_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.GetPayment(ctx, &paymentv1.GetPaymentRequest{Id: 1})
	if err == nil {
		t.Error("expected error for zero client ID")
	}
}

func TestPaymentHandler_GetPayment_WithClaims_NilDB_Success(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{getResult: makePaymentModel(8)})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.GetPayment(ctx, &paymentv1.GetPaymentRequest{Id: 8})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// PaymentHandler.ListPaymentsByAccount (gRPC)
// =====================

func TestPaymentHandler_ListPaymentsByAccount_Success(t *testing.T) {
	payments := []models.Payment{*makePaymentModel(1), *makePaymentModel(2)}
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{listByAccountResult: payments})
	resp, err := h.ListPaymentsByAccount(context.Background(), &paymentv1.ListPaymentsByAccountRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Payments) != 2 {
		t.Errorf("expected 2 payments, got %d", len(resp.Payments))
	}
}

func TestPaymentHandler_ListPaymentsByAccount_ServiceError(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{listByAccountErr: errors.New("db error")})
	_, err := h.ListPaymentsByAccount(context.Background(), &paymentv1.ListPaymentsByAccountRequest{AccountId: 1})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPaymentHandler_ListPaymentsByAccount_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.ListPaymentsByAccount(ctx, &paymentv1.ListPaymentsByAccountRequest{AccountId: 1})
	if err == nil {
		t.Error("expected error for zero client ID")
	}
}

func TestPaymentHandler_ListPaymentsByAccount_WithClaims_NilDB(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{listByAccountResult: []models.Payment{*makePaymentModel(1)}})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.ListPaymentsByAccount(ctx, &paymentv1.ListPaymentsByAccountRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPaymentHandler_ListPaymentsByAccount_WithFilter(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{listByAccountResult: []models.Payment{}})
	req := &paymentv1.ListPaymentsByAccountRequest{
		AccountId: 1,
		Status:    "u_obradi",
		MinAmount: 10,
		MaxAmount: 1000,
		DateFrom:  "2024-01-01T00:00:00Z",
		DateTo:    "2024-12-31T00:00:00Z",
		Page:      1,
		PageSize:  10,
	}
	_, err := h.ListPaymentsByAccount(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// PaymentHandler.ListPaymentsByClient (gRPC)
// =====================

func TestPaymentHandler_ListPaymentsByClient_Success(t *testing.T) {
	payments := []models.Payment{*makePaymentModel(1)}
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{listByClientResult: payments})
	resp, err := h.ListPaymentsByClient(context.Background(), &paymentv1.ListPaymentsByClientRequest{ClientId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Payments) != 1 {
		t.Errorf("expected 1 payment, got %d", len(resp.Payments))
	}
}

func TestPaymentHandler_ListPaymentsByClient_ServiceError(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{listByClientErr: errors.New("db error")})
	_, err := h.ListPaymentsByClient(context.Background(), &paymentv1.ListPaymentsByClientRequest{ClientId: 5})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPaymentHandler_ListPaymentsByClient_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.ListPaymentsByClient(ctx, &paymentv1.ListPaymentsByClientRequest{ClientId: 5})
	if err == nil {
		t.Error("expected error for zero client ID")
	}
}

func TestPaymentHandler_ListPaymentsByClient_WithClaims_MismatchClientID_ReturnsDenied(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.ListPaymentsByClient(ctx, &paymentv1.ListPaymentsByClientRequest{ClientId: 99})
	if err == nil {
		t.Error("expected error for mismatched client ID")
	}
}

func TestPaymentHandler_ListPaymentsByClient_WithClaims_Match(t *testing.T) {
	h := newPaymentGRPCHandler(&mockPaymentGRPCSvc{listByClientResult: []models.Payment{}})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.ListPaymentsByClient(ctx, &paymentv1.ListPaymentsByClientRequest{ClientId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// parsePaymentFilter
// =====================

func TestParsePaymentFilter_WithAmounts(t *testing.T) {
	f := parsePaymentFilter("", "", "", 10.0, 500.0, 1, 20)
	if f.MinAmount == nil || *f.MinAmount != 10.0 {
		t.Errorf("expected MinAmount=10, got %v", f.MinAmount)
	}
	if f.MaxAmount == nil || *f.MaxAmount != 500.0 {
		t.Errorf("expected MaxAmount=500, got %v", f.MaxAmount)
	}
}

func TestParsePaymentFilter_WithValidDates(t *testing.T) {
	f := parsePaymentFilter("", "2024-01-01T00:00:00Z", "2024-12-31T00:00:00Z", 0, 0, 0, 0)
	if f.DateFrom == nil {
		t.Error("expected DateFrom to be set")
	}
	if f.DateTo == nil {
		t.Error("expected DateTo to be set")
	}
}

func TestParsePaymentFilter_WithInvalidDates_IgnoresThem(t *testing.T) {
	f := parsePaymentFilter("", "not-a-date", "also-not-a-date", 0, 0, 0, 0)
	if f.DateFrom != nil {
		t.Error("expected DateFrom to be nil for invalid input")
	}
	if f.DateTo != nil {
		t.Error("expected DateTo to be nil for invalid input")
	}
}

func TestToPaymentProto_WithRecipientID(t *testing.T) {
	rid := uint(42)
	p := makePaymentModel(9)
	p.RecipientID = &rid
	proto := toPaymentProto(p)
	if proto.RecipientId != 42 {
		t.Errorf("expected RecipientId=42, got %d", proto.RecipientId)
	}
}

// =====================
// gRPC PaymentRecipientHandler mocks
// =====================

type mockRecipientGRPCSvc struct {
	createResult *models.PaymentRecipient
	createErr    error
	listResult   []models.PaymentRecipient
	listErr      error
	updateResult *models.PaymentRecipient
	updateErr    error
	deleteErr    error
}

func (m *mockRecipientGRPCSvc) CreateRecipient(_ service.CreateRecipientInput) (*models.PaymentRecipient, error) {
	return m.createResult, m.createErr
}
func (m *mockRecipientGRPCSvc) ListRecipientsByClient(_ uint) ([]models.PaymentRecipient, error) {
	return m.listResult, m.listErr
}
func (m *mockRecipientGRPCSvc) UpdateRecipient(_ uint, _ uint, _ service.UpdateRecipientInput) (*models.PaymentRecipient, error) {
	return m.updateResult, m.updateErr
}
func (m *mockRecipientGRPCSvc) DeleteRecipient(_ uint, _ uint) error {
	return m.deleteErr
}

func newRecipientGRPCHandler(svc PaymentRecipientServiceInterface) *PaymentRecipientHandler {
	return NewPaymentRecipientHandlerWithService(svc)
}

func makeRecipient(id uint) *models.PaymentRecipient {
	return &models.PaymentRecipient{
		ID:         id,
		ClientID:   5,
		Naziv:      "Test",
		BrojRacuna: "000111000111000111",
	}
}

// =====================
// PaymentRecipientHandler.CreateRecipient (gRPC)
// =====================

func TestRecipientHandler_CreateRecipient_Success_NoContext(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{createResult: makeRecipient(1)})
	resp, err := h.CreateRecipient(context.Background(), &prv1.CreateRecipientRequest{
		ClientId:   5,
		Naziv:      "Test",
		BrojRacuna: "000111000111000111",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Recipient.Id != 1 {
		t.Errorf("expected ID=1, got %d", resp.Recipient.Id)
	}
}

func TestRecipientHandler_CreateRecipient_ZeroClientID_NoContext_ReturnsError(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	_, err := h.CreateRecipient(context.Background(), &prv1.CreateRecipientRequest{ClientId: 0})
	if err == nil {
		t.Error("expected error for zero client_id")
	}
}

func TestRecipientHandler_CreateRecipient_ServiceError(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{createErr: errors.New("duplicate")})
	_, err := h.CreateRecipient(context.Background(), &prv1.CreateRecipientRequest{ClientId: 5, Naziv: "X"})
	if err == nil {
		t.Error("expected error from service")
	}
}

func TestRecipientHandler_CreateRecipient_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.CreateRecipient(ctx, &prv1.CreateRecipientRequest{ClientId: 5})
	if err == nil {
		t.Error("expected error for zero clientID in claims")
	}
}

func TestRecipientHandler_CreateRecipient_WithClaims_MismatchClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.CreateRecipient(ctx, &prv1.CreateRecipientRequest{ClientId: 99})
	if err == nil {
		t.Error("expected error for mismatched client ID")
	}
}

func TestRecipientHandler_CreateRecipient_WithClaims_Match_Success(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{createResult: makeRecipient(2)})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.CreateRecipient(ctx, &prv1.CreateRecipientRequest{
		ClientId:   5,
		Naziv:      "Test",
		BrojRacuna: "000111000111000111",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// PaymentRecipientHandler.ListRecipients (gRPC)
// =====================

func TestRecipientHandler_ListRecipients_Success(t *testing.T) {
	list := []models.PaymentRecipient{*makeRecipient(1), *makeRecipient(2)}
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{listResult: list})
	resp, err := h.ListRecipients(context.Background(), &prv1.ListRecipientsRequest{ClientId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Recipients) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(resp.Recipients))
	}
}

func TestRecipientHandler_ListRecipients_ServiceError(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{listErr: errors.New("db error")})
	_, err := h.ListRecipients(context.Background(), &prv1.ListRecipientsRequest{ClientId: 5})
	if err == nil {
		t.Error("expected error")
	}
}

func TestRecipientHandler_ListRecipients_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.ListRecipients(ctx, &prv1.ListRecipientsRequest{ClientId: 5})
	if err == nil {
		t.Error("expected error for zero clientID in claims")
	}
}

func TestRecipientHandler_ListRecipients_WithClaims_MismatchClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.ListRecipients(ctx, &prv1.ListRecipientsRequest{ClientId: 99})
	if err == nil {
		t.Error("expected error for mismatched client ID")
	}
}

func TestRecipientHandler_ListRecipients_WithClaims_Match(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{listResult: []models.PaymentRecipient{*makeRecipient(1)}})
	ctx := ctxWithClaims(clientClaims(5))
	resp, err := h.ListRecipients(ctx, &prv1.ListRecipientsRequest{ClientId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Recipients) != 1 {
		t.Errorf("expected 1 recipient, got %d", len(resp.Recipients))
	}
}

// =====================
// PaymentRecipientHandler.UpdateRecipient (gRPC)
// =====================

func TestRecipientHandler_UpdateRecipient_Success(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{updateResult: makeRecipient(1)})
	resp, err := h.UpdateRecipient(context.Background(), &prv1.UpdateRecipientRequest{
		Id:       1,
		ClientId: 5,
		Naziv:    "Updated",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Recipient.Id != 1 {
		t.Errorf("expected ID=1, got %d", resp.Recipient.Id)
	}
}

func TestRecipientHandler_UpdateRecipient_ServiceError(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{updateErr: errors.New("not found")})
	_, err := h.UpdateRecipient(context.Background(), &prv1.UpdateRecipientRequest{Id: 99, ClientId: 5})
	if err == nil {
		t.Error("expected error")
	}
}

func TestRecipientHandler_UpdateRecipient_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.UpdateRecipient(ctx, &prv1.UpdateRecipientRequest{Id: 1, ClientId: 5})
	if err == nil {
		t.Error("expected error for zero clientID in claims")
	}
}

func TestRecipientHandler_UpdateRecipient_WithClaims_MismatchClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.UpdateRecipient(ctx, &prv1.UpdateRecipientRequest{Id: 1, ClientId: 99})
	if err == nil {
		t.Error("expected error for mismatched client ID")
	}
}

func TestRecipientHandler_UpdateRecipient_WithClaims_Match_Success(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{updateResult: makeRecipient(1)})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.UpdateRecipient(ctx, &prv1.UpdateRecipientRequest{Id: 1, ClientId: 5, Naziv: "Updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// PaymentRecipientHandler.DeleteRecipient (gRPC)
// =====================

func TestRecipientHandler_DeleteRecipient_Success(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	resp, err := h.DeleteRecipient(context.Background(), &prv1.DeleteRecipientRequest{Id: 1, ClientId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestRecipientHandler_DeleteRecipient_ServiceError(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{deleteErr: errors.New("not found")})
	_, err := h.DeleteRecipient(context.Background(), &prv1.DeleteRecipientRequest{Id: 99, ClientId: 5})
	if err == nil {
		t.Error("expected error")
	}
}

func TestRecipientHandler_DeleteRecipient_WithClaims_ZeroClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(&util.Claims{ClientID: 0})
	_, err := h.DeleteRecipient(ctx, &prv1.DeleteRecipientRequest{Id: 1, ClientId: 5})
	if err == nil {
		t.Error("expected error for zero clientID in claims")
	}
}

func TestRecipientHandler_DeleteRecipient_WithClaims_MismatchClientID_ReturnsDenied(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.DeleteRecipient(ctx, &prv1.DeleteRecipientRequest{Id: 1, ClientId: 99})
	if err == nil {
		t.Error("expected error for mismatched client ID")
	}
}

func TestRecipientHandler_DeleteRecipient_WithClaims_Match_Success(t *testing.T) {
	h := newRecipientGRPCHandler(&mockRecipientGRPCSvc{})
	ctx := ctxWithClaims(clientClaims(5))
	_, err := h.DeleteRecipient(ctx, &prv1.DeleteRecipientRequest{Id: 1, ClientId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =====================
// JWT-authenticated HTTP handler tests
// =====================

const testJWTSecret = "test-secret-key"

func makeTestJWT(claims *util.Claims) string {
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testJWTSecret))
	return signed
}

func testCfg() *config.Config {
	return &config.Config{JWTSecret: testJWTSecret}
}

func makeClientJWT(clientID uint) string {
	return makeTestJWT(&util.Claims{
		ClientID:    clientID,
		TokenSource: "client",
		TokenType:   "access",
		Permissions: []string{models.PermClientBasic},
	})
}

// =====================
// CreatePaymentHTTPHandler with auth
// =====================

func TestCreateHTTPHandler_WithAuth_AccountOwnershipCheck(t *testing.T) {
	h := &CreatePaymentHTTPHandler{
		svc: &mockCreateSvc{result: makePaymentModel(1)},
		cfg: testCfg(),
	}
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"000001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (nil db → ownership allowed), got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateHTTPHandler_WithAuth_RecipientOwnershipCheck(t *testing.T) {
	h := &CreatePaymentHTTPHandler{
		svc: &mockCreateSvc{result: makePaymentModel(2)},
		cfg: testCfg(),
	}
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"000001","iznos":100,"recipient_id":3}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (nil db → recipient ownership allowed), got %d: %s", w.Code, w.Body.String())
	}
}

// =====================
// PrenosHTTPHandler with auth
// =====================

func TestPrenosHandler_Create_WithAuth_ClaimsNotNil(t *testing.T) {
	h := &PrenosHTTPHandler{
		svc: &mockPrenosSvc{createResult: makePaymentModel(1)},
		cfg: testCfg(),
	}
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"000001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (nil db → allowed), got %d: %s", w.Code, w.Body.String())
	}
}

func TestPrenosHandler_Create_WithAuth_ServiceError(t *testing.T) {
	h := &PrenosHTTPHandler{
		svc: &mockPrenosSvc{createErr: errors.New("limit exceeded")},
		cfg: testCfg(),
	}
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_broj":"000001","iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPrenosHandler_Verify_WithAuth_ClaimsNotNil(t *testing.T) {
	h := &PrenosHTTPHandler{
		svc: &mockPrenosSvc{verifyResult: makePaymentModel(1)},
		cfg: testCfg(),
	}
	body := `{"verification_code":"ABC123"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos/1/verify", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (nil db → allowed), got %d: %s", w.Code, w.Body.String())
	}
}

func TestPrenosHandler_Verify_WithAuth_VerificationError_UnsupportedAccounts(t *testing.T) {
	verErr := &service.PaymentVerificationError{
		Code:    "unsupported_prenos_accounts",
		Message: "unsupported accounts",
	}
	h := &PrenosHTTPHandler{
		svc: &mockPrenosSvc{verifyErr: verErr},
		cfg: testCfg(),
	}
	body := `{"verification_code":"ABC"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/prenos/1/verify", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for unsupported_prenos_accounts, got %d", w.Code)
	}
}

// =====================
// PaymentMobileVerificationHandler with auth
// =====================

func TestMobileHandler_Approve_WithAuth_ClaimsNotNil_NilDB_Success(t *testing.T) {
	h := &PaymentMobileVerificationHandler{
		svc: &mockMobileSvc{approveResult: makePaymentModel(5)},
		cfg: testCfg(),
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/5/approve", nil)
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMobileHandler_Reject_WithAuth_ClaimsNotNil_NilDB_Success(t *testing.T) {
	h := &PaymentMobileVerificationHandler{
		svc: &mockMobileSvc{rejectResult: makePaymentModel(5)},
		cfg: testCfg(),
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payment/5/reject", nil)
	r.Header.Set("Authorization", "Bearer "+makeClientJWT(5))
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
