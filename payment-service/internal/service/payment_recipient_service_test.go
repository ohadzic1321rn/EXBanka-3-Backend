package service_test

import (
	"errors"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/service"
)

// --- mock repo ---

type mockRecipientRepo struct {
	recipients map[uint]*models.PaymentRecipient
	nextID     uint
	createErr  error
	updateErr  error
	deleteErr  error
	deleted    uint
}

func newMockRepo() *mockRecipientRepo {
	return &mockRecipientRepo{
		recipients: make(map[uint]*models.PaymentRecipient),
		nextID:     1,
	}
}

func (m *mockRecipientRepo) Create(r *models.PaymentRecipient) error {
	if m.createErr != nil {
		return m.createErr
	}
	r.ID = m.nextID
	m.nextID++
	m.recipients[r.ID] = r
	return nil
}

func (m *mockRecipientRepo) FindByID(id uint) (*models.PaymentRecipient, error) {
	if r, ok := m.recipients[id]; ok {
		return r, nil
	}
	return nil, errors.New("record not found")
}

func (m *mockRecipientRepo) ListByClientID(clientID uint) ([]models.PaymentRecipient, error) {
	var result []models.PaymentRecipient
	for _, r := range m.recipients {
		if r.ClientID == clientID {
			result = append(result, *r)
		}
	}
	return result, nil
}

func (m *mockRecipientRepo) Update(r *models.PaymentRecipient) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.recipients[r.ID] = r
	return nil
}

func (m *mockRecipientRepo) Delete(id uint) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = id
	delete(m.recipients, id)
	return nil
}

// validAccountNumber returns a valid 18-digit account number accepted by ValidateAccountNumber.
// The payment-service validator requires: 18 digits, bank code (first 3) in {111,222,333,444}.
func validAccountNumber() string {
	return "111000000000000000"
}

// --- tests ---

func TestCreateRecipient_Success(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewPaymentRecipientServiceWithRepo(repo)

	r, err := svc.CreateRecipient(service.CreateRecipientInput{
		ClientID:   1,
		Naziv:      "Petar Petrovic",
		BrojRacuna: validAccountNumber(),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil recipient")
	}
	if r.Naziv != "Petar Petrovic" {
		t.Errorf("expected Naziv=Petar Petrovic, got %s", r.Naziv)
	}
	if r.ClientID != 1 {
		t.Errorf("expected ClientID=1, got %d", r.ClientID)
	}
}

func TestCreateRecipient_InvalidAccountNumber_ReturnsError(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewPaymentRecipientServiceWithRepo(repo)

	_, err := svc.CreateRecipient(service.CreateRecipientInput{
		ClientID:   1,
		Naziv:      "Test",
		BrojRacuna: "123456789",
	})

	if err == nil {
		t.Fatal("expected error for invalid account number, got nil")
	}
}

func TestListRecipientsByClient_ReturnsOnlyClientRecipients(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewPaymentRecipientServiceWithRepo(repo)

	// Create recipients for two different clients
	svc.CreateRecipient(service.CreateRecipientInput{ClientID: 1, Naziv: "A", BrojRacuna: validAccountNumber()})
	svc.CreateRecipient(service.CreateRecipientInput{ClientID: 1, Naziv: "B", BrojRacuna: validAccountNumber()})
	svc.CreateRecipient(service.CreateRecipientInput{ClientID: 2, Naziv: "C", BrojRacuna: validAccountNumber()})

	result, err := svc.ListRecipientsByClient(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 recipients for client 1, got %d", len(result))
	}
	for _, r := range result {
		if r.ClientID != 1 {
			t.Errorf("got recipient belonging to client %d, expected only client 1", r.ClientID)
		}
	}
}

func TestUpdateRecipient_WrongClient_ReturnsError(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewPaymentRecipientServiceWithRepo(repo)

	// Create recipient owned by client 1
	created, _ := svc.CreateRecipient(service.CreateRecipientInput{
		ClientID:   1,
		Naziv:      "Owner",
		BrojRacuna: validAccountNumber(),
	})

	// Try to update as client 2
	_, err := svc.UpdateRecipient(created.ID, 2, service.UpdateRecipientInput{Naziv: "Attacker"})

	if err == nil {
		t.Fatal("expected ownership error, got nil")
	}
}

func TestDeleteRecipient_Success(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewPaymentRecipientServiceWithRepo(repo)

	created, _ := svc.CreateRecipient(service.CreateRecipientInput{
		ClientID:   1,
		Naziv:      "To Delete",
		BrojRacuna: validAccountNumber(),
	})

	err := svc.DeleteRecipient(created.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's gone
	remaining, _ := svc.ListRecipientsByClient(1)
	if len(remaining) != 0 {
		t.Errorf("expected 0 recipients after delete, got %d", len(remaining))
	}
}
