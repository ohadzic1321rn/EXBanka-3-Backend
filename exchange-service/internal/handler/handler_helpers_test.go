package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/config"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/service"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/util"
)

// --- callerIdentity ---

func TestCallerIdentity_Client(t *testing.T) {
	uid, ut := callerIdentity(&util.Claims{TokenSource: "client", ClientID: 42})
	if uid != 42 || ut != "client" {
		t.Errorf("got (%d,%q), want (42,client)", uid, ut)
	}
}

func TestCallerIdentity_EmployeeReturnsBank(t *testing.T) {
	uid, ut := callerIdentity(&util.Claims{TokenSource: "employee", EmployeeID: 99})
	if uid != bankUserID || ut != bankUserType {
		t.Errorf("got (%d,%q), want (%d,%q)", uid, ut, bankUserID, bankUserType)
	}
}

// --- isSupervisor ---

func TestIsSupervisor_True(t *testing.T) {
	c := &util.Claims{TokenSource: "employee", Permissions: []string{models.PermEmployeeSupervisor}}
	if !isSupervisor(c) {
		t.Error("expected supervisor")
	}
}

func TestIsSupervisor_FalseWhenClient(t *testing.T) {
	c := &util.Claims{TokenSource: "client", Permissions: []string{models.PermEmployeeSupervisor}}
	if isSupervisor(c) {
		t.Error("clients are not supervisors")
	}
}

func TestIsSupervisor_FalseWithoutPermission(t *testing.T) {
	c := &util.Claims{TokenSource: "employee", Permissions: []string{models.PermEmployeeAgent}}
	if isSupervisor(c) {
		t.Error("agent without supervisor perm is not supervisor")
	}
}

// --- isOwner / isHoldingOwner ---

func TestIsOwner_TrueForBankCallerOnBankOrder(t *testing.T) {
	claims := &util.Claims{TokenSource: "employee", EmployeeID: 5}
	o := &models.OrderRecord{UserID: bankUserID, UserType: bankUserType}
	if !isOwner(claims, o) {
		t.Error("expected ownership match")
	}
}

func TestIsOwner_FalseAcrossUserTypes(t *testing.T) {
	claims := &util.Claims{TokenSource: "client", ClientID: 5}
	o := &models.OrderRecord{UserID: 5, UserType: "bank"}
	if isOwner(claims, o) {
		t.Error("expected mismatch on user type")
	}
}

func TestIsHoldingOwner(t *testing.T) {
	claims := &util.Claims{TokenSource: "client", ClientID: 5}
	h := &models.PortfolioHoldingRecord{UserID: 5, UserType: "client"}
	if !isHoldingOwner(claims, h) {
		t.Error("expected match")
	}
	h.UserID = 6
	if isHoldingOwner(claims, h) {
		t.Error("expected mismatch")
	}
}

// --- requireSupervisorHTTP ---

func TestRequireSupervisorHTTP_AcceptsSupervisor(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "employee", EmployeeID: 1, Permissions: []string{models.PermEmployeeSupervisor}}
	if !requireSupervisorHTTP(rec, c) {
		t.Fatal("expected accept")
	}
}

func TestRequireSupervisorHTTP_RejectsClient(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "client", ClientID: 1}
	if requireSupervisorHTTP(rec, c) {
		t.Fatal("expected reject")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status=%d", rec.Code)
	}
}

func TestRequireSupervisorHTTP_RejectsAgentWithoutSuperPerm(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "employee", EmployeeID: 1, Permissions: []string{models.PermEmployeeAgent}}
	if requireSupervisorHTTP(rec, c) {
		t.Fatal("expected reject")
	}
}

// --- requireClientPermissionHTTP ---

func TestRequireClientPermissionHTTP_AcceptsClient(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "client", ClientID: 1, Permissions: []string{models.PermClientTrading}}
	if !requireClientPermissionHTTP(rec, c, models.PermClientTrading) {
		t.Fatal("expected accept")
	}
}

func TestRequireClientPermissionHTTP_RejectsEmployee(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "employee", EmployeeID: 1}
	if requireClientPermissionHTTP(rec, c, models.PermClientTrading) {
		t.Fatal("expected reject employee")
	}
}

func TestRequireClientPermissionHTTP_RejectsClientMissingPerm(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "client", ClientID: 1}
	if requireClientPermissionHTTP(rec, c, models.PermClientTrading) {
		t.Fatal("expected reject")
	}
}

// --- requireMarketReadAccessHTTP edge cases ---

func TestRequireMarketReadAccessHTTP_RejectsZeroClientID(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "client"}
	if requireMarketReadAccessHTTP(rec, c) {
		t.Fatal("expected reject")
	}
}

func TestRequireMarketReadAccessHTTP_RejectsZeroEmployeeID(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "employee"}
	if requireMarketReadAccessHTTP(rec, c) {
		t.Fatal("expected reject")
	}
}

func TestRequireMarketReadAccessHTTP_RejectsUnknownTokenSource(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "alien"}
	if requireMarketReadAccessHTTP(rec, c) {
		t.Fatal("expected reject")
	}
}

// --- requireTradingAccessHTTP ---

func TestRequireTradingAccessHTTP_AcceptsAgent(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "employee", EmployeeID: 1, Permissions: []string{models.PermEmployeeAgent}}
	if !requireTradingAccessHTTP(rec, c) {
		t.Fatal("expected accept agent")
	}
}

func TestRequireTradingAccessHTTP_RejectsEmployeeWithoutAgent(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "employee", EmployeeID: 1, Permissions: []string{models.PermEmployeeBasic}}
	if requireTradingAccessHTTP(rec, c) {
		t.Fatal("expected reject")
	}
}

func TestRequireTradingAccessHTTP_AcceptsClientWithTrading(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "client", ClientID: 1, Permissions: []string{models.PermClientTrading}}
	if !requireTradingAccessHTTP(rec, c) {
		t.Fatal("expected accept client")
	}
}

func TestRequireTradingAccessHTTP_RejectsClientWithoutTrading(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "client", ClientID: 1}
	if requireTradingAccessHTTP(rec, c) {
		t.Fatal("expected reject")
	}
}

func TestRequireTradingAccessHTTP_RejectsUnknownSource(t *testing.T) {
	rec := httptest.NewRecorder()
	c := &util.Claims{TokenSource: "?"}
	if requireTradingAccessHTTP(rec, c) {
		t.Fatal("expected reject")
	}
}

// --- requireAuthenticatedHTTP / parseHTTPClaims ---

func TestRequireAuthenticatedHTTP_NoToken(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	if _, ok := requireAuthenticatedHTTP(rec, r, cfg); ok {
		t.Fatal("expected unauthorized")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status=%d", rec.Code)
	}
}

func TestRequireAuthenticatedHTTP_BadToken(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	if _, ok := requireAuthenticatedHTTP(rec, r, cfg); ok {
		t.Fatal("expected unauthorized")
	}
}

// --- writeJSON ---

func TestWriteJSON_SetsContentTypeAndStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusTeapot, map[string]string{"k": "v"})
	if rec.Code != http.StatusTeapot {
		t.Errorf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Errorf("content-type=%q", rec.Header().Get("Content-Type"))
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["k"] != "v" {
		t.Errorf("body=%+v", body)
	}
}

// --- response builders ---

func TestHoldingToResponse_PreservesAssetFields(t *testing.T) {
	h := service.HoldingWithPnL{
		Holding: &models.PortfolioHoldingRecord{
			ID: 1, UserID: 0, UserType: "bank", AssetID: 9, Quantity: 5, AvgBuyPrice: 100,
			RealizedProfit: 12.5, IsPublic: true, PublicQuantity: 3, ReservedQuantity: 1, AccountID: 7, CreatedAt: time.Now().UTC(),
			Asset: models.MarketListingRecord{
				Ticker: "AAPL", Name: "Apple",
				Type: "stock", Exchange: models.MarketExchangeRecord{Acronym: "NASDAQ", Currency: "USD"},
			},
		},
		CurrentPrice: 110, MarketValue: 550, UnrealizedPnL: 50, UnrealizedPnLPct: 10,
	}
	// Set asset ID so the builder includes the asset fields.
	h.Holding.Asset.ID = 9

	resp := holdingToResponse(h)
	if resp.AssetTicker != "AAPL" || resp.AssetName != "Apple" || resp.AssetType != "stock" {
		t.Errorf("asset fields not propagated: %+v", resp)
	}
	if resp.MarketValue != 550 || resp.UnrealizedPnL != 50 {
		t.Errorf("pnl fields not propagated: %+v", resp)
	}
	if resp.PublicQuantity != 3 || resp.ReservedQuantity != 1 || resp.AvailableForOTC != 2 {
		t.Errorf("OTC fields not propagated: %+v", resp)
	}
}

func TestHoldingToResponse_EmptyAssetIsBlank(t *testing.T) {
	h := service.HoldingWithPnL{
		Holding: &models.PortfolioHoldingRecord{ID: 1, Quantity: 1, CreatedAt: time.Now().UTC()},
	}
	resp := holdingToResponse(h)
	if resp.AssetTicker != "" || resp.AssetName != "" {
		t.Errorf("expected empty asset fields, got %+v", resp)
	}
}

func TestBuildPortfolioSummary_AggregatesTotals(t *testing.T) {
	now := time.Now().UTC()
	h1 := service.HoldingWithPnL{
		Holding:     &models.PortfolioHoldingRecord{ID: 1, RealizedProfit: 10, CreatedAt: now},
		MarketValue: 100, UnrealizedPnL: 5,
	}
	h2 := service.HoldingWithPnL{
		Holding:     &models.PortfolioHoldingRecord{ID: 2, RealizedProfit: 20, CreatedAt: now},
		MarketValue: 200, UnrealizedPnL: 15,
	}
	summary := buildPortfolioSummary(0, "bank", []service.HoldingWithPnL{h1, h2})
	if summary.PositionCount != 2 {
		t.Errorf("positions=%d", summary.PositionCount)
	}
	if summary.EstimatedValue != 300 {
		t.Errorf("estimated=%v", summary.EstimatedValue)
	}
	if summary.UnrealizedPnL != 20 {
		t.Errorf("unrealized=%v", summary.UnrealizedPnL)
	}
	if summary.RealizedProfit != 30 {
		t.Errorf("realized=%v", summary.RealizedProfit)
	}
	if summary.OwnerType != "bank" || summary.OwnerID != "0" {
		t.Errorf("owner=%v/%v", summary.OwnerID, summary.OwnerType)
	}
}

func TestRound2_HandlerVariant(t *testing.T) {
	if v := round2(1.234); v != 1.23 {
		t.Errorf("got %v", v)
	}
	if v := round2(1.236); v != 1.24 {
		t.Errorf("got %v", v)
	}
}

// --- orderToResponse ---

func TestOrderToResponse_PropagatesAssetAndFlags(t *testing.T) {
	now := time.Now().UTC()
	o := &models.OrderRecord{
		ID: 1, UserID: 0, UserType: "bank", OrderType: "limit", Direction: "buy",
		Quantity: 5, ContractSize: 1, PricePerUnit: 100, Status: "approved",
		IsAON: true, IsMargin: true, RemainingPortions: 5, Commission: 5,
		AccountID: 9, AfterHours: true, LastModification: now, CreatedAt: now,
		Asset: models.MarketListingRecord{Ticker: "AAPL", Name: "Apple"},
	}
	r := orderToResponse(o)
	if r.AssetTicker != "AAPL" || r.AssetName != "Apple" {
		t.Errorf("asset not propagated: %+v", r)
	}
	if !r.IsAON || !r.IsMargin || !r.AfterHours {
		t.Errorf("flags not propagated: %+v", r)
	}
}

// --- isCallerSelf (tax_http_handler) ---

func TestIsCallerSelf_True(t *testing.T) {
	c := &util.Claims{TokenSource: "client", ClientID: 5}
	if !isCallerSelf(c, 5, "client") {
		t.Error("expected match")
	}
}

func TestIsCallerSelf_FalseDifferentID(t *testing.T) {
	c := &util.Claims{TokenSource: "client", ClientID: 5}
	if isCallerSelf(c, 6, "client") {
		t.Error("expected mismatch")
	}
}
