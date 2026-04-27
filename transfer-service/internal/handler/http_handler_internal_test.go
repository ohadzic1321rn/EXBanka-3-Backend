package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/transfer-service/internal/config"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/transfer-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/transfer-service/internal/service"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/transfer-service/internal/util"
)

// --- mock HTTP service ---

type mockHTTPSvc struct {
	createResult    *models.Transfer
	createErr       error
	previewResult   *service.TransferPreview
	previewErr      error
	byClientResult  []models.Transfer
	byClientTotal   int64
	byAccountResult []models.Transfer
	byAccountTotal  int64
	listErr         error
}

func (m *mockHTTPSvc) CreateAndSettleTransfer(_ service.CreateTransferInput) (*models.Transfer, error) {
	return m.createResult, m.createErr
}

func (m *mockHTTPSvc) PreviewTransfer(_ service.CreateTransferInput) (*service.TransferPreview, error) {
	return m.previewResult, m.previewErr
}

func (m *mockHTTPSvc) ListTransfersByAccount(_ uint, _ models.TransferFilter) ([]models.Transfer, int64, error) {
	return m.byAccountResult, m.byAccountTotal, m.listErr
}

func (m *mockHTTPSvc) ListTransfersByClient(_ uint, _ models.TransferFilter) ([]models.Transfer, int64, error) {
	return m.byClientResult, m.byClientTotal, m.listErr
}

// --- mock mobile verification service ---

type mockMobileSvc struct {
	approveTransfer *models.Transfer
	approveErr      error
	rejectTransfer  *models.Transfer
	rejectErr       error
}

func (m *mockMobileSvc) ApproveTransferMobile(_ uint, _ string) (*models.Transfer, string, *time.Time, error) {
	return m.approveTransfer, "", nil, m.approveErr
}

func (m *mockMobileSvc) RejectTransfer(_ uint) (*models.Transfer, error) {
	return m.rejectTransfer, m.rejectErr
}

// --- helpers ---

func makeTestTransfer(id uint) *models.Transfer {
	return &models.Transfer{
		ID:                id,
		RacunPosiljaocaID: 1,
		RacunPrimaocaID:   2,
		Iznos:             100,
		ValutaIznosa:      "RSD",
		KonvertovaniIznos: 100,
		Kurs:              1.0,
		Status:            "uspesno",
	}
}

func newTestHTTPHandler(svc transferHTTPService) *TransferHTTPHandler {
	return &TransferHTTPHandler{svc: svc}
}

func newTestMobileHandler(svc transferMobileVerificationService) *TransferMobileVerificationHandler {
	return &TransferMobileVerificationHandler{svc: svc}
}

// =====================
// writeAuthError
// =====================

func TestWriteAuthError_SetsStatusAndBody(t *testing.T) {
	w := httptest.NewRecorder()
	writeAuthError(w, http.StatusUnauthorized, "missing token")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), "missing token") {
		t.Errorf("body missing message: %s", w.Body.String())
	}
}

// =====================
// parseHTTPClaims
// =====================

func TestParseHTTPClaims_NilConfig_ReturnsNilTrue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	claims, ok := parseHTTPClaims(w, r, nil)
	if !ok {
		t.Error("expected ok=true with nil config")
	}
	if claims != nil {
		t.Error("expected nil claims with nil config")
	}
}

func TestParseHTTPClaims_MissingAuthHeader_Returns401(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	_, ok := parseHTTPClaims(w, r, cfg)
	if ok {
		t.Error("expected ok=false for missing auth header")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestParseHTTPClaims_InvalidBearerPrefix_Returns401(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Basic abc123")
	w := httptest.NewRecorder()
	_, ok := parseHTTPClaims(w, r, cfg)
	if ok {
		t.Error("expected ok=false for non-bearer scheme")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
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

// =====================
// requireClientPermissionHTTP
// =====================

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
		t.Error("expected false for employee token source")
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

func TestRequireClientPermissionHTTP_HasPermission_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 1, Permissions: []string{models.PermClientBasic}}
	if !requireClientPermissionHTTP(w, claims, models.PermClientBasic) {
		t.Error("expected true when client has permission")
	}
}

func TestRequireClientPermissionHTTP_MissingPermission_ReturnsFalse(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 1, Permissions: []string{}}
	if requireClientPermissionHTTP(w, claims, models.PermClientBasic) {
		t.Error("expected false when permission is missing")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// =====================
// requireClientBasicHTTP
// =====================

func TestRequireClientBasicHTTP_MatchingClientID_ReturnsTrue(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 5, Permissions: []string{models.PermClientBasic}}
	if !requireClientBasicHTTP(w, claims, 5) {
		t.Error("expected true for matching client ID")
	}
}

func TestRequireClientBasicHTTP_MismatchedClientID_ReturnsFalse(t *testing.T) {
	w := httptest.NewRecorder()
	claims := &util.Claims{TokenSource: "client", ClientID: 5, Permissions: []string{models.PermClientBasic}}
	if requireClientBasicHTTP(w, claims, 99) {
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

// =====================
// parsePositiveInt
// =====================

func TestParsePositiveInt(t *testing.T) {
	cases := []struct {
		input    string
		fallback int
		want     int
	}{
		{"5", 1, 5},
		{"0", 1, 1},
		{"-1", 1, 1},
		{"abc", 1, 1},
		{"", 20, 20},
		{"100", 20, 100},
		{" 10 ", 1, 10},
	}
	for _, c := range cases {
		got := parsePositiveInt(c.input, c.fallback)
		if got != c.want {
			t.Errorf("parsePositiveInt(%q, %d) = %d, want %d", c.input, c.fallback, got, c.want)
		}
	}
}

// =====================
// extractPathUint
// =====================

func TestExtractPathUint_ValidPath(t *testing.T) {
	got, err := extractPathUint("/api/v1/transfers/client/42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestExtractPathUint_TooShortPath_ReturnsError(t *testing.T) {
	if _, err := extractPathUint("/api/v1/short"); err == nil {
		t.Error("expected error for short path")
	}
}

func TestExtractPathUint_NonNumericID_ReturnsError(t *testing.T) {
	if _, err := extractPathUint("/api/v1/transfers/client/notanumber"); err == nil {
		t.Error("expected error for non-numeric id")
	}
}

// =====================
// uintToString
// =====================

func TestUintToString(t *testing.T) {
	if got := uintToString(123); got != "123" {
		t.Errorf("expected '123', got '%s'", got)
	}
	if got := uintToString(0); got != "0" {
		t.Errorf("expected '0', got '%s'", got)
	}
}

// =====================
// decodeTransferInput
// =====================

func TestDecodeTransferInput_SnakeCase(t *testing.T) {
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_id":2,"iznos":500,"svrha":"test"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	input, err := decodeTransferInput(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.RacunPosiljaocaID != 1 || input.RacunPrimaocaID != 2 || input.Iznos != 500 || input.Svrha != "test" {
		t.Errorf("unexpected input: %+v", input)
	}
}

func TestDecodeTransferInput_CamelCase(t *testing.T) {
	body := `{"racunPosiljaocaId":3,"racunPrimaocaId":4,"iznos":200}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	input, err := decodeTransferInput(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.RacunPosiljaocaID != 3 || input.RacunPrimaocaID != 4 {
		t.Errorf("unexpected input: %+v", input)
	}
}

func TestDecodeTransferInput_InvalidJSON_ReturnsError(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	if _, err := decodeTransferInput(r); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// =====================
// toTransferHTTPJSON
// =====================

func TestToTransferHTTPJSON(t *testing.T) {
	tr := makeTestTransfer(7)
	j := toTransferHTTPJSON(tr)
	if j.ID != "7" {
		t.Errorf("expected ID='7', got '%s'", j.ID)
	}
	if j.Iznos != tr.Iznos {
		t.Errorf("expected Iznos=%f, got %f", tr.Iznos, j.Iznos)
	}
	if j.Status != tr.Status {
		t.Errorf("expected Status=%s, got %s", tr.Status, j.Status)
	}
	if j.RacunPosiljaocaID != "1" || j.RacunPrimaocaID != "2" {
		t.Errorf("unexpected account IDs: %s, %s", j.RacunPosiljaocaID, j.RacunPrimaocaID)
	}
}

// =====================
// parseHTTPTransferFilter
// =====================

func TestParseHTTPTransferFilter_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/transfers", nil)
	f := parseHTTPTransferFilter(r)
	if f.Page != 1 {
		t.Errorf("expected page=1, got %d", f.Page)
	}
	if f.PageSize != 20 {
		t.Errorf("expected pageSize=20, got %d", f.PageSize)
	}
	if f.MinAmount != nil || f.MaxAmount != nil || f.DateFrom != nil || f.DateTo != nil {
		t.Error("expected nil optional filters")
	}
}

func TestParseHTTPTransferFilter_WithParams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/transfers?page=3&page_size=10&status=uspesno&min_amount=100&max_amount=500", nil)
	f := parseHTTPTransferFilter(r)
	if f.Page != 3 {
		t.Errorf("expected page=3, got %d", f.Page)
	}
	if f.PageSize != 10 {
		t.Errorf("expected pageSize=10, got %d", f.PageSize)
	}
	if f.Status != "uspesno" {
		t.Errorf("expected status=uspesno, got %s", f.Status)
	}
	if f.MinAmount == nil || *f.MinAmount != 100 {
		t.Error("expected minAmount=100")
	}
	if f.MaxAmount == nil || *f.MaxAmount != 500 {
		t.Error("expected maxAmount=500")
	}
}

func TestParseHTTPTransferFilter_WithDates(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/transfers?date_from=2025-01-01T00:00:00Z&date_to=2025-12-31T23:59:59Z", nil)
	f := parseHTTPTransferFilter(r)
	if f.DateFrom == nil {
		t.Error("expected DateFrom to be set")
	}
	if f.DateTo == nil {
		t.Error("expected DateTo to be set")
	}
}

func TestParseHTTPTransferFilter_InvalidAmounts_IgnoredGracefully(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/transfers?min_amount=notanumber&max_amount=alsonotanumber", nil)
	f := parseHTTPTransferFilter(r)
	if f.MinAmount != nil {
		t.Error("expected MinAmount nil for invalid value")
	}
	if f.MaxAmount != nil {
		t.Error("expected MaxAmount nil for invalid value")
	}
}

// =====================
// writeJSON
// =====================

func TestWriteJSON_SetsStatusAndBody(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]interface{}{"key": "value"})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("expected key=value, got %v", body["key"])
	}
}

// =====================
// writeTransferJSON
// =====================

func TestWriteTransferJSON_SetsStatusAndContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeTransferJSON(w, http.StatusCreated, map[string]interface{}{"message": "ok"})
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

// =====================
// extractTransferID
// =====================

func TestExtractTransferID_ValidPath(t *testing.T) {
	id, err := extractTransferID("/api/v1/transfer/99/approve")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 99 {
		t.Errorf("expected 99, got %d", id)
	}
}

func TestExtractTransferID_TooShortPath_ReturnsError(t *testing.T) {
	if _, err := extractTransferID("/api/v1"); err == nil {
		t.Error("expected error for short path")
	}
}

func TestExtractTransferID_NonNumericID_ReturnsError(t *testing.T) {
	if _, err := extractTransferID("/api/v1/transfer/notanumber/approve"); err == nil {
		t.Error("expected error for non-numeric ID")
	}
}

// =====================
// TransferHTTPHandler.Create
// =====================

func TestHTTPCreate_MissingAccountIDs_Returns400(t *testing.T) {
	h := newTestHTTPHandler(&mockHTTPSvc{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(`{"iznos":100}`))
	w := httptest.NewRecorder()
	h.Create(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHTTPCreate_InvalidJSON_Returns400(t *testing.T) {
	h := newTestHTTPHandler(&mockHTTPSvc{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	h.Create(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHTTPCreate_ServiceError_Returns400(t *testing.T) {
	svc := &mockHTTPSvc{createErr: errors.New("insufficient funds")}
	h := newTestHTTPHandler(svc)
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_id":2,"iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Create(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHTTPCreate_Success_Returns200(t *testing.T) {
	svc := &mockHTTPSvc{createResult: makeTestTransfer(1)}
	h := newTestHTTPHandler(svc)
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_id":2,"iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Create(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// =====================
// TransferHTTPHandler.Preview
// =====================

func TestHTTPPreview_MissingAccountIDs_Returns400(t *testing.T) {
	h := newTestHTTPHandler(&mockHTTPSvc{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers/preview", strings.NewReader(`{"iznos":100}`))
	w := httptest.NewRecorder()
	h.Preview(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHTTPPreview_ServiceError_Returns400(t *testing.T) {
	svc := &mockHTTPSvc{previewErr: errors.New("exchange service unavailable")}
	h := newTestHTTPHandler(svc)
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_id":2,"iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers/preview", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Preview(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHTTPPreview_Success_Returns200(t *testing.T) {
	svc := &mockHTTPSvc{previewResult: &service.TransferPreview{Iznos: 100, ValutaIznosa: "RSD"}}
	h := newTestHTTPHandler(svc)
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_id":2,"iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers/preview", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Preview(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// =====================
// TransferHTTPHandler.ListByClient
// =====================

func TestHTTPListByClient_BadPath_Returns400(t *testing.T) {
	h := newTestHTTPHandler(&mockHTTPSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/short", nil)
	w := httptest.NewRecorder()
	h.ListByClient(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHTTPListByClient_ServiceError_Returns500(t *testing.T) {
	svc := &mockHTTPSvc{listErr: errors.New("db error")}
	h := newTestHTTPHandler(svc)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/client/5", nil)
	w := httptest.NewRecorder()
	h.ListByClient(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHTTPListByClient_Success_Returns200(t *testing.T) {
	svc := &mockHTTPSvc{byClientResult: []models.Transfer{*makeTestTransfer(1)}, byClientTotal: 1}
	h := newTestHTTPHandler(svc)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/client/5", nil)
	w := httptest.NewRecorder()
	h.ListByClient(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// =====================
// TransferHTTPHandler.ListByAccount
// =====================

func TestHTTPListByAccount_BadPath_Returns400(t *testing.T) {
	h := newTestHTTPHandler(&mockHTTPSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/short", nil)
	w := httptest.NewRecorder()
	h.ListByAccount(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHTTPListByAccount_ServiceError_Returns500(t *testing.T) {
	svc := &mockHTTPSvc{listErr: errors.New("db error")}
	h := newTestHTTPHandler(svc)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/account/5", nil)
	w := httptest.NewRecorder()
	h.ListByAccount(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHTTPListByAccount_Success_Returns200(t *testing.T) {
	svc := &mockHTTPSvc{byAccountResult: []models.Transfer{*makeTestTransfer(2)}, byAccountTotal: 1}
	h := newTestHTTPHandler(svc)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/account/5", nil)
	w := httptest.NewRecorder()
	h.ListByAccount(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// =====================
// TransferMobileVerificationHandler.Approve
// =====================

func TestMobileApprove_WrongMethod_Returns405(t *testing.T) {
	h := newTestMobileHandler(&mockMobileSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfer/1/approve", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestMobileApprove_BadPath_Returns400(t *testing.T) {
	h := newTestMobileHandler(&mockMobileSvc{})
	r := httptest.NewRequest(http.MethodPost, "/bad", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestMobileApprove_Success_Returns200(t *testing.T) {
	svc := &mockMobileSvc{approveTransfer: makeTestTransfer(5)}
	h := newTestMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfer/5/approve", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMobileApprove_WithModeBody_Success(t *testing.T) {
	svc := &mockMobileSvc{approveTransfer: makeTestTransfer(5)}
	h := newTestMobileHandler(svc)
	body := `{"mode":"2fa"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfer/5/approve", strings.NewReader(body))
	r.ContentLength = int64(len(body))
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMobileApprove_ServiceError_Returns400(t *testing.T) {
	svc := &mockMobileSvc{approveErr: errors.New("transfer not found")}
	h := newTestMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfer/5/approve", nil)
	w := httptest.NewRecorder()
	h.Approve(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// =====================
// TransferMobileVerificationHandler.Reject
// =====================

func TestMobileReject_WrongMethod_Returns405(t *testing.T) {
	h := newTestMobileHandler(&mockMobileSvc{})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfer/1/reject", nil)
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestMobileReject_Success_Returns200(t *testing.T) {
	svc := &mockMobileSvc{rejectTransfer: makeTestTransfer(3)}
	h := newTestMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfer/3/reject", nil)
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMobileReject_ServiceError_Returns400(t *testing.T) {
	svc := &mockMobileSvc{rejectErr: errors.New("transfer already settled")}
	h := newTestMobileHandler(svc)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfer/3/reject", nil)
	w := httptest.NewRecorder()
	h.Reject(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// =====================
// writeVerificationError
// =====================

func TestWriteVerificationError_TransferNotPending_Returns409(t *testing.T) {
	h := newTestMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	verErr := &service.TransferVerificationError{
		Code:    "transfer_not_pending",
		Message: "transfer is not pending",
		Status:  "uspesno",
	}
	h.writeVerificationError(w, verErr)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["code"] != "transfer_not_pending" {
		t.Errorf("expected code=transfer_not_pending, got %v", body["code"])
	}
}

func TestWriteVerificationError_WithAttemptsRemaining_IncludesField(t *testing.T) {
	h := newTestMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	verErr := &service.TransferVerificationError{
		Code:              "insufficient_balance",
		Message:           "not enough balance",
		AttemptsRemaining: 2,
	}
	h.writeVerificationError(w, verErr)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if _, ok := body["attemptsRemaining"]; !ok {
		t.Error("expected attemptsRemaining in response")
	}
}

func TestWriteVerificationError_UnsupportedMode_SetsCode(t *testing.T) {
	h := newTestMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	h.writeVerificationError(w, errors.New("unsupported approval mode"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["code"] != "unsupported_approval_mode" {
		t.Errorf("expected unsupported_approval_mode, got %v", body["code"])
	}
}

func TestWriteVerificationError_GenericError_Returns400(t *testing.T) {
	h := newTestMobileHandler(&mockMobileSvc{})
	w := httptest.NewRecorder()
	h.writeVerificationError(w, errors.New("something went wrong"))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// =====================
// CombinedTransferHandler routing
// =====================

func TestCombinedHandler_RoutesToCreate(t *testing.T) {
	httpSvc := &mockHTTPSvc{createResult: makeTestTransfer(1)}
	combined := NewCombinedTransferHandler(newTestHTTPHandler(httpSvc), newTestMobileHandler(&mockMobileSvc{}), http.NotFoundHandler())
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_id":2,"iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers", strings.NewReader(body))
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_RoutesToPreview(t *testing.T) {
	httpSvc := &mockHTTPSvc{previewResult: &service.TransferPreview{Iznos: 100}}
	combined := NewCombinedTransferHandler(newTestHTTPHandler(httpSvc), newTestMobileHandler(&mockMobileSvc{}), http.NotFoundHandler())
	body := `{"racun_posiljaoca_id":1,"racun_primaoca_id":2,"iznos":100}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfers/preview", strings.NewReader(body))
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_RoutesToListByClient(t *testing.T) {
	combined := NewCombinedTransferHandler(newTestHTTPHandler(&mockHTTPSvc{}), newTestMobileHandler(&mockMobileSvc{}), http.NotFoundHandler())
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/client/5", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_RoutesToListByAccount(t *testing.T) {
	combined := NewCombinedTransferHandler(newTestHTTPHandler(&mockHTTPSvc{}), newTestMobileHandler(&mockMobileSvc{}), http.NotFoundHandler())
	r := httptest.NewRequest(http.MethodGet, "/api/v1/transfers/account/5", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_RoutesToApprove(t *testing.T) {
	mobileSvc := &mockMobileSvc{approveTransfer: makeTestTransfer(1)}
	combined := NewCombinedTransferHandler(newTestHTTPHandler(&mockHTTPSvc{}), newTestMobileHandler(mobileSvc), http.NotFoundHandler())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfer/1/approve", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_RoutesToReject(t *testing.T) {
	mobileSvc := &mockMobileSvc{rejectTransfer: makeTestTransfer(1)}
	combined := NewCombinedTransferHandler(newTestHTTPHandler(&mockHTTPSvc{}), newTestMobileHandler(mobileSvc), http.NotFoundHandler())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/transfer/1/reject", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCombinedHandler_UnknownRoute_UsesBackend(t *testing.T) {
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	combined := NewCombinedTransferHandler(newTestHTTPHandler(&mockHTTPSvc{}), newTestMobileHandler(&mockMobileSvc{}), fallback)
	r := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()
	combined.ServeHTTP(w, r)
	if w.Code != http.StatusTeapot {
		t.Errorf("expected 418, got %d", w.Code)
	}
}
