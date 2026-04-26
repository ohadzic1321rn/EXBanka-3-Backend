package repository

import (
	"testing"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
)

func seedExchangeAndAsset(t *testing.T, name, ticker string) (db_id_repo *OrderRepository, marketRepo *MarketRepository, portfolioRepo *PortfolioRepository, taxRepo *TaxRepository, assetID uint) {
	t.Helper()
	db := openMarketRepositoryTestDB(t, name)
	exch := models.MarketExchangeRecord{
		Acronym: "X", Name: "X Exchange", MICCode: "X1", Polity: "X", Currency: "USD",
		Timezone: "UTC", WorkingHours: "09:00-17:00",
	}
	if err := db.Create(&exch).Error; err != nil {
		t.Fatalf("seed exchange: %v", err)
	}
	listing := models.MarketListingRecord{
		Ticker: ticker, Name: ticker, Type: "stock",
		ExchangeID: exch.ID, Price: 100, Ask: 101, Bid: 99, Volume: 1000,
	}
	if err := db.Create(&listing).Error; err != nil {
		t.Fatalf("seed listing: %v", err)
	}
	return NewOrderRepository(db), NewMarketRepository(db), NewPortfolioRepository(db), NewTaxRepository(db), listing.ID
}

func TestOrderRepository_CreateAndQuery(t *testing.T) {
	repo, _, _, _, assetID := seedExchangeAndAsset(t, "or_create_query", "AAA")

	o := &models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 5, ContractSize: 1, PricePerUnit: 100,
		Status: "pending", RemainingPortions: 5, AccountID: 1,
	}
	if err := repo.CreateOrder(o); err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if o.ID == 0 {
		t.Fatal("expected order ID set")
	}

	got, err := repo.GetOrderByID(o.ID)
	if err != nil || got == nil {
		t.Fatalf("GetOrderByID: %v %v", got, err)
	}
	if got.UserType != "bank" {
		t.Errorf("got %v", got)
	}

	if _, err := repo.ListOrdersForUser(0, "bank", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ListOrdersForUser(0, "bank", "pending"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ListAllOrders(""); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ListAllOrders("pending"); err != nil {
		t.Fatal(err)
	}
}

func TestOrderRepository_GetOrderByID_NotFound(t *testing.T) {
	repo, _, _, _, _ := seedExchangeAndAsset(t, "or_notfound", "BBB")
	got, err := repo.GetOrderByID(9999)
	if err != nil {
		t.Fatalf("expected nil error for not-found, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestOrderRepository_StatusFlow(t *testing.T) {
	repo, _, _, _, assetID := seedExchangeAndAsset(t, "or_status_flow", "CCC")

	o := &models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 5, ContractSize: 1, PricePerUnit: 100,
		Status: "pending", RemainingPortions: 5, AccountID: 1,
	}
	_ = repo.CreateOrder(o)

	approver := uint(42)
	if err := repo.UpdateOrderStatus(o.ID, "approved", &approver); err != nil {
		t.Fatalf("UpdateOrderStatus: %v", err)
	}
	got, _ := repo.GetOrderByID(o.ID)
	if got.Status != "approved" {
		t.Errorf("got status=%v", got.Status)
	}
	if got.ApprovedBy == nil || *got.ApprovedBy != 42 {
		t.Errorf("approved_by not set, got %v", got.ApprovedBy)
	}

	if err := repo.DecrementRemainingPortions(o.ID, 2); err != nil {
		t.Fatalf("DecrementRemainingPortions: %v", err)
	}
	got, _ = repo.GetOrderByID(o.ID)
	if got.RemainingPortions != 3 {
		t.Errorf("expected 3 remaining, got %d", got.RemainingPortions)
	}

	if err := repo.DecrementRemainingPortions(o.ID, 3); err != nil {
		t.Fatalf("DecrementRemainingPortions full: %v", err)
	}
	got, _ = repo.GetOrderByID(o.ID)
	if !got.IsDone || got.Status != "done" {
		t.Errorf("expected order done, got %+v", got)
	}
}

func TestOrderRepository_SetRemainingAndCancel(t *testing.T) {
	repo, _, _, _, assetID := seedExchangeAndAsset(t, "or_cancel", "DDD")
	o := &models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 5, ContractSize: 1, PricePerUnit: 100,
		Status: "pending", RemainingPortions: 5, AccountID: 1,
	}
	_ = repo.CreateOrder(o)

	if err := repo.SetRemainingPortions(o.ID, 2); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetOrderByID(o.ID)
	if got.RemainingPortions != 2 {
		t.Errorf("expected 2, got %d", got.RemainingPortions)
	}

	if err := repo.FullCancelOrder(o.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetOrderByID(o.ID)
	if got.Status != "cancelled" || !got.IsDone {
		t.Errorf("expected cancelled+done, got %+v", got)
	}
}

func TestOrderRepository_ListPendingActiveOrders(t *testing.T) {
	repo, _, _, _, assetID := seedExchangeAndAsset(t, "or_pending_active", "EEE")
	_ = repo.CreateOrder(&models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 1, ContractSize: 1, PricePerUnit: 100,
		Status: "approved", RemainingPortions: 1, AccountID: 1,
	})
	_ = repo.CreateOrder(&models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 1, ContractSize: 1, PricePerUnit: 100,
		Status: "done", IsDone: true, RemainingPortions: 0, AccountID: 1,
	})

	active, err := repo.ListPendingActiveOrders()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestOrderRepository_TransactionsAndQueries(t *testing.T) {
	repo, _, _, _, assetID := seedExchangeAndAsset(t, "or_transactions", "FFF")
	o := &models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, OrderType: "market",
		Direction: "buy", Quantity: 1, ContractSize: 1, PricePerUnit: 100,
		Status: "approved", RemainingPortions: 1, AccountID: 1,
	}
	_ = repo.CreateOrder(o)

	tx := &models.OrderTransactionRecord{OrderID: o.ID, Quantity: 1, PricePerUnit: 100, ExecutedAt: time.Now().UTC()}
	if err := repo.CreateOrderTransaction(tx); err != nil {
		t.Fatal(err)
	}
	txs, err := repo.ListTransactionsForOrder(o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 {
		t.Errorf("expected 1 tx, got %d", len(txs))
	}
}

func TestOrderRepository_GetSettlementDate(t *testing.T) {
	repo, _, _, _, assetID := seedExchangeAndAsset(t, "or_settle_none", "GGG")
	got, err := repo.GetSettlementDate(assetID)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for stock, got %v", got)
	}
}

// --- PortfolioRepository ---

func TestPortfolioRepository_BuyAndSellFlow(t *testing.T) {
	_, _, prepo, _, assetID := seedExchangeAndAsset(t, "pr_buy_sell", "HHH")

	if err := prepo.RecordBuyFill(0, "bank", assetID, 1, 10, 100); err != nil {
		t.Fatal(err)
	}
	// Second buy at higher price — weighted average.
	if err := prepo.RecordBuyFill(0, "bank", assetID, 1, 10, 110); err != nil {
		t.Fatal(err)
	}

	h, err := prepo.GetHoldingByUserAndAsset(0, "bank", assetID)
	if err != nil {
		t.Fatal(err)
	}
	if h.Quantity != 20 {
		t.Errorf("expected 20 qty, got %v", h.Quantity)
	}
	if h.AvgBuyPrice != 105 {
		t.Errorf("expected avg=105, got %v", h.AvgBuyPrice)
	}

	profit, err := prepo.RecordSellFill(0, "bank", assetID, 5, 120)
	if err != nil {
		t.Fatal(err)
	}
	if profit <= 0 {
		t.Errorf("expected positive profit, got %v", profit)
	}

	h2, _ := prepo.GetHoldingByID(h.ID)
	if h2.Quantity != 15 {
		t.Errorf("expected 15 qty after sell of 5, got %v", h2.Quantity)
	}

	list, _ := prepo.ListHoldingsForUser(0, "bank")
	if len(list) != 1 {
		t.Errorf("expected 1 holding, got %d", len(list))
	}

	if err := prepo.SetHoldingPublic(h.ID, true); err != nil {
		t.Fatal(err)
	}
	h3, _ := prepo.GetHoldingByID(h.ID)
	if !h3.IsPublic {
		t.Error("expected public flag set")
	}

	if err := prepo.ExerciseOptionHolding(h.ID, 50); err != nil {
		t.Fatal(err)
	}
	h4, _ := prepo.GetHoldingByID(h.ID)
	if h4.Quantity != 0 {
		t.Errorf("expected 0 qty after exercise, got %v", h4.Quantity)
	}
}

func TestPortfolioRepository_NotFoundReturnsNil(t *testing.T) {
	_, _, prepo, _, _ := seedExchangeAndAsset(t, "pr_notfound", "III")
	got, err := prepo.GetHoldingByUserAndAsset(0, "bank", 9999)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
	got2, err := prepo.GetHoldingByID(9999)
	if err != nil {
		t.Fatal(err)
	}
	if got2 != nil {
		t.Errorf("expected nil, got %+v", got2)
	}
}

// --- TaxRepository ---

func TestTaxRepository_FullFlow(t *testing.T) {
	_, _, _, taxRepo, _ := seedExchangeAndAsset(t, "tr_flow", "JJJ")

	now := time.Now().UTC()
	period := now.Format("2006-01")

	rec := &models.TaxRecord{
		UserID: 1, UserType: "client", AssetID: 1, Period: period,
		ProfitRSD: 1000, TaxRSD: 150, Status: "unpaid", CreatedAt: now, UpdatedAt: now,
	}
	if err := taxRepo.CreateTaxRecord(rec); err != nil {
		t.Fatal(err)
	}

	listForUser, err := taxRepo.ListTaxRecordsForUser(1, "client", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listForUser) != 1 {
		t.Errorf("expected 1, got %d", len(listForUser))
	}

	// With period filter.
	if list, _ := taxRepo.ListTaxRecordsForUser(1, "client", period); len(list) != 1 {
		t.Errorf("expected 1 with period filter, got %d", len(list))
	}

	if all, _ := taxRepo.ListAllTaxRecords(""); len(all) != 1 {
		t.Errorf("expected 1 all, got %d", len(all))
	}
	if all, _ := taxRepo.ListAllTaxRecords(period); len(all) != 1 {
		t.Errorf("expected 1 all-w-period, got %d", len(all))
	}

	unpaid, _ := taxRepo.SumUnpaidTaxForUser(1, "client", period)
	if unpaid != 150 {
		t.Errorf("expected 150 unpaid, got %v", unpaid)
	}

	users, _ := taxRepo.ListDistinctUsersWithUnpaidTax(period)
	if len(users) != 1 {
		t.Errorf("expected 1 distinct unpaid user, got %d", len(users))
	}

	if err := taxRepo.MarkTaxRecordsPaid(1, "client", period); err != nil {
		t.Fatal(err)
	}

	year := period[:4]
	paid, _ := taxRepo.SumPaidTaxForUserYear(1, "client", year)
	if paid != 150 {
		t.Errorf("expected 150 paid after mark, got %v", paid)
	}
	unpaid, _ = taxRepo.SumUnpaidTaxForUser(1, "client", period)
	if unpaid != 0 {
		t.Errorf("expected 0 unpaid after mark, got %v", unpaid)
	}
}

// --- MarketRepository ---

func TestMarketRepository_GetListingByTickerAndID(t *testing.T) {
	_, mrepo, _, _, assetID := seedExchangeAndAsset(t, "mr_lookup", "KKK")

	byTicker, err := mrepo.GetListingRecordByTicker("KKK")
	if err != nil || byTicker == nil {
		t.Fatalf("by ticker: %v %v", byTicker, err)
	}

	byID, err := mrepo.GetListingRecordByID(assetID)
	if err != nil || byID == nil {
		t.Fatalf("by id: %v %v", byID, err)
	}

	missing, err := mrepo.GetListingRecordByTicker("NOPE")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing ticker, got %+v", missing)
	}
}
