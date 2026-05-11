package interbank

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/repository"
)

// LocalParticipantType is the prefix we use when encoding a local
// holder identity into a ForeignBankId. The protocol's ID field is
// opaque to the partner, so we can choose any scheme — these are ours.
type LocalParticipantType string

const (
	LocalParticipantClient LocalParticipantType = "client"
	LocalParticipantBank   LocalParticipantType = "bank"
)

// EncodeLocalParticipantID packs a (userType, userID) into the opaque
// id half of a ForeignBankId. Use DecodeLocalParticipantID to
// round-trip. The encoding is "{userType}-{userID}" so partner banks
// can pattern-match it for debug logging without breaking the contract
// that the field is opaque.
func EncodeLocalParticipantID(userType LocalParticipantType, userID uint) string {
	return fmt.Sprintf("%s-%d", string(userType), userID)
}

// DecodeLocalParticipantID is the inverse of EncodeLocalParticipantID.
// Returns an error if the wire shape is unrecognised.
func DecodeLocalParticipantID(s string) (LocalParticipantType, uint, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("interbank: malformed local participant id %q", s)
	}
	pt := LocalParticipantType(parts[0])
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("interbank: malformed local participant id %q: %w", s, err)
	}
	return pt, uint(id), nil
}

// OTCHandler exposes the partner-facing OTC routes from spec §3 — the
// "browse offers and resolve display names" surface that sits next to
// the negotiation endpoints. All routes here assume AuthMiddleware has
// already attached a *PartnerBank to the request context.
type OTCHandler struct {
	registry      *Registry
	portfolioRepo *repository.PortfolioRepository
	resolver      DisplayNameResolver
}

// DisplayNameResolver maps a (LocalParticipantType, userID) to the
// person-or-firm display name to show in another bank's UI. It is
// expected to live near the auth boundary (we own the local user
// directory), so the interbank package consumes it via this
// interface rather than reaching for it directly.
type DisplayNameResolver interface {
	ResolveDisplayName(pt LocalParticipantType, userID uint) (string, error)
}

// StubDisplayNameResolver is a placeholder until exchange-service can
// reach client-service / employee-service for real names. It returns
// a synthetic but stable string so cross-bank UIs render *something*
// recognisable; real wiring is a separate task once we have the
// cross-service auth pattern settled.
type StubDisplayNameResolver struct{}

func (StubDisplayNameResolver) ResolveDisplayName(pt LocalParticipantType, userID uint) (string, error) {
	switch pt {
	case LocalParticipantClient:
		return fmt.Sprintf("Client #%d", userID), nil
	case LocalParticipantBank:
		return "EXBanka", nil
	default:
		return "", fmt.Errorf("unknown participant type %q", string(pt))
	}
}

// NewOTCHandler wires up the inter-bank OTC routes.
func NewOTCHandler(registry *Registry, portfolioRepo *repository.PortfolioRepository, resolver DisplayNameResolver) *OTCHandler {
	return &OTCHandler{
		registry:      registry,
		portfolioRepo: portfolioRepo,
		resolver:      resolver,
	}
}

// PublicStock implements GET /public-stock — aggregated list of stocks
// for which we have public OTC sellers, grouped by ticker. Per spec
// §3.4. Auth: X-Api-Key (any registered partner).
func (h *OTCHandler) PublicStock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	holdings, err := h.portfolioRepo.ListPublicOTCHoldings(0, "")
	if err != nil {
		writeProblemJSON(w, http.StatusInternalServerError, fmt.Sprintf("listing public holdings: %v", err))
		return
	}

	own := h.registry.OwnRoutingNumber()
	byTicker := map[string]*PublicStock{}
	for i := range holdings {
		hold := &holdings[i]
		amt := hold.AvailableForOTC()
		if amt <= 0 {
			continue
		}
		ticker := hold.Asset.Ticker
		if ticker == "" {
			continue
		}
		entry, ok := byTicker[ticker]
		if !ok {
			entry = &PublicStock{Stock: StockDescription{Ticker: ticker}}
			byTicker[ticker] = entry
		}
		entry.Sellers = append(entry.Sellers, PublicStockSeller{
			Seller: ForeignBankId{
				RoutingNumber: own,
				ID:            EncodeLocalParticipantID(localParticipantTypeFromHolding(hold), hold.UserID),
			},
			Amount: amt,
		})
	}

	out := make(PublicStocksResponse, 0, len(byTicker))
	for _, ps := range byTicker {
		out = append(out, *ps)
	}
	writeJSON(w, http.StatusOK, out)
}

// UserInfo implements GET /user/{routingNumber}/{id} — display-name
// resolution. Per spec §3.7. The id field is the opaque half of a
// ForeignBankId, encoded by EncodeLocalParticipantID.
//
// If the request asks about our own routing number, we resolve via
// the local user directory. If it asks about a different routing
// number, we forward to the owning partner bank — partner UIs only
// ever ask the bank that issued the id, but we accept the relay for
// resilience.
func (h *OTCHandler) UserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	routingStr, idStr, ok := pathPair(r.URL.Path, "/user/")
	if !ok {
		writeProblemJSON(w, http.StatusBadRequest, "expected path /user/{routingNumber}/{id}")
		return
	}
	routingInt, err := strconv.Atoi(routingStr)
	if err != nil {
		writeProblemJSON(w, http.StatusBadRequest, "routingNumber must be numeric")
		return
	}
	routing := RoutingNumber(routingInt)

	if routing != h.registry.OwnRoutingNumber() {
		writeProblemJSON(w, http.StatusNotFound,
			fmt.Sprintf("we are routing number %d, cannot resolve users for %d", h.registry.OwnRoutingNumber(), routing))
		return
	}

	pt, userID, decodeErr := DecodeLocalParticipantID(idStr)
	if decodeErr != nil {
		writeProblemJSON(w, http.StatusBadRequest, decodeErr.Error())
		return
	}

	displayName, err := h.resolver.ResolveDisplayName(pt, userID)
	if err != nil {
		writeProblemJSON(w, http.StatusNotFound, fmt.Sprintf("no such user: %v", err))
		return
	}

	ownPartner := bankDisplayNameForRouting(h.registry, h.registry.OwnRoutingNumber())
	writeJSON(w, http.StatusOK, UserInformation{
		BankDisplayName: ownPartner,
		DisplayName:     displayName,
	})
}

// localParticipantTypeFromHolding maps the existing portfolio user_type
// ("client" or "bank") onto our LocalParticipantType constants.
func localParticipantTypeFromHolding(h *models.PortfolioHoldingRecord) LocalParticipantType {
	switch h.UserType {
	case "client":
		return LocalParticipantClient
	case "bank":
		return LocalParticipantBank
	default:
		return LocalParticipantType(h.UserType)
	}
}

// bankDisplayNameForRouting returns a human display name for a routing
// number. For our own routing number we use a constant; for partners
// we look up Registry.Lookup. Falls back to the numeric code as a
// last resort.
func bankDisplayNameForRouting(reg *Registry, code RoutingNumber) string {
	if code == reg.OwnRoutingNumber() {
		return "EXBanka"
	}
	if p := reg.Lookup(code); p != nil && p.DisplayName != "" {
		return p.DisplayName
	}
	return fmt.Sprintf("Bank %d", code)
}

// pathPair splits a path of the form prefix + "{a}/{b}" into (a, b).
// Trailing slashes and extra segments are tolerated only when they
// exactly match the expected shape — anything else returns ok=false.
func pathPair(path, prefix string) (string, string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.TrimSuffix(rest, "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		writeProblemJSON(w, http.StatusInternalServerError, fmt.Sprintf("encoding response: %v", err))
		return
	}
	writeRaw(w, status, b)
}
