package service_test

import (
	"strings"
	"testing"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/repository"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/service"
)

func TestPreviousMonthPeriod_FormatYYYYMM(t *testing.T) {
	got := service.PreviousMonthPeriod()
	if len(got) != 7 || got[4] != '-' {
		t.Errorf("expected YYYY-MM, got %q", got)
	}
	expected := time.Now().UTC().AddDate(0, -1, 0).Format("2006-01")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestTaxCollector_NoUnpaidUsers(t *testing.T) {
	db := openTestDB(t, "tax_cron_empty")
	taxSvc := service.NewTaxService(repository.NewTaxRepository(db), repository.NewMarketRepository(db), &mockRateProv{})
	collector := service.NewTaxCollector(taxSvc, repository.NewOrderRepository(db), repository.NewTaxRepository(db))

	res := collector.CollectForPeriod("2026-01")
	if res.UsersProcessed != 0 {
		t.Errorf("expected 0 processed, got %d", res.UsersProcessed)
	}
	if res.TotalCollected != 0 {
		t.Errorf("expected 0 collected, got %v", res.TotalCollected)
	}
}

func TestTaxCollector_TreasuryNotFound(t *testing.T) {
	db := openTestDB(t, "tax_cron_no_treasury")
	taxRepo := repository.NewTaxRepository(db)
	period := "2026-04"

	// Seed an unpaid record so the user-list query returns one entry.
	now := time.Now().UTC()
	rec := &models.TaxRecord{
		UserID: 1, UserType: "client", AssetID: 1, Period: period,
		ProfitRSD: 1000, TaxRSD: 150, Status: "unpaid", CreatedAt: now, UpdatedAt: now,
	}
	if err := taxRepo.CreateTaxRecord(rec); err != nil {
		t.Fatal(err)
	}

	taxSvc := service.NewTaxService(taxRepo, repository.NewMarketRepository(db), &mockRateProv{})
	collector := service.NewTaxCollector(taxSvc, repository.NewOrderRepository(db), taxRepo)

	// `accounts` table doesn't exist in sqlite migration; GetStateTreasuryAccountID
	// will return a non-nil error, exercising the early-return path.
	res := collector.CollectForPeriod(period)
	if res.TotalCollected != 0 {
		t.Errorf("expected nothing collected, got %v", res.TotalCollected)
	}
}

func TestTaxCollector_ResultPeriodPropagated(t *testing.T) {
	db := openTestDB(t, "tax_cron_period")
	taxSvc := service.NewTaxService(repository.NewTaxRepository(db), repository.NewMarketRepository(db), &mockRateProv{})
	collector := service.NewTaxCollector(taxSvc, repository.NewOrderRepository(db), repository.NewTaxRepository(db))

	res := collector.CollectForPeriod("2026-04")
	if !strings.HasPrefix(res.Period, "2026-04") {
		t.Errorf("expected period to round-trip, got %q", res.Period)
	}
}
