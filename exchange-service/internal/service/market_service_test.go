package service_test

import (
	"errors"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/service"
)

type fakeMarketProvider struct {
	exchanges []models.Exchange
	listings  []models.Listing
	listing   *models.Listing
	history   []models.ListingDailyPriceInfo
	portfolio *models.Portfolio
	err       error
}

func (f *fakeMarketProvider) GetExchanges() ([]models.Exchange, error) {
	return f.exchanges, f.err
}
func (f *fakeMarketProvider) GetListings() ([]models.Listing, error) {
	return f.listings, f.err
}
func (f *fakeMarketProvider) GetListing(ticker string) (*models.Listing, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listing, nil
}
func (f *fakeMarketProvider) GetHistory(ticker string) ([]models.ListingDailyPriceInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.history, nil
}
func (f *fakeMarketProvider) GetPortfolio(ownerID uint, ownerType models.PortfolioOwnerType) (*models.Portfolio, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.portfolio, nil
}

func TestMarketService_ListExchanges(t *testing.T) {
	p := &fakeMarketProvider{exchanges: []models.Exchange{{Acronym: "NASDAQ"}}}
	svc := service.NewMarketService(p)
	got, err := svc.ListExchanges()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Acronym != "NASDAQ" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestMarketService_ListListings_NoQuery(t *testing.T) {
	p := &fakeMarketProvider{listings: []models.Listing{
		{Ticker: "AAPL", Name: "Apple"},
		{Ticker: "GOOG", Name: "Google"},
	}}
	svc := service.NewMarketService(p)
	got, err := svc.ListListings("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 listings, got %d", len(got))
	}
}

func TestMarketService_ListListings_FilterByTicker(t *testing.T) {
	p := &fakeMarketProvider{listings: []models.Listing{
		{Ticker: "AAPL", Name: "Apple"},
		{Ticker: "GOOG", Name: "Google"},
	}}
	svc := service.NewMarketService(p)
	got, err := svc.ListListings("aap")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Ticker != "AAPL" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestMarketService_ListListings_FilterByName(t *testing.T) {
	p := &fakeMarketProvider{listings: []models.Listing{
		{Ticker: "AAPL", Name: "Apple"},
		{Ticker: "GOOG", Name: "Google"},
	}}
	svc := service.NewMarketService(p)
	got, err := svc.ListListings("oog")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Ticker != "GOOG" {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestMarketService_ListListings_ProviderError(t *testing.T) {
	p := &fakeMarketProvider{err: errors.New("boom")}
	svc := service.NewMarketService(p)
	if _, err := svc.ListListings(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarketService_GetListing_Found(t *testing.T) {
	p := &fakeMarketProvider{listing: &models.Listing{Ticker: "AAPL"}}
	svc := service.NewMarketService(p)
	got, err := svc.GetListing("aapl")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ticker != "AAPL" {
		t.Errorf("got %v", got)
	}
}

func TestMarketService_GetListing_NotFound(t *testing.T) {
	p := &fakeMarketProvider{listing: nil}
	svc := service.NewMarketService(p)
	if _, err := svc.GetListing("XYZ"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestMarketService_GetListing_ProviderError(t *testing.T) {
	p := &fakeMarketProvider{err: errors.New("boom")}
	svc := service.NewMarketService(p)
	if _, err := svc.GetListing("AAPL"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarketService_GetListingHistory_Found(t *testing.T) {
	p := &fakeMarketProvider{history: []models.ListingDailyPriceInfo{{Price: 100}}}
	svc := service.NewMarketService(p)
	got, err := svc.GetListingHistory("AAPL")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("got %d", len(got))
	}
}

func TestMarketService_GetListingHistory_NotFound(t *testing.T) {
	p := &fakeMarketProvider{history: nil}
	svc := service.NewMarketService(p)
	if _, err := svc.GetListingHistory("AAPL"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarketService_GetPortfolio_RequiresOwnerID(t *testing.T) {
	svc := service.NewMarketService(&fakeMarketProvider{})
	if _, err := svc.GetPortfolio(0, models.PortfolioOwnerTypeClient); err == nil {
		t.Fatal("expected owner id error")
	}
}

func TestMarketService_GetPortfolio_RequiresOwnerType(t *testing.T) {
	svc := service.NewMarketService(&fakeMarketProvider{})
	if _, err := svc.GetPortfolio(1, ""); err == nil {
		t.Fatal("expected owner type error")
	}
}

func TestMarketService_GetPortfolio_DelegatesToProvider(t *testing.T) {
	p := &fakeMarketProvider{portfolio: &models.Portfolio{OwnerID: 1}}
	svc := service.NewMarketService(p)
	got, err := svc.GetPortfolio(1, models.PortfolioOwnerTypeClient)
	if err != nil {
		t.Fatal(err)
	}
	if got.OwnerID != 1 {
		t.Errorf("got %v", got)
	}
}
