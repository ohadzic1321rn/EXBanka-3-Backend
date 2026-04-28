package repository

import (
	"testing"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"gorm.io/gorm"
)

func seedRepositoryListing(t *testing.T, db *gorm.DB, ticker, listingType string) uint {
	t.Helper()
	exchange := models.MarketExchangeRecord{
		Acronym:      "X" + ticker,
		Name:         "Exchange " + ticker,
		MICCode:      "M" + ticker,
		Polity:       "US",
		Currency:     "USD",
		Timezone:     "UTC",
		WorkingHours: "09:30-16:00",
		Enabled:      true,
	}
	if err := db.Create(&exchange).Error; err != nil {
		t.Fatal(err)
	}

	listing := models.MarketListingRecord{
		Ticker:      ticker,
		Name:        ticker + " Corp",
		ExchangeID:  exchange.ID,
		LastRefresh: time.Now().UTC(),
		Price:       100,
		Ask:         101,
		Bid:         99,
		Volume:      1000,
		Type:        listingType,
	}
	if err := db.Create(&listing).Error; err != nil {
		t.Fatal(err)
	}
	return listing.ID
}

func TestPortfolioRepository_ListPublicOTCHoldings(t *testing.T) {
	db := openMarketRepositoryTestDB(t, "portfolio_public_otc")
	repo := NewPortfolioRepository(db)

	publicStockID := seedRepositoryListing(t, db, "PUB", string(models.ListingTypeStock))
	legacyStockID := seedRepositoryListing(t, db, "LEG", string(models.ListingTypeStock))
	privateStockID := seedRepositoryListing(t, db, "PRI", string(models.ListingTypeStock))
	reservedStockID := seedRepositoryListing(t, db, "RES", string(models.ListingTypeStock))
	ownStockID := seedRepositoryListing(t, db, "OWN", string(models.ListingTypeStock))
	forexID := seedRepositoryListing(t, db, "FX", "forex")

	holdings := []models.PortfolioHoldingRecord{
		{UserID: 200, UserType: "client", AssetID: publicStockID, Quantity: 10, PublicQuantity: 6, ReservedQuantity: 2, AvgBuyPrice: 90, AccountID: 1},
		{UserID: 201, UserType: "client", AssetID: legacyStockID, Quantity: 8, IsPublic: true, AvgBuyPrice: 80, AccountID: 1},
		{UserID: 202, UserType: "client", AssetID: privateStockID, Quantity: 5, AvgBuyPrice: 75, AccountID: 1},
		{UserID: 203, UserType: "client", AssetID: reservedStockID, Quantity: 5, PublicQuantity: 5, ReservedQuantity: 5, AvgBuyPrice: 70, AccountID: 1},
		{UserID: 100, UserType: "client", AssetID: ownStockID, Quantity: 5, PublicQuantity: 5, AvgBuyPrice: 60, AccountID: 1},
		{UserID: 204, UserType: "client", AssetID: forexID, Quantity: 5, PublicQuantity: 5, AvgBuyPrice: 1.1, AccountID: 1},
	}
	if err := db.Create(&holdings).Error; err != nil {
		t.Fatal(err)
	}

	records, err := repo.ListPublicOTCHoldings(100, "client")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 public OTC stock holdings, got %d", len(records))
	}

	seen := map[string]float64{}
	for _, record := range records {
		seen[record.Asset.Ticker] = record.AvailableForOTC()
	}
	if seen["PUB"] != 4 {
		t.Fatalf("expected PUB available quantity 4, got %v", seen["PUB"])
	}
	if seen["LEG"] != 8 {
		t.Fatalf("expected legacy public LEG available quantity 8, got %v", seen["LEG"])
	}
	if _, ok := seen["OWN"]; ok {
		t.Fatal("requester should not see own public holding")
	}
	if _, ok := seen["FX"]; ok {
		t.Fatal("non-stock holding should not be listed for OTC")
	}
}
