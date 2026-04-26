package service

import (
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
)

func mkOrder(orderType, direction string) *models.OrderRecord {
	return &models.OrderRecord{OrderType: orderType, Direction: direction}
}

func TestConditionsMet_Market(t *testing.T) {
	if !conditionsMet(mkOrder("market", "buy"), &models.MarketListingRecord{}) {
		t.Error("market should always meet conditions")
	}
}

func TestConditionsMet_LimitBuy(t *testing.T) {
	o := mkOrder("limit", "buy")
	o.LimitValue = ptrFloat(100)
	if !conditionsMet(o, &models.MarketListingRecord{Ask: 99}) {
		t.Error("buy limit should trigger when ask<=limit")
	}
	if conditionsMet(o, &models.MarketListingRecord{Ask: 101}) {
		t.Error("buy limit should NOT trigger when ask>limit")
	}
}

func TestConditionsMet_LimitSell(t *testing.T) {
	o := mkOrder("limit", "sell")
	o.LimitValue = ptrFloat(100)
	if !conditionsMet(o, &models.MarketListingRecord{Bid: 101}) {
		t.Error("sell limit should trigger when bid>=limit")
	}
	if conditionsMet(o, &models.MarketListingRecord{Bid: 99}) {
		t.Error("sell limit should NOT trigger when bid<limit")
	}
}

func TestConditionsMet_StopBuy(t *testing.T) {
	o := mkOrder("stop", "buy")
	o.StopValue = ptrFloat(100)
	if !conditionsMet(o, &models.MarketListingRecord{Ask: 101}) {
		t.Error("buy stop should trigger when ask>stop")
	}
	if conditionsMet(o, &models.MarketListingRecord{Ask: 99}) {
		t.Error("buy stop should NOT trigger when ask<=stop")
	}
}

func TestConditionsMet_StopSell(t *testing.T) {
	o := mkOrder("stop", "sell")
	o.StopValue = ptrFloat(100)
	if !conditionsMet(o, &models.MarketListingRecord{Bid: 99}) {
		t.Error("sell stop should trigger when bid<stop")
	}
	if conditionsMet(o, &models.MarketListingRecord{Bid: 101}) {
		t.Error("sell stop should NOT trigger when bid>=stop")
	}
}

func TestConditionsMet_StopLimitBuy(t *testing.T) {
	o := mkOrder("stop_limit", "buy")
	o.StopValue = ptrFloat(100)
	o.LimitValue = ptrFloat(105)
	// triggered: 100 <= ask <= 105
	if !conditionsMet(o, &models.MarketListingRecord{Ask: 102}) {
		t.Error("expected triggered")
	}
	if conditionsMet(o, &models.MarketListingRecord{Ask: 99}) {
		t.Error("ask below stop should not trigger")
	}
	if conditionsMet(o, &models.MarketListingRecord{Ask: 110}) {
		t.Error("ask above limit should not trigger")
	}
}

func TestConditionsMet_StopLimitSell(t *testing.T) {
	o := mkOrder("stop_limit", "sell")
	o.StopValue = ptrFloat(100)
	o.LimitValue = ptrFloat(95)
	if !conditionsMet(o, &models.MarketListingRecord{Bid: 97}) {
		t.Error("expected triggered")
	}
	if conditionsMet(o, &models.MarketListingRecord{Bid: 105}) {
		t.Error("bid above stop should not trigger sell")
	}
	if conditionsMet(o, &models.MarketListingRecord{Bid: 90}) {
		t.Error("bid below limit should not trigger")
	}
}

func TestConditionsMet_UnknownType(t *testing.T) {
	if conditionsMet(mkOrder("foo", "buy"), &models.MarketListingRecord{}) {
		t.Error("unknown order type should not meet conditions")
	}
}

func TestFillQuantity_AON(t *testing.T) {
	o := &models.OrderRecord{IsAON: true, RemainingPortions: 5}
	if got := fillQuantity(o); got != 5 {
		t.Errorf("AON should return all 5, got %d", got)
	}
}

func TestFillQuantity_OneRemaining(t *testing.T) {
	o := &models.OrderRecord{RemainingPortions: 1}
	if got := fillQuantity(o); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
}

func TestFillQuantity_RandomInRange(t *testing.T) {
	o := &models.OrderRecord{RemainingPortions: 10}
	for i := 0; i < 30; i++ {
		got := fillQuantity(o)
		if got < 1 || got > 10 {
			t.Fatalf("fillQuantity should be in [1,10], got %d", got)
		}
	}
}

func TestExecuteFillPrice_MarketBuy(t *testing.T) {
	o := mkOrder("market", "buy")
	if executeFillPrice(o, &models.MarketListingRecord{Ask: 50, Bid: 49}) != 50 {
		t.Error("market buy should fill at ask")
	}
}

func TestExecuteFillPrice_MarketSell(t *testing.T) {
	o := mkOrder("market", "sell")
	if executeFillPrice(o, &models.MarketListingRecord{Ask: 50, Bid: 49}) != 49 {
		t.Error("market sell should fill at bid")
	}
}

func TestExecuteFillPrice_LimitBuyUsesMin(t *testing.T) {
	o := mkOrder("limit", "buy")
	o.LimitValue = ptrFloat(100)
	if executeFillPrice(o, &models.MarketListingRecord{Ask: 99}) != 99 {
		t.Error("buy limit should fill at min(limit, ask)=99")
	}
	if executeFillPrice(o, &models.MarketListingRecord{Ask: 101}) != 100 {
		t.Error("buy limit should fill at min(limit, ask)=100")
	}
}

func TestExecuteFillPrice_LimitSellUsesMax(t *testing.T) {
	o := mkOrder("limit", "sell")
	o.LimitValue = ptrFloat(100)
	if executeFillPrice(o, &models.MarketListingRecord{Bid: 101}) != 101 {
		t.Error("sell limit should fill at max(limit, bid)=101")
	}
	if executeFillPrice(o, &models.MarketListingRecord{Bid: 99}) != 100 {
		t.Error("sell limit should fill at max(limit, bid)=100")
	}
}

func TestExecuteFillPrice_StopLimit(t *testing.T) {
	o := mkOrder("stop_limit", "buy")
	o.LimitValue = ptrFloat(100)
	if executeFillPrice(o, &models.MarketListingRecord{Ask: 95}) != 95 {
		t.Error("stop_limit buy should fill at min(limit, ask)=95")
	}
}
