package provider_test

import (
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/provider"
)

func TestStaticProvider_GetRate_ReturnsPositive(t *testing.T) {
	p := provider.NewStaticRateProvider()
	rate, err := p.GetRate("EUR", "RSD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate <= 0 {
		t.Errorf("expected positive rate, got %f", rate)
	}
}

func TestStaticProvider_GetRate_SameCurrency_ReturnsOne(t *testing.T) {
	p := provider.NewStaticRateProvider()
	rate, err := p.GetRate("EUR", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 1.0 {
		t.Errorf("expected 1.0, got %f", rate)
	}
}

func TestStaticProvider_GetAllRates_HasBuyAndSellRates(t *testing.T) {
	p := provider.NewStaticRateProvider()
	rates := p.GetAllRates()
	if len(rates) == 0 {
		t.Fatal("expected non-empty rate list")
	}
	for _, r := range rates {
		if r.BuyRate <= 0 {
			t.Errorf("pair %s→%s: expected positive BuyRate, got %f", r.From, r.To, r.BuyRate)
		}
		if r.SellRate <= 0 {
			t.Errorf("pair %s→%s: expected positive SellRate, got %f", r.From, r.To, r.SellRate)
		}
	}
}

func TestStaticProvider_SellRateHigherThanBuyRate(t *testing.T) {
	p := provider.NewStaticRateProvider()
	rates := p.GetAllRates()
	for _, r := range rates {
		if r.SellRate <= r.BuyRate {
			t.Errorf("pair %s→%s: expected SellRate(%f) > BuyRate(%f)", r.From, r.To, r.SellRate, r.BuyRate)
		}
	}
}

func TestStaticProvider_SpreadWithinReasonableBounds(t *testing.T) {
	p := provider.NewStaticRateProvider()
	rates := p.GetAllRates()
	for _, r := range rates {
		// Spread should be ≤ 5% of mid rate
		spread := (r.SellRate - r.BuyRate) / r.Rate
		if spread > 0.05 {
			t.Errorf("pair %s→%s: spread %f%% exceeds 5%%", r.From, r.To, spread*100)
		}
	}
}

func TestStaticProvider_RateIsMidpointOfBuySell(t *testing.T) {
	p := provider.NewStaticRateProvider()
	rates := p.GetAllRates()
	for _, r := range rates {
		mid := (r.BuyRate + r.SellRate) / 2.0
		diff := (mid - r.Rate)
		if diff < 0 {
			diff = -diff
		}
		if diff > r.Rate*0.001 { // within 0.1% tolerance
			t.Errorf("pair %s→%s: Rate(%f) not midpoint of Buy(%f)/Sell(%f)", r.From, r.To, r.Rate, r.BuyRate, r.SellRate)
		}
	}
}
