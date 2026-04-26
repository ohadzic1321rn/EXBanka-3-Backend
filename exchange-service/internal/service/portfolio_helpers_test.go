package service

import (
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
)

func TestRoundPnL(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{1.234, 1.23},
		{1.236, 1.24},
		{0, 0},
		{-1.236, -1.24},
	}
	for _, c := range cases {
		if got := roundPnL(c.in); got != c.want {
			t.Errorf("roundPnL(%v)=%v, want %v", c.in, got, c.want)
		}
	}
}

func TestComputePnL_Profit(t *testing.T) {
	h := &models.PortfolioHoldingRecord{
		Quantity:    10,
		AvgBuyPrice: 100,
		Asset:       models.MarketListingRecord{Price: 110},
	}
	pnl := computePnL(h)
	if pnl.CurrentPrice != 110 {
		t.Errorf("CurrentPrice=%v, want 110", pnl.CurrentPrice)
	}
	if pnl.MarketValue != 1100 {
		t.Errorf("MarketValue=%v, want 1100", pnl.MarketValue)
	}
	if pnl.UnrealizedPnL != 100 {
		t.Errorf("UnrealizedPnL=%v, want 100", pnl.UnrealizedPnL)
	}
	if pnl.UnrealizedPnLPct != 10 {
		t.Errorf("UnrealizedPnLPct=%v, want 10", pnl.UnrealizedPnLPct)
	}
}

func TestComputePnL_Loss(t *testing.T) {
	h := &models.PortfolioHoldingRecord{
		Quantity:    5,
		AvgBuyPrice: 200,
		Asset:       models.MarketListingRecord{Price: 180},
	}
	pnl := computePnL(h)
	if pnl.UnrealizedPnL != -100 {
		t.Errorf("UnrealizedPnL=%v, want -100", pnl.UnrealizedPnL)
	}
	if pnl.UnrealizedPnLPct != -10 {
		t.Errorf("UnrealizedPnLPct=%v, want -10", pnl.UnrealizedPnLPct)
	}
}

func TestComputePnL_ZeroAvgBuyPrice(t *testing.T) {
	h := &models.PortfolioHoldingRecord{
		Quantity:    1,
		AvgBuyPrice: 0,
		Asset:       models.MarketListingRecord{Price: 100},
	}
	pnl := computePnL(h)
	if pnl.UnrealizedPnLPct != 0 {
		t.Errorf("expected 0%% when avg buy price is 0, got %v", pnl.UnrealizedPnLPct)
	}
}
