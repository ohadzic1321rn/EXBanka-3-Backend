package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newInMemoryDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// =====================
// Migrate
// =====================

func TestMigrate_RunsWithoutError(t *testing.T) {
	db := newInMemoryDB(t, "test_migrate")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
}

func TestMigrate_CreatesExpectedTables(t *testing.T) {
	db := newInMemoryDB(t, "test_migrate_tables")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	tableModels := []interface{}{
		&models.MarketExchangeRecord{},
		&models.MarketListingRecord{},
		&models.StockRecord{},
		&models.ForexPairRecord{},
		&models.FuturesContractRecord{},
		&models.OptionRecord{},
		&models.OrderRecord{},
	}
	for _, m := range tableModels {
		if !db.Migrator().HasTable(m) {
			t.Errorf("expected table for %T to exist after migration", m)
		}
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	db := newInMemoryDB(t, "test_migrate_idempotent")
	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate failed: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate failed: %v", err)
	}
}

// =====================
// SeedMarketData
// =====================

func TestSeedMarketData_RunsWithoutError(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_market")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("SeedMarketData failed: %v", err)
	}
}

func TestSeedMarketData_InsertsExchanges(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_exchanges")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("SeedMarketData failed: %v", err)
	}

	var count int64
	db.Model(&models.MarketExchangeRecord{}).Count(&count)
	if count < 3 {
		t.Errorf("expected at least 3 exchanges, got %d", count)
	}
}

func TestSeedMarketData_InsertsStockListings(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_stocks")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("SeedMarketData failed: %v", err)
	}

	var count int64
	db.Model(&models.MarketListingRecord{}).Where("type = ?", "stock").Count(&count)
	if count < 10 {
		t.Errorf("expected at least 10 stock listings, got %d", count)
	}
}

func TestSeedMarketData_InsertsForexListings(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_forex")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("SeedMarketData failed: %v", err)
	}

	var count int64
	db.Model(&models.MarketListingRecord{}).Where("type = ?", "forex").Count(&count)
	if count < 3 {
		t.Errorf("expected at least 3 forex listings, got %d", count)
	}
}

func TestSeedMarketData_InsertsFuturesListings(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_futures")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("SeedMarketData failed: %v", err)
	}

	var count int64
	db.Model(&models.MarketListingRecord{}).Where("type = ?", "futures").Count(&count)
	if count < 3 {
		t.Errorf("expected at least 3 futures listings, got %d", count)
	}
}

func TestSeedMarketData_InsertsOptions(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_options")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("SeedMarketData failed: %v", err)
	}

	var count int64
	db.Model(&models.MarketListingRecord{}).Where("type = ?", "option").Count(&count)
	if count == 0 {
		t.Error("expected at least one option to be seeded")
	}
}

func TestSeedMarketData_InsertsHistory(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_history")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("SeedMarketData failed: %v", err)
	}

	var count int64
	db.Model(&models.MarketListingDailyPriceInfoRecord{}).Count(&count)
	// 12 stocks + 6 forex + 6 futures = 24 non-option listings × 30 days history
	if count < 200 {
		t.Errorf("expected substantial history records, got %d", count)
	}
}

func TestSeedMarketData_IsIdempotent(t *testing.T) {
	db := newInMemoryDB(t, "test_seed_idempotent")
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	if err := SeedMarketData(db); err != nil {
		t.Fatalf("first SeedMarketData failed: %v", err)
	}

	var countBefore int64
	db.Model(&models.MarketListingRecord{}).Count(&countBefore)

	if err := SeedMarketData(db); err != nil {
		t.Fatalf("second SeedMarketData failed: %v", err)
	}

	var countAfter int64
	db.Model(&models.MarketListingRecord{}).Count(&countAfter)

	if countAfter != countBefore {
		t.Errorf("idempotency broken: before=%d, after=%d", countBefore, countAfter)
	}
}

// =====================
// seedForexListings
// =====================

func TestSeedForexListings_ReturnsForexType(t *testing.T) {
	listings := seedForexListings()
	if len(listings) == 0 {
		t.Fatal("expected non-empty forex listings")
	}
	for _, l := range listings {
		if l.Type != models.ListingTypeForex {
			t.Errorf("expected type=forex, got %v for ticker %s", l.Type, l.Ticker)
		}
		if l.BaseCurrency == "" || l.QuoteCurrency == "" {
			t.Errorf("expected non-empty currencies for %s", l.Ticker)
		}
	}
}

func TestSeedForexListings_HasAtLeastThreeEntries(t *testing.T) {
	listings := seedForexListings()
	if len(listings) < 3 {
		t.Errorf("expected at least 3 forex listings, got %d", len(listings))
	}
}

func TestSeedForexListings_AllHavePositivePrices(t *testing.T) {
	for _, l := range seedForexListings() {
		if l.Price <= 0 {
			t.Errorf("forex %s has non-positive price %f", l.Ticker, l.Price)
		}
	}
}

// =====================
// seedFuturesListings
// =====================

func TestSeedFuturesListings_ReturnsFuturesType(t *testing.T) {
	listings := seedFuturesListings()
	if len(listings) == 0 {
		t.Fatal("expected non-empty futures listings")
	}
	for _, l := range listings {
		if l.Type != models.ListingTypeFutures {
			t.Errorf("expected type=futures, got %v for ticker %s", l.Type, l.Ticker)
		}
		if l.ContractSize <= 0 {
			t.Errorf("expected positive ContractSize for %s", l.Ticker)
		}
		if l.ContractUnit == "" {
			t.Errorf("expected non-empty ContractUnit for %s", l.Ticker)
		}
	}
}

func TestSeedFuturesListings_HasAtLeastThreeEntries(t *testing.T) {
	listings := seedFuturesListings()
	if len(listings) < 3 {
		t.Errorf("expected at least 3 futures listings, got %d", len(listings))
	}
}

func TestSeedFuturesListings_SettlementDatesInFuture(t *testing.T) {
	ref := seedReferenceTime()
	for _, l := range seedFuturesListings() {
		if !l.SettlementDate.After(ref) {
			t.Errorf("futures %s settlement date %v is not after reference %v", l.Ticker, l.SettlementDate, ref)
		}
	}
}

// =====================
// generateExpirationDates
// =====================

func TestGenerateExpirationDates_ReturnsThreeDates(t *testing.T) {
	ref := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	dates := generateExpirationDates(ref)
	if len(dates) != 3 {
		t.Errorf("expected 3 expiration dates, got %d", len(dates))
	}
}

func TestGenerateExpirationDates_DatesAreAfterReference(t *testing.T) {
	ref := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	for i, d := range generateExpirationDates(ref) {
		if !d.After(ref) {
			t.Errorf("date[%d] %v is not after reference %v", i, d, ref)
		}
	}
}

func TestGenerateExpirationDates_DatesAreOrdered(t *testing.T) {
	ref := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	dates := generateExpirationDates(ref)
	for i := 1; i < len(dates); i++ {
		if !dates[i].After(dates[i-1]) {
			t.Errorf("date[%d] %v is not after date[%d] %v", i, dates[i], i-1, dates[i-1])
		}
	}
}
