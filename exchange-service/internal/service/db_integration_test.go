package service_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/database"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/repository"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/service"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedAsset(t *testing.T, db *gorm.DB, ticker string, price float64, currency string) uint {
	t.Helper()
	exch := models.MarketExchangeRecord{
		Acronym: "TEST", Name: "Test Exchange", MICCode: "T1", Polity: "X", Currency: currency,
		Timezone: "UTC", WorkingHours: "09:00-17:00",
	}
	if err := db.Create(&exch).Error; err != nil {
		t.Fatalf("seed exchange: %v", err)
	}
	listing := models.MarketListingRecord{
		Ticker: ticker, Name: ticker, Type: "stock",
		ExchangeID: exch.ID, Price: price, Ask: price + 1, Bid: price - 1, Volume: 1000,
	}
	if err := db.Create(&listing).Error; err != nil {
		t.Fatalf("seed listing: %v", err)
	}
	return listing.ID
}

// --- Mock rate provider ---

type mockRateProv struct {
	rates map[string]float64
}

func (m *mockRateProv) GetRate(from, to string) (float64, error) {
	if from == to {
		return 1, nil
	}
	if r, ok := m.rates[from+":"+to]; ok {
		return r, nil
	}
	return 0, errors.New("no rate")
}

func (m *mockRateProv) GetAllRates() []service.ExchangeRate { return nil }

// --- PortfolioService tests ---

func TestPortfolioService_RecordFillAndList(t *testing.T) {
	db := openTestDB(t, "ps_record_fill")
	assetID := seedAsset(t, db, "ZZZ", 100, "USD")

	portfolioRepo := repository.NewPortfolioRepository(db)
	taxRepo := repository.NewTaxRepository(db)
	marketRepo := repository.NewMarketRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	rates := &mockRateProv{rates: map[string]float64{"USD:RSD": 110}}
	taxSvc := service.NewTaxService(taxRepo, marketRepo, rates)
	psvc := service.NewPortfolioService(portfolioRepo, taxSvc, marketRepo, orderRepo)

	// Record a buy fill via the service.
	buyOrder := &models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, ContractSize: 1,
		Direction: "buy", AccountID: 1,
		Asset: models.MarketListingRecord{Type: "stock", Exchange: models.MarketExchangeRecord{Currency: "USD"}},
	}
	if err := psvc.RecordFill(buyOrder, 10, 100); err != nil {
		t.Fatalf("RecordFill buy: %v", err)
	}

	holdings, err := psvc.ListHoldingsWithPnL(0, "bank")
	if err != nil {
		t.Fatalf("ListHoldingsWithPnL: %v", err)
	}
	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(holdings))
	}
	if holdings[0].Holding.Quantity != 10 {
		t.Errorf("quantity=%v", holdings[0].Holding.Quantity)
	}

	// Record a sell at a higher price -> realised gain.
	sellOrder := &models.OrderRecord{
		UserID: 0, UserType: "bank", AssetID: assetID, ContractSize: 1,
		Direction: "sell", AccountID: 1,
		Asset: models.MarketListingRecord{Type: "stock", Exchange: models.MarketExchangeRecord{Currency: "USD"}},
	}
	if err := psvc.RecordFill(sellOrder, 5, 120); err != nil {
		t.Fatalf("RecordFill sell: %v", err)
	}

	got, err := psvc.GetHoldingByID(holdings[0].Holding.ID)
	if err != nil {
		t.Fatalf("GetHoldingByID: %v", err)
	}
	if got.Quantity != 5 {
		t.Errorf("expected qty=5 after sell of 5, got %v", got.Quantity)
	}
	if got.RealizedProfit <= 0 {
		t.Errorf("expected positive realised profit, got %v", got.RealizedProfit)
	}

	// SetPublic flips the flag.
	if err := psvc.SetPublic(got.ID, true); err != nil {
		t.Fatalf("SetPublic: %v", err)
	}
	again, _ := psvc.GetHoldingByID(got.ID)
	if !again.IsPublic || again.PublicQuantity != again.Quantity {
		t.Errorf("expected SetPublic to expose all remaining shares, got isPublic=%v publicQty=%v qty=%v", again.IsPublic, again.PublicQuantity, again.Quantity)
	}

	if err := psvc.SetPublicQuantity(got.ID, 3); err != nil {
		t.Fatalf("SetPublicQuantity: %v", err)
	}
	again, _ = psvc.GetHoldingByID(got.ID)
	if again.PublicQuantity != 3 || again.AvailableForOTC() != 3 {
		t.Errorf("unexpected OTC quantities after SetPublicQuantity: public=%v available=%v", again.PublicQuantity, again.AvailableForOTC())
	}

	// ListHoldings (raw, no PnL).
	raw, err := psvc.ListHoldings(0, "bank")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 {
		t.Errorf("expected 1 raw holding, got %d", len(raw))
	}
}

func TestPortfolioService_GetHoldingByID_NotFound(t *testing.T) {
	db := openTestDB(t, "ps_not_found")
	psvc := service.NewPortfolioService(
		repository.NewPortfolioRepository(db),
		service.NewTaxService(repository.NewTaxRepository(db), repository.NewMarketRepository(db), &mockRateProv{}),
		repository.NewMarketRepository(db),
		repository.NewOrderRepository(db),
	)
	if _, err := psvc.GetHoldingByID(9999); err == nil {
		t.Error("expected not found")
	}
	if _, err := psvc.GetHoldingWithPnL(9999); err == nil {
		t.Error("expected not found from GetHoldingWithPnL")
	}
}

// --- TaxService tests ---

func TestTaxService_RecordCapitalGainTax_PositiveStock(t *testing.T) {
	db := openTestDB(t, "tax_positive")
	taxSvc := service.NewTaxService(
		repository.NewTaxRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{rates: map[string]float64{"USD:RSD": 100}},
	)
	if err := taxSvc.RecordCapitalGainTax(5, "client", 1, 50, "stock", "USD"); err != nil {
		t.Fatalf("RecordCapitalGainTax: %v", err)
	}
	records, err := taxSvc.ListTaxRecords(5, "client", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	// 50 USD -> 5000 RSD; 15% -> 750
	if records[0].TaxRSD != 750 {
		t.Errorf("expected tax_rsd=750, got %v", records[0].TaxRSD)
	}
}

func TestTaxService_RecordCapitalGainTax_NonStockSkipped(t *testing.T) {
	db := openTestDB(t, "tax_skip_forex")
	taxSvc := service.NewTaxService(
		repository.NewTaxRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{rates: map[string]float64{"USD:RSD": 100}},
	)
	if err := taxSvc.RecordCapitalGainTax(5, "client", 1, 50, "forex", "USD"); err != nil {
		t.Fatal(err)
	}
	records, _ := taxSvc.ListTaxRecords(5, "client", "")
	if len(records) != 0 {
		t.Errorf("forex profit should be skipped, got %d records", len(records))
	}
}

func TestTaxService_RecordCapitalGainTax_NonPositiveSkipped(t *testing.T) {
	db := openTestDB(t, "tax_skip_loss")
	taxSvc := service.NewTaxService(
		repository.NewTaxRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{rates: map[string]float64{"USD:RSD": 100}},
	)
	if err := taxSvc.RecordCapitalGainTax(5, "client", 1, -10, "stock", "USD"); err != nil {
		t.Fatal(err)
	}
	records, _ := taxSvc.ListTaxRecords(5, "client", "")
	if len(records) != 0 {
		t.Errorf("loss should be skipped, got %d records", len(records))
	}
}

func TestTaxService_RecordCapitalGainTax_RSDNoConversion(t *testing.T) {
	db := openTestDB(t, "tax_rsd")
	taxSvc := service.NewTaxService(
		repository.NewTaxRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{},
	)
	if err := taxSvc.RecordCapitalGainTax(5, "client", 1, 1000, "stock", "RSD"); err != nil {
		t.Fatal(err)
	}
	records, _ := taxSvc.ListTaxRecords(5, "client", "")
	if len(records) != 1 || records[0].TaxRSD != 150 {
		t.Errorf("expected 1 record with tax=150, got %+v", records)
	}
}

func TestTaxService_RecordCapitalGainTax_NoRateFallback(t *testing.T) {
	db := openTestDB(t, "tax_fallback")
	taxSvc := service.NewTaxService(
		repository.NewTaxRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{}, // no rates
	)
	if err := taxSvc.RecordCapitalGainTax(5, "client", 1, 100, "stock", "GBP"); err != nil {
		t.Fatal(err)
	}
	records, _ := taxSvc.ListTaxRecords(5, "client", "")
	if len(records) != 1 || records[0].TaxRSD != 15 {
		t.Errorf("expected fallback 1:1 (tax=15), got %+v", records)
	}
}

func TestTaxService_ListAllAndSumQueries(t *testing.T) {
	db := openTestDB(t, "tax_sums")
	taxSvc := service.NewTaxService(
		repository.NewTaxRepository(db),
		repository.NewMarketRepository(db),
		&mockRateProv{},
	)
	_ = taxSvc.RecordCapitalGainTax(1, "client", 10, 100, "stock", "RSD")
	_ = taxSvc.RecordCapitalGainTax(2, "client", 10, 200, "stock", "RSD")

	all, err := taxSvc.ListAllTaxRecords("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 all records, got %d", len(all))
	}

	period := all[0].Period
	unpaid, err := taxSvc.SumUnpaidTax(1, "client", period)
	if err != nil {
		t.Fatal(err)
	}
	if unpaid != 15 {
		t.Errorf("expected 15 unpaid for client 1, got %v", unpaid)
	}

	year := period[:4]
	paid, err := taxSvc.SumPaidTaxForYear(1, "client", year)
	if err != nil {
		t.Fatal(err)
	}
	if paid != 0 {
		t.Errorf("expected 0 paid (none marked paid), got %v", paid)
	}
}
