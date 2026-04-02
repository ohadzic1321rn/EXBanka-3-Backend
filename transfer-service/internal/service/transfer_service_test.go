package service_test

import (
	"errors"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/transfer-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/transfer-service/internal/service"
	"gorm.io/gorm"
)

// --- mocks ---

type mockAccountRepo struct {
	accounts      map[uint]*models.Account
	updatedID     uint
	updatedFields map[string]interface{}
	err           error
}

func (m *mockAccountRepo) FindByID(id uint) (*models.Account, error) {
	if m.err != nil {
		return nil, m.err
	}
	if a, ok := m.accounts[id]; ok {
		return a, nil
	}
	return nil, errors.New("account not found")
}

func (m *mockAccountRepo) FindByIDForUpdate(_ *gorm.DB, id uint) (*models.Account, error) {
	return m.FindByID(id)
}

func (m *mockAccountRepo) UpdateFields(id uint, fields map[string]interface{}) error {
	m.updatedID = id
	m.updatedFields = fields
	return nil
}

func (m *mockAccountRepo) UpdateFieldsTx(_ *gorm.DB, id uint, fields map[string]interface{}) error {
	return m.UpdateFields(id, fields)
}

func (m *mockAccountRepo) FindBankAccountByCurrency(currencyKod string) (*models.Account, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &models.Account{
		ID:                9000,
		Stanje:            10_000_000,
		RaspolozivoStanje: 10_000_000,
		Currency:          models.Currency{Kod: currencyKod},
	}, nil
}

func (m *mockAccountRepo) FindBankAccountByCurrencyForUpdate(_ *gorm.DB, currencyKod string) (*models.Account, error) {
	return m.FindBankAccountByCurrency(currencyKod)
}

type mockTransferRepo struct {
	created               *models.Transfer
	listByAccountResult   []models.Transfer
	listByAccountTotal    int64
	listByClientResult    []models.Transfer
	listByClientTotal     int64
	capturedAccountFilter models.TransferFilter
	capturedClientFilter  models.TransferFilter
}

func (m *mockTransferRepo) Create(t *models.Transfer) error {
	m.created = t
	return nil
}
func (m *mockTransferRepo) FindByID(id uint) (*models.Transfer, error) {
	if m.created != nil && m.created.ID == id {
		return m.created, nil
	}
	return nil, errors.New("not found")
}
func (m *mockTransferRepo) Save(t *models.Transfer) error { m.created = t; return nil }
func (m *mockTransferRepo) ListByAccountID(_ uint, filter models.TransferFilter) ([]models.Transfer, int64, error) {
	m.capturedAccountFilter = filter
	return m.listByAccountResult, m.listByAccountTotal, nil
}
func (m *mockTransferRepo) ListByClientID(_ uint, filter models.TransferFilter) ([]models.Transfer, int64, error) {
	m.capturedClientFilter = filter
	return m.listByClientResult, m.listByClientTotal, nil
}

type mockExchangeRateService struct {
	rate float64
	err  error
}

func (m *mockExchangeRateService) GetRate(from, to string) (float64, error) {
	return m.rate, m.err
}

func (m *mockExchangeRateService) GetSellRate(from, to string) (float64, error) {
	return m.rate, m.err
}

type mockNotificationSender struct {
	called   bool
	toEmail  string
	toName   string
	transfer *models.Transfer
	err      error
}

func (m *mockNotificationSender) SendTransferVerificationCode(toEmail, toName string, transfer *models.Transfer) error {
	m.called = true
	m.toEmail = toEmail
	m.toName = toName
	m.transfer = transfer
	return m.err
}

func rsdAccount(id uint, balance float64) *models.Account {
	clientID := uint(1)
	return &models.Account{
		ID: id, RaspolozivoStanje: balance, Stanje: balance,
		DnevniLimit: 100000, MesecniLimit: 1000000, CurrencyID: 1,
		ClientID: &clientID,
		Currency: models.Currency{ID: 1, Kod: "RSD"},
		Client:   &models.Client{ID: 1, Ime: "Ana", Prezime: "Jovic", Email: "ana@example.com"},
	}
}

// captureAccountRepo records all UpdateFields calls indexed by account ID.
type captureAccountRepo struct {
	accounts map[uint]*models.Account
	updates  map[uint]map[string]interface{}
}

func newCaptureRepo(accounts map[uint]*models.Account) *captureAccountRepo {
	return &captureAccountRepo{accounts: accounts, updates: make(map[uint]map[string]interface{})}
}

func (r *captureAccountRepo) FindByID(id uint) (*models.Account, error) {
	if a, ok := r.accounts[id]; ok {
		return a, nil
	}
	return nil, errors.New("account not found")
}

func (r *captureAccountRepo) FindByIDForUpdate(_ *gorm.DB, id uint) (*models.Account, error) {
	return r.FindByID(id)
}

func (r *captureAccountRepo) UpdateFields(id uint, fields map[string]interface{}) error {
	r.updates[id] = fields
	return nil
}

func (r *captureAccountRepo) UpdateFieldsTx(_ *gorm.DB, id uint, fields map[string]interface{}) error {
	return r.UpdateFields(id, fields)
}

func (r *captureAccountRepo) FindBankAccountByCurrency(currencyKod string) (*models.Account, error) {
	return &models.Account{
		ID:                9000,
		Stanje:            10_000_000,
		RaspolozivoStanje: 10_000_000,
		Currency:          models.Currency{Kod: currencyKod},
	}, nil
}

func (r *captureAccountRepo) FindBankAccountByCurrencyForUpdate(_ *gorm.DB, currencyKod string) (*models.Account, error) {
	return r.FindBankAccountByCurrency(currencyKod)
}

func eurAccount(id uint, balance float64) *models.Account {
	clientID := uint(1)
	return &models.Account{
		ID: id, RaspolozivoStanje: balance, Stanje: balance,
		DnevniLimit: 10000, MesecniLimit: 100000, CurrencyID: 2,
		ClientID: &clientID,
		Currency: models.Currency{ID: 2, Kod: "EUR"},
		Client:   &models.Client{ID: 1, Ime: "Ana", Prezime: "Jovic", Email: "ana@example.com"},
	}
}

// --- tests ---

func TestCreateTransfer_SameCurrency_Success(t *testing.T) {
	accountRepo := &mockAccountRepo{accounts: map[uint]*models.Account{
		1: rsdAccount(1, 5000),
		2: rsdAccount(2, 1000),
	}}
	transferRepo := &mockTransferRepo{}
	svc := service.NewTransferServiceWithRepos(accountRepo, transferRepo, &mockExchangeRateService{})

	tr, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1,
		RacunPrimaocaID:   2,
		Iznos:             1000,
		Svrha:             "Test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil transfer")
	}
	if tr.Iznos != 1000 {
		t.Errorf("expected Iznos=1000, got %f", tr.Iznos)
	}
	if tr.Status != "u_obradi" {
		t.Errorf("expected Status=u_obradi (pending verification), got %s", tr.Status)
	}
	if tr.Kurs != 1.0 {
		t.Errorf("expected Kurs=1.0 for same-currency, got %f", tr.Kurs)
	}
}

func TestCreateTransfer_CrossCurrency_AppliesExchangeRate(t *testing.T) {
	accountRepo := &mockAccountRepo{accounts: map[uint]*models.Account{
		1: eurAccount(1, 1000),
		2: rsdAccount(2, 0),
	}}
	transferRepo := &mockTransferRepo{}
	rateSvc := &mockExchangeRateService{rate: 117.0}
	svc := service.NewTransferServiceWithRepos(accountRepo, transferRepo, rateSvc)

	tr, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1,
		RacunPrimaocaID:   2,
		Iznos:             100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Kurs != 117.0 {
		t.Errorf("expected Kurs=117.0, got %f", tr.Kurs)
	}
	if tr.KonvertovaniIznos != 11700 {
		t.Errorf("expected KonvertovaniIznos=11700, got %f", tr.KonvertovaniIznos)
	}
	if tr.ValutaIznosa != "EUR" {
		t.Errorf("expected ValutaIznosa=EUR, got %s", tr.ValutaIznosa)
	}
}

func TestCreateTransfer_InsufficientBalance_ReturnsError(t *testing.T) {
	accountRepo := &mockAccountRepo{accounts: map[uint]*models.Account{
		1: rsdAccount(1, 500),
		2: rsdAccount(2, 0),
	}}
	svc := service.NewTransferServiceWithRepos(accountRepo, &mockTransferRepo{}, &mockExchangeRateService{})

	_, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 2, Iznos: 1000,
	})
	if err == nil {
		t.Fatal("expected insufficient balance error, got nil")
	}
}

func TestCreateTransfer_SameAccount_ReturnsError(t *testing.T) {
	accountRepo := &mockAccountRepo{accounts: map[uint]*models.Account{
		1: rsdAccount(1, 5000),
	}}
	svc := service.NewTransferServiceWithRepos(accountRepo, &mockTransferRepo{}, &mockExchangeRateService{})

	_, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 1, Iznos: 100,
	})
	if err == nil {
		t.Fatal("expected same-account error, got nil")
	}
}

func TestCreateTransfer_NegativeAmount_ReturnsError(t *testing.T) {
	svc := service.NewTransferServiceWithRepos(&mockAccountRepo{accounts: map[uint]*models.Account{}}, &mockTransferRepo{}, &mockExchangeRateService{})

	_, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 2, Iznos: -50,
	})
	if err == nil {
		t.Fatal("expected negative amount error, got nil")
	}
}

func TestCreateTransfer_DailyLimitExceeded_ReturnsError(t *testing.T) {
	clientID := uint(1)
	accountRepo := &mockAccountRepo{accounts: map[uint]*models.Account{
		1: {ID: 1, ClientID: &clientID, RaspolozivoStanje: 200000, Stanje: 200000, DnevniLimit: 100000, CurrencyID: 1, Currency: models.Currency{Kod: "RSD"}},
		2: rsdAccount(2, 0),
	}}
	svc := service.NewTransferServiceWithRepos(accountRepo, &mockTransferRepo{}, &mockExchangeRateService{})

	_, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 2, Iznos: 150000,
	})
	if err == nil {
		t.Fatal("expected daily limit error, got nil")
	}
}

// --- Verify flow tests ---

func TestCreateTransfer_SetsStatusUObradi(t *testing.T) {
	accountRepo := &mockAccountRepo{accounts: map[uint]*models.Account{
		1: rsdAccount(1, 5000),
		2: rsdAccount(2, 1000),
	}}
	transferRepo := &mockTransferRepo{}
	svc := service.NewTransferServiceWithRepos(accountRepo, transferRepo, &mockExchangeRateService{})

	tr, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 2, Iznos: 1000, Svrha: "Test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Status != "u_obradi" {
		t.Errorf("expected Status=u_obradi, got %s", tr.Status)
	}
}


func TestCreateTransfer_DoesNotUpdateSenderBalance(t *testing.T) {
	accountRepo := newCaptureRepo(map[uint]*models.Account{
		1: rsdAccount(1, 5000),
		2: rsdAccount(2, 0),
	})
	svc := service.NewTransferServiceWithRepos(accountRepo, &mockTransferRepo{}, &mockExchangeRateService{})

	_, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 2, Iznos: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, updated := accountRepo.updates[1]; updated {
		t.Error("expected sender balance NOT to be updated in CreateTransfer")
	}
}

// --- ListTransfersByAccount tests ---

func TestListTransfersByAccount_ReturnsTransfers(t *testing.T) {
	transfers := []models.Transfer{{ID: 1}, {ID: 2}}
	transferRepo := &mockTransferRepo{listByAccountResult: transfers, listByAccountTotal: 2}
	svc := service.NewTransferServiceWithRepos(&mockAccountRepo{accounts: map[uint]*models.Account{}}, transferRepo, &mockExchangeRateService{})

	result, total, err := svc.ListTransfersByAccount(5, models.TransferFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 transfers, got %d", len(result))
	}
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
}

func TestListTransfersByAccount_FilterPassedThrough(t *testing.T) {
	transferRepo := &mockTransferRepo{}
	svc := service.NewTransferServiceWithRepos(&mockAccountRepo{accounts: map[uint]*models.Account{}}, transferRepo, &mockExchangeRateService{})

	_, _, err := svc.ListTransfersByAccount(5, models.TransferFilter{Status: "uspesno", Page: 2, PageSize: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transferRepo.capturedAccountFilter.Status != "uspesno" {
		t.Errorf("expected Status filter=uspesno, got %q", transferRepo.capturedAccountFilter.Status)
	}
	if transferRepo.capturedAccountFilter.Page != 2 {
		t.Errorf("expected Page=2, got %d", transferRepo.capturedAccountFilter.Page)
	}
}

// --- ListTransfersByClient tests ---

func TestListTransfersByClient_ReturnsTransfers(t *testing.T) {
	transfers := []models.Transfer{{ID: 10}, {ID: 11}, {ID: 12}}
	transferRepo := &mockTransferRepo{listByClientResult: transfers, listByClientTotal: 3}
	svc := service.NewTransferServiceWithRepos(&mockAccountRepo{accounts: map[uint]*models.Account{}}, transferRepo, &mockExchangeRateService{})

	result, total, err := svc.ListTransfersByClient(7, models.TransferFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 transfers, got %d", len(result))
	}
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
}

func TestListTransfersByClient_PaginationPassedThrough(t *testing.T) {
	transferRepo := &mockTransferRepo{}
	svc := service.NewTransferServiceWithRepos(&mockAccountRepo{accounts: map[uint]*models.Account{}}, transferRepo, &mockExchangeRateService{})

	_, _, err := svc.ListTransfersByClient(7, models.TransferFilter{Page: 3, PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transferRepo.capturedClientFilter.Page != 3 {
		t.Errorf("expected Page=3, got %d", transferRepo.capturedClientFilter.Page)
	}
	if transferRepo.capturedClientFilter.PageSize != 20 {
		t.Errorf("expected PageSize=20, got %d", transferRepo.capturedClientFilter.PageSize)
	}
}

// --- DnevnaPotrosnja / MesecnaPotrosnja tests ---

func TestCreateTransfer_DailySpendingExceedsLimit_ReturnsError(t *testing.T) {
	clientID := uint(1)
	accountRepo := newCaptureRepo(map[uint]*models.Account{
		1: {
			ID: 1, RaspolozivoStanje: 50000, Stanje: 50000,
			DnevniLimit: 100000, MesecniLimit: 1000000,
			DnevnaPotrosnja: 90000, // already spent 90k today
			ClientID:        &clientID,
			CurrencyID:      1, Currency: models.Currency{Kod: "RSD"},
		},
		2: rsdAccount(2, 0),
	})
	svc := service.NewTransferServiceWithRepos(accountRepo, &mockTransferRepo{}, &mockExchangeRateService{})

	_, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 2, Iznos: 20000, // 90000+20000=110000 > 100000
	})
	if err == nil {
		t.Fatal("expected daily spending limit error, got nil")
	}
}

func TestCreateTransfer_MonthlySpendingExceedsLimit_ReturnsError(t *testing.T) {
	clientID := uint(1)
	accountRepo := newCaptureRepo(map[uint]*models.Account{
		1: {
			ID: 1, RaspolozivoStanje: 100000, Stanje: 100000,
			DnevniLimit: 500000, MesecniLimit: 1000000,
			MesecnaPotrosnja: 970000, // already spent 970k this month
			ClientID:         &clientID,
			CurrencyID:       1, Currency: models.Currency{Kod: "RSD"},
		},
		2: rsdAccount(2, 0),
	})
	svc := service.NewTransferServiceWithRepos(accountRepo, &mockTransferRepo{}, &mockExchangeRateService{})

	_, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1, RacunPrimaocaID: 2, Iznos: 50000, // 970000+50000=1020000 > 1000000
	})
	if err == nil {
		t.Fatal("expected monthly spending limit error, got nil")
	}
}

func TestApproveTransferMobile_DefaultModeReturnsPendingWithoutSettlement(t *testing.T) {
	accountRepo := newCaptureRepo(map[uint]*models.Account{
		1: rsdAccount(1, 5000),
		2: rsdAccount(2, 0),
	})
	transferRepo := &mockTransferRepo{}
	svc := service.NewTransferServiceWithRepos(accountRepo, transferRepo, &mockExchangeRateService{})

	created, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1,
		RacunPrimaocaID:   2,
		Iznos:             100,
		Svrha:             "Mobile code",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	transfer, _, _, err := svc.ApproveTransferMobile(created.ID, "")
	if err != nil {
		t.Fatalf("approve mobile failed: %v", err)
	}
	if transfer.Status != "u_obradi" {
		t.Fatalf("expected transfer to remain pending, got %s", transfer.Status)
	}
	if len(accountRepo.updates) != 0 {
		t.Fatal("expected no balance updates in default mode")
	}
}

func TestApproveTransferMobile_ConfirmExecutesSettlement(t *testing.T) {
	accountRepo := newCaptureRepo(map[uint]*models.Account{
		1: rsdAccount(1, 5000),
		2: rsdAccount(2, 0),
	})
	transferRepo := &mockTransferRepo{}
	svc := service.NewTransferServiceWithRepos(accountRepo, transferRepo, &mockExchangeRateService{})

	created, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1,
		RacunPrimaocaID:   2,
		Iznos:             120,
		Svrha:             "Mobile confirm",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	transfer, code, expiresAt, err := svc.ApproveTransferMobile(created.ID, "confirm")
	if err != nil {
		t.Fatalf("confirm mobile failed: %v", err)
	}
	if transfer.Status != "uspesno" {
		t.Fatalf("expected completed transfer, got %s", transfer.Status)
	}
	if code != "" || expiresAt != nil {
		t.Fatal("expected confirm mode not to return verification code or expiry")
	}
	if accountRepo.updates[1]["raspolozivo_stanje"].(float64) != 4880 {
		t.Fatalf("expected sender balance 4880, got %v", accountRepo.updates[1]["raspolozivo_stanje"])
	}
	if accountRepo.updates[2]["raspolozivo_stanje"].(float64) != 120 {
		t.Fatalf("expected receiver balance 120, got %v", accountRepo.updates[2]["raspolozivo_stanje"])
	}
}

func TestRejectTransfer_CancelsPendingTransfer(t *testing.T) {
	accountRepo := newCaptureRepo(map[uint]*models.Account{
		1: rsdAccount(1, 5000),
		2: rsdAccount(2, 0),
	})
	transferRepo := &mockTransferRepo{}
	svc := service.NewTransferServiceWithRepos(accountRepo, transferRepo, &mockExchangeRateService{})

	created, err := svc.CreateTransfer(service.CreateTransferInput{
		RacunPosiljaocaID: 1,
		RacunPrimaocaID:   2,
		Iznos:             80,
		Svrha:             "Mobile reject",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	rejected, err := svc.RejectTransfer(created.ID)
	if err != nil {
		t.Fatalf("reject mobile failed: %v", err)
	}
	if rejected.Status != "stornirano" {
		t.Fatalf("expected cancelled transfer, got %s", rejected.Status)
	}
	if len(accountRepo.updates) != 0 {
		t.Fatal("expected reject not to touch balances")
	}
}

