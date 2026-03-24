package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/handler"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/service"
)

// --- mock service ---

type mockLoanService struct {
	loan         *models.Loan
	loans        []models.Loan
	installments []models.LoanInstallment
	err          error
}

func (m *mockLoanService) RequestLoan(_ service.CreateLoanInput) (*models.Loan, error) {
	return m.loan, m.err
}
func (m *mockLoanService) ApproveLoan(_, _ uint) (*models.Loan, error)  { return m.loan, m.err }
func (m *mockLoanService) RejectLoan(_, _ uint) (*models.Loan, error)   { return m.loan, m.err }
func (m *mockLoanService) ListByClient(_ uint) ([]models.Loan, error)   { return m.loans, m.err }
func (m *mockLoanService) GetByID(_ uint) (*models.Loan, error)          { return m.loan, m.err }
func (m *mockLoanService) ListInstallments(_ uint) ([]models.LoanInstallment, error) {
	return m.installments, m.err
}
func (m *mockLoanService) ListRequests() ([]models.Loan, error) { return m.loans, m.err }

// --- helpers ---

func newHandler(svc handler.LoanServiceInterface) http.Handler {
	return handler.NewLoanHandler(svc)
}

func postJSON(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func getRequest(h http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// --- RequestLoan tests ---

func TestLoanHandler_RequestLoan_Returns201(t *testing.T) {
	svc := &mockLoanService{loan: &models.Loan{ID: 1, Status: "zahtev"}}
	h := newHandler(svc)

	w := postJSON(t, h, "/api/v1/loans/request", map[string]any{
		"vrsta": "gotovinski", "iznos": 100000, "period": 12,
		"tip_kamate": "fiksna", "client_id": 1, "currency_id": 1,
	})

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoanHandler_RequestLoan_InvalidJSON_Returns400(t *testing.T) {
	h := newHandler(&mockLoanService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/request", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLoanHandler_RequestLoan_ServiceError_Returns400(t *testing.T) {
	svc := &mockLoanService{err: service.ErrInvalidInput}
	h := newHandler(svc)

	w := postJSON(t, h, "/api/v1/loans/request", map[string]any{
		"vrsta": "bad", "iznos": -1,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- ListByClient tests ---

func TestLoanHandler_ListByClient_Returns200(t *testing.T) {
	svc := &mockLoanService{loans: []models.Loan{{ID: 1}, {ID: 2}}}
	w := getRequest(newHandler(svc), "/api/v1/loans/client/5")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 loans, got %d", len(resp))
	}
}

func TestLoanHandler_ListByClient_InvalidID_Returns400(t *testing.T) {
	w := getRequest(newHandler(&mockLoanService{}), "/api/v1/loans/client/abc")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- GetByID tests ---

func TestLoanHandler_GetByID_Returns200(t *testing.T) {
	svc := &mockLoanService{loan: &models.Loan{ID: 3, Status: "aktivan"}}
	w := getRequest(newHandler(svc), "/api/v1/loans/3")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "aktivan" {
		t.Errorf("expected status=aktivan, got %v", resp["status"])
	}
}

func TestLoanHandler_GetByID_InvalidID_Returns400(t *testing.T) {
	w := getRequest(newHandler(&mockLoanService{}), "/api/v1/loans/xyz")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- ListInstallments tests ---

func TestLoanHandler_ListInstallments_Returns200(t *testing.T) {
	svc := &mockLoanService{installments: []models.LoanInstallment{{ID: 1, RedniBroj: 1}}}
	w := getRequest(newHandler(svc), "/api/v1/loans/1/installments")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 installment, got %d", len(resp))
	}
}

// --- ApproveLoan tests ---

func TestLoanHandler_ApproveLoan_Returns200(t *testing.T) {
	svc := &mockLoanService{loan: &models.Loan{ID: 1, Status: "aktivan"}}
	w := postJSON(t, newHandler(svc), "/api/v1/loans/1/approve", map[string]any{"zaposleni_id": 42})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoanHandler_ApproveLoan_InvalidID_Returns400(t *testing.T) {
	w := postJSON(t, newHandler(&mockLoanService{}), "/api/v1/loans/abc/approve", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- RejectLoan tests ---

func TestLoanHandler_RejectLoan_Returns200(t *testing.T) {
	svc := &mockLoanService{loan: &models.Loan{ID: 1, Status: "odbijen"}}
	w := postJSON(t, newHandler(svc), "/api/v1/loans/1/reject", map[string]any{"zaposleni_id": 42})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- ListRequests tests ---

func TestLoanHandler_ListRequests_Returns200(t *testing.T) {
	svc := &mockLoanService{loans: []models.Loan{{ID: 1, Status: "zahtev"}}}
	w := getRequest(newHandler(svc), "/api/v1/loans/requests")

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 request, got %d", len(resp))
	}
}

// --- ResponseBody tests ---

func TestLoanHandler_RequestLoan_ReturnsLoanJSON(t *testing.T) {
	svc := &mockLoanService{loan: &models.Loan{ID: 7, Status: "zahtev", Iznos: 50000}}
	h := newHandler(svc)

	w := postJSON(t, h, "/api/v1/loans/request", map[string]any{
		"vrsta": "auto", "iznos": 50000, "period": 24,
		"tip_kamate": "fiksna", "client_id": 1, "currency_id": 1,
	})

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "zahtev" {
		t.Errorf("expected status=zahtev in response, got %v", resp["status"])
	}
}
