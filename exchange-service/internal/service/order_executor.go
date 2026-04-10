package service

import (
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/repository"
)

const afterHoursDelay = 30 * time.Minute

// OrderExecutor fills active orders on each cron tick.
type OrderExecutor struct {
	orderRepo    *repository.OrderRepository
	marketRepo   *repository.MarketRepository
	portfolioSvc *PortfolioService
	rateProvider RateProviderInterface
}

func NewOrderExecutor(orderRepo *repository.OrderRepository, marketRepo *repository.MarketRepository, portfolioSvc *PortfolioService, rateProvider RateProviderInterface) *OrderExecutor {
	return &OrderExecutor{orderRepo: orderRepo, marketRepo: marketRepo, portfolioSvc: portfolioSvc, rateProvider: rateProvider}
}

// Run processes all approved, not-done orders and partially or fully fills them
// when their execution conditions are met.
func (e *OrderExecutor) Run() {
	orders, err := e.orderRepo.ListPendingActiveOrders()
	if err != nil {
		slog.Error("order executor: failed to load active orders", "error", err)
		return
	}

	for _, order := range orders {
		// Only execute approved orders; pending ones wait for supervisor action.
		if order.Status != "approved" {
			continue
		}

		listing := &order.Asset // preloaded by ListPendingActiveOrders

		// Check if order conditions are currently satisfied.
		if !conditionsMet(&order, listing) {
			continue
		}

		// After-hours orders: enforce a 30-minute gap between consecutive fills.
		if order.AfterHours && time.Since(order.LastModification) < afterHoursDelay {
			continue
		}

		// Determine fill quantity and price.
		fillQty := fillQuantity(&order)
		price := executeFillPrice(&order, listing)

		// Persist the fill event.
		txRecord := &models.OrderTransactionRecord{
			OrderID:      order.ID,
			Quantity:     fillQty,
			PricePerUnit: price,
			ExecutedAt:   time.Now().UTC(),
		}
		if err := e.orderRepo.CreateOrderTransaction(txRecord); err != nil {
			slog.Error("order executor: failed to create transaction", "orderID", order.ID, "error", err)
			continue
		}

		// Decrement remaining portions; marks order done if it reaches 0.
		if err := e.orderRepo.DecrementRemainingPortions(order.ID, fillQty); err != nil {
			slog.Error("order executor: failed to decrement portions", "orderID", order.ID, "error", err)
			continue
		}

		// Update portfolio holdings to reflect the fill.
		if err := e.portfolioSvc.RecordFill(&order, fillQty, price); err != nil {
			slog.Error("order executor: failed to update portfolio", "orderID", order.ID, "error", err)
			// Non-fatal: order fill is committed; portfolio can be reconciled later.
		}

		// Credit account for sell fills, converting to account currency if needed.
		// Commission is deducted from the proceeds and credited to the bank account.
		if order.Direction == "sell" {
			fillAmount := round2(float64(fillQty) * float64(order.ContractSize) * price)
			assetCurrency := order.Asset.Exchange.Currency

			// Proportional commission for this partial fill (in asset trading currency).
			fillCommission := round2(order.Commission * float64(fillQty) / float64(order.Quantity))

			// Credit bank account with the commission in the asset's trading currency.
			if fillCommission > 0 {
				bankAccountID, err := e.orderRepo.GetBankAccountByCurrency(assetCurrency)
				if err != nil {
					slog.Error("order executor: failed to find bank account for sell commission", "orderID", order.ID, "currency", assetCurrency, "error", err)
				} else if bankAccountID > 0 {
					if err := e.orderRepo.CreditAccount(bankAccountID, fillCommission); err != nil {
						slog.Error("order executor: failed to credit bank sell commission", "orderID", order.ID, "currency", assetCurrency, "amount", fillCommission, "error", err)
					}
				} else {
					slog.Warn("order executor: no bank account found for currency, sell commission not credited", "orderID", order.ID, "currency", assetCurrency)
				}
			}

			// Net proceeds after commission, converted to account currency if needed.
			netAmount := round2(fillAmount - fillCommission)
			_, accountCurrency, err := e.orderRepo.GetAccountBalance(order.AccountID)
			if err != nil {
				slog.Error("order executor: failed to get account currency on sell", "orderID", order.ID, "error", err)
			} else if assetCurrency != accountCurrency {
				rate, err := e.rateProvider.GetRate(assetCurrency, accountCurrency)
				if err != nil || rate == 0 {
					slog.Error("order executor: no forex rate for sell proceeds", "orderID", order.ID, "from", assetCurrency, "to", accountCurrency)
				} else {
					netAmount = round2(netAmount * rate)
				}
			}
			if err := e.orderRepo.CreditAccount(order.AccountID, netAmount); err != nil {
				slog.Error("order executor: failed to credit account on sell", "orderID", order.ID, "error", err)
			}
		}

		// Credit the bank's account with the proportional commission for buy fills.
		// Commission was already deducted from the client's account at order creation.
		// The commission is in the asset's trading currency (exchange currency), not the user's account currency.
		if order.Direction == "buy" && order.Commission > 0 {
			fillCommission := round2(order.Commission * float64(fillQty) / float64(order.Quantity))
			if fillCommission > 0 {
				currencyKod := order.Asset.Exchange.Currency
				bankAccountID, err := e.orderRepo.GetBankAccountByCurrency(currencyKod)
				if err != nil {
					slog.Error("order executor: failed to find bank account for commission", "orderID", order.ID, "currency", currencyKod, "error", err)
				} else if bankAccountID > 0 {
					if err := e.orderRepo.CreditAccount(bankAccountID, fillCommission); err != nil {
						slog.Error("order executor: failed to credit bank commission", "orderID", order.ID, "currency", currencyKod, "amount", fillCommission, "error", err)
					}
				} else {
					slog.Warn("order executor: no bank account found for currency, commission not credited", "orderID", order.ID, "currency", currencyKod)
				}
			}
		}

		slog.Info("order executor: filled",
			"orderID", order.ID,
			"type", order.OrderType,
			"direction", order.Direction,
			"filled", fillQty,
			"price", price,
			"remaining", order.RemainingPortions-fillQty,
		)
	}
}

// conditionsMet reports whether current market prices satisfy the order's
// execution trigger.
//
//	market:     always executable
//	limit buy:  ask <= limit_value
//	limit sell: bid >= limit_value
//	stop buy:   ask > stop_value  (then executes at market)
//	stop sell:  bid < stop_value  (then executes at market)
//	stop_limit: stop must trigger AND limit condition must be satisfied
func conditionsMet(order *models.OrderRecord, listing *models.MarketListingRecord) bool {
	switch order.OrderType {
	case "market":
		return true
	case "limit":
		if order.Direction == "buy" {
			return listing.Ask <= *order.LimitValue
		}
		return listing.Bid >= *order.LimitValue
	case "stop":
		if order.Direction == "buy" {
			return listing.Ask > *order.StopValue
		}
		return listing.Bid < *order.StopValue
	case "stop_limit":
		// Stop triggers activation; limit guards the fill price.
		// Buy:  Ask reaches or exceeds stop value, AND Ask is still within limit.
		// Sell: Bid falls below stop value, AND Bid is still within limit.
		if order.Direction == "buy" {
			return listing.Ask >= *order.StopValue && listing.Ask <= *order.LimitValue
		}
		return listing.Bid < *order.StopValue && listing.Bid >= *order.LimitValue
	}
	return false
}

// fillQuantity returns how many units to fill in this cycle.
// AON orders must fill the entire remaining quantity at once or not at all.
// Regular orders fill a random portion: rand[1, remaining_portions].
func fillQuantity(order *models.OrderRecord) int64 {
	if order.IsAON {
		return order.RemainingPortions
	}
	if order.RemainingPortions == 1 {
		return 1
	}
	return rand.Int63n(order.RemainingPortions) + 1
}

// executeFillPrice returns the actual per-unit price for a fill.
//
//	market / stop:      current ask (buy) or bid (sell)
//	limit / stop_limit: min(limit_value, ask) for buy; max(limit_value, bid) for sell
func executeFillPrice(order *models.OrderRecord, listing *models.MarketListingRecord) float64 {
	switch order.OrderType {
	case "limit", "stop_limit":
		if order.Direction == "buy" {
			return math.Min(*order.LimitValue, listing.Ask)
		}
		return math.Max(*order.LimitValue, listing.Bid)
	default: // market, stop
		if order.Direction == "buy" {
			return listing.Ask
		}
		return listing.Bid
	}
}
