package service

import (
	"fmt"
	"math"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/repository"
)

const capitalGainsTaxRate = 0.15

// PortfolioService maintains PortfolioHoldingRecords in response to order fills
// and exposes P&L views over the live holdings.
type PortfolioService struct {
	portfolioRepo *repository.PortfolioRepository
	taxRepo       *repository.TaxRepository
}

func NewPortfolioService(portfolioRepo *repository.PortfolioRepository, taxRepo *repository.TaxRepository) *PortfolioService {
	return &PortfolioService{portfolioRepo: portfolioRepo, taxRepo: taxRepo}
}

// HoldingWithPnL enriches a PortfolioHoldingRecord with live unrealized P&L
// computed from the holding's preloaded Asset.Price.
type HoldingWithPnL struct {
	Holding          *models.PortfolioHoldingRecord
	CurrentPrice     float64
	UnrealizedPnL    float64 // (currentPrice - avgBuyPrice) * quantity
	UnrealizedPnLPct float64 // as a percentage of cost basis
	MarketValue      float64 // currentPrice * quantity
}

// RecordFill updates the portfolio after an order fill and creates a TaxRecord
// for any realised profit on a sell.
//
// Buy fill: upsert holding — increase quantity, recalculate weighted avg buy price.
// Sell fill: decrease quantity, accumulate realized profit, create TaxRecord if profit > 0.
//
// Effective quantity = fillQty × contractSize.
func (s *PortfolioService) RecordFill(order *models.OrderRecord, fillQty int64, fillPrice float64) error {
	effectiveQty := float64(fillQty) * float64(order.ContractSize)

	if order.Direction == "buy" {
		if err := s.portfolioRepo.RecordBuyFill(
			order.UserID, order.UserType, order.AssetID, order.AccountID,
			effectiveQty, fillPrice,
		); err != nil {
			return fmt.Errorf("portfolio: failed to record buy fill for order %d: %w", order.ID, err)
		}
		return nil
	}

	// Sell: decrease holding and accumulate realized profit.
	realizedProfit, err := s.portfolioRepo.RecordSellFill(
		order.UserID, order.UserType, order.AssetID, effectiveQty, fillPrice,
	)
	if err != nil {
		return fmt.Errorf("portfolio: failed to record sell fill for order %d: %w", order.ID, err)
	}

	// Create a TaxRecord for any capital gain.
	// profit_rsd and tax_rsd store the profit in the asset's native currency;
	// Phase 7 (TaxService) applies the RSD conversion before collection.
	if realizedProfit > 0 {
		tax := realizedProfit * capitalGainsTaxRate
		now := time.Now().UTC()
		record := &models.TaxRecord{
			UserID:    order.UserID,
			UserType:  order.UserType,
			AssetID:   order.AssetID,
			Period:    now.Format("2006-01"),
			ProfitRSD: roundPnL(realizedProfit),
			TaxRSD:    roundPnL(tax),
			Status:    "unpaid",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.taxRepo.CreateTaxRecord(record); err != nil {
			// Non-fatal: tax record failure should not block the fill.
			// Phase 7 cron can reconcile from realized_profit on the holding.
			_ = err
		}
	}

	return nil
}

// ListHoldingsWithPnL returns all active holdings for a user enriched with
// live unrealized P&L computed from the preloaded Asset.Price.
func (s *PortfolioService) ListHoldingsWithPnL(userID uint, userType string) ([]HoldingWithPnL, error) {
	holdings, err := s.portfolioRepo.ListHoldingsForUser(userID, userType)
	if err != nil {
		return nil, err
	}

	result := make([]HoldingWithPnL, 0, len(holdings))
	for i := range holdings {
		result = append(result, computePnL(&holdings[i]))
	}
	return result, nil
}

// GetHoldingWithPnL returns a single holding enriched with live P&L.
func (s *PortfolioService) GetHoldingWithPnL(id uint) (*HoldingWithPnL, error) {
	h, err := s.portfolioRepo.GetHoldingByID(id)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, fmt.Errorf("holding not found")
	}
	pnl := computePnL(h)
	return &pnl, nil
}

// GetHoldingByID returns a single holding by ID.
func (s *PortfolioService) GetHoldingByID(id uint) (*models.PortfolioHoldingRecord, error) {
	h, err := s.portfolioRepo.GetHoldingByID(id)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, fmt.Errorf("holding not found")
	}
	return h, nil
}

// ListHoldings returns all active holdings for a user.
func (s *PortfolioService) ListHoldings(userID uint, userType string) ([]models.PortfolioHoldingRecord, error) {
	return s.portfolioRepo.ListHoldingsForUser(userID, userType)
}

// SetPublic toggles the is_public flag on a holding.
func (s *PortfolioService) SetPublic(holdingID uint, isPublic bool) error {
	return s.portfolioRepo.SetHoldingPublic(holdingID, isPublic)
}

// --- helpers ---

// computePnL enriches a holding with unrealized P&L using the preloaded asset price.
func computePnL(h *models.PortfolioHoldingRecord) HoldingWithPnL {
	currentPrice := h.Asset.Price
	unrealized := roundPnL((currentPrice - h.AvgBuyPrice) * h.Quantity)
	marketValue := roundPnL(currentPrice * h.Quantity)

	pct := 0.0
	if h.AvgBuyPrice > 0 {
		pct = roundPnL(((currentPrice - h.AvgBuyPrice) / h.AvgBuyPrice) * 100)
	}

	return HoldingWithPnL{
		Holding:          h,
		CurrentPrice:     currentPrice,
		UnrealizedPnL:    unrealized,
		UnrealizedPnLPct: pct,
		MarketValue:      marketValue,
	}
}

func roundPnL(v float64) float64 {
	return math.Round(v*100) / 100
}
