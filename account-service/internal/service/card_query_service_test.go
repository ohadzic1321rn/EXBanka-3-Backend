package service_test

import (
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/account-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/account-service/internal/service"
)

// reuses mockCardRepo and mockAcctRepoForCard from card_service_test.go

// extended mockCardRepo that can return preset cards for list/find
type listCardRepo struct {
	mockCardRepo
	cards  []models.Card
	findFn func(id uint) (*models.Card, error)
}

func (r *listCardRepo) ListByAccountID(_ uint) ([]models.Card, error) { return r.cards, nil }
func (r *listCardRepo) ListByClientID(_ uint) ([]models.Card, error)  { return r.cards, nil }
func (r *listCardRepo) FindByID(id uint) (*models.Card, error) {
	if r.findFn != nil {
		return r.findFn(id)
	}
	for i := range r.cards {
		if r.cards[i].ID == id {
			return &r.cards[i], nil
		}
	}
	return nil, nil
}

func newListCardSvc(cards []models.Card) *service.CardService {
	cr := &listCardRepo{cards: cards}
	return service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)
}

// --- ListByAccount ---

func TestListByAccount_ReturnsAllCards(t *testing.T) {
	cards := []models.Card{
		{ID: 1, AccountID: 10, Status: "aktivna"},
		{ID: 2, AccountID: 10, Status: "blokirana"},
	}
	svc := newListCardSvc(cards)
	result, err := svc.ListByAccount(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 cards, got %d", len(result))
	}
}

func TestListByAccount_EmptyReturnsEmptySlice(t *testing.T) {
	svc := newListCardSvc(nil)
	result, err := svc.ListByAccount(99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil slice for empty result")
	}
}

// --- ListByClient ---

func TestListByClient_ReturnsAllCards(t *testing.T) {
	cards := []models.Card{
		{ID: 1, ClientID: 5, Status: "aktivna"},
		{ID: 2, ClientID: 5, Status: "deaktivirana"},
	}
	svc := newListCardSvc(cards)
	result, err := svc.ListByClient(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 cards, got %d", len(result))
	}
}

// --- BlockCard (client) ---

func TestBlockCard_ActiveCard_SetsBlokirana(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "aktivna"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	result, err := svc.BlockCard(1, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "blokirana" {
		t.Errorf("expected status=blokirana, got %s", result.Status)
	}
}

func TestBlockCard_WrongClient_ReturnsError(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "aktivna"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	_, err := svc.BlockCard(1, 99) // different clientID
	if err == nil {
		t.Error("expected error when wrong client tries to block card")
	}
}

func TestBlockCard_AlreadyBlokirana_ReturnsError(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "blokirana"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	_, err := svc.BlockCard(1, 5)
	if err == nil {
		t.Error("expected error when blocking already blocked card")
	}
}

func TestBlockCard_Deaktivirana_ReturnsError(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "deaktivirana"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	_, err := svc.BlockCard(1, 5)
	if err == nil {
		t.Error("expected error when trying to block deactivated card")
	}
}

// --- UnblockCard (employee) ---

func TestUnblockCard_BlokiranaCard_SetsAktivna(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "blokirana"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	result, err := svc.UnblockCard(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "aktivna" {
		t.Errorf("expected status=aktivna, got %s", result.Status)
	}
}

func TestUnblockCard_NotBlokirana_ReturnsError(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "aktivna"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	_, err := svc.UnblockCard(1)
	if err == nil {
		t.Error("expected error when unblocking non-blocked card")
	}
}

// --- DeactivateCard (employee) ---

func TestDeactivateCard_AnyStatus_SetsDeaktivirana(t *testing.T) {
	for _, status := range []string{"aktivna", "blokirana"} {
		card := models.Card{ID: 1, ClientID: 5, Status: status}
		cr := &listCardRepo{cards: []models.Card{card}}
		svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

		result, err := svc.DeactivateCard(1)
		if err != nil {
			t.Fatalf("status=%s: unexpected error: %v", status, err)
		}
		if result.Status != "deaktivirana" {
			t.Errorf("status=%s: expected deaktivirana, got %s", status, result.Status)
		}
	}
}

func TestDeactivateCard_AlreadyDeaktivirana_ReturnsError(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "deaktivirana"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	_, err := svc.DeactivateCard(1)
	if err == nil {
		t.Error("expected error when deactivating already deactivated card")
	}
}

func TestDeactivateCard_IsPermanent_CannotReactivate(t *testing.T) {
	card := models.Card{ID: 1, ClientID: 5, Status: "deaktivirana"}
	cr := &listCardRepo{cards: []models.Card{card}}
	svc := service.NewCardService(cr, &mockAcctRepoForCard{account: licniAccount()}, nil)

	// Neither unblock nor deactivate again should work
	_, err1 := svc.UnblockCard(1)
	_, err2 := svc.DeactivateCard(1)
	if err1 == nil {
		t.Error("expected error when unblocking deactivated card")
	}
	if err2 == nil {
		t.Error("expected error when deactivating already deactivated card")
	}
}
