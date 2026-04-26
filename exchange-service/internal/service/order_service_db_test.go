package service_test

import (
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/repository"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/service"
)

func TestOrderService_GetOrder_NotFound(t *testing.T) {
	db := openTestDB(t, "os_get_notfound")
	svc := service.NewOrderService(
		repository.NewOrderRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{},
	)
	_, err := svc.GetOrder(9999)
	if err == nil {
		t.Error("expected error when order missing")
	}
}

func TestOrderService_ListAndGet(t *testing.T) {
	db := openTestDB(t, "os_list_get")
	assetID := seedAsset(t, db, "AAA", 50, "USD")

	orderRepo := repository.NewOrderRepository(db)
	if err := orderRepo.CreateOrder(&models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 1, ContractSize: 1, PricePerUnit: 50,
		Status: "approved", RemainingPortions: 1, AccountID: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := orderRepo.CreateOrder(&models.OrderRecord{
		UserID: 1, UserType: "client", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 2, ContractSize: 1, PricePerUnit: 50,
		Status: "pending", RemainingPortions: 2, AccountID: 1,
	}); err != nil {
		t.Fatal(err)
	}

	svc := service.NewOrderService(orderRepo, repository.NewMarketRepository(db), &mockRateProv{})

	bankOrders, err := svc.ListOrdersForUser(0, "bank", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bankOrders) != 1 {
		t.Errorf("expected 1 bank order, got %d", len(bankOrders))
	}

	all, err := svc.ListAllOrders("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 total orders, got %d", len(all))
	}

	pending, err := svc.ListAllOrders("pending")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}

	got, err := svc.GetOrder(bankOrders[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserType != "bank" {
		t.Errorf("got %v", got)
	}

	txs, err := svc.ListTransactionsForOrder(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 0 {
		t.Errorf("expected 0 txs for fresh order, got %d", len(txs))
	}
}

func TestOrderService_CreateOrder_RejectsInvalidInput(t *testing.T) {
	db := openTestDB(t, "os_create_invalid")
	svc := service.NewOrderService(
		repository.NewOrderRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{},
	)
	_, err := svc.CreateOrder(service.CreateOrderInput{
		// missing everything
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestOrderService_CreateOrder_AssetNotFound(t *testing.T) {
	db := openTestDB(t, "os_create_no_asset")
	svc := service.NewOrderService(
		repository.NewOrderRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{},
	)
	_, err := svc.CreateOrder(service.CreateOrderInput{
		UserType: "client", UserID: 1, AssetTicker: "NOPE",
		OrderType: "market", Direction: "buy", Quantity: 1, AccountID: 1,
	})
	if err == nil {
		t.Fatal("expected asset-not-found error")
	}
}
