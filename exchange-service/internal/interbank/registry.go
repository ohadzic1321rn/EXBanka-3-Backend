package interbank

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// PartnerBank holds the per-partner config we need to (a) route account numbers,
// (b) authenticate inbound requests from that partner, and (c) authenticate
// outbound requests to that partner.
type PartnerBank struct {
	// Code is the 3-digit routing number that identifies the partner bank
	// (= first three digits of any account number they own).
	Code RoutingNumber `json:"code"`

	// BaseURL is the partner's HTTP root, no trailing slash. Outbound message
	// envelopes are POSTed to {BaseURL}/interbank; OTC requests to
	// {BaseURL}/negotiations etc.
	BaseURL string `json:"baseUrl"`

	// OutboundKey is the X-Api-Key the PARTNER issued to US. We send it on
	// every outbound request to them.
	OutboundKey string `json:"outboundKey"`

	// InboundKey is the X-Api-Key WE issued to the partner. We require it on
	// every inbound request from them.
	InboundKey string `json:"inboundKey"`

	// DisplayName is the human-friendly name (for UI labels).
	DisplayName string `json:"displayName,omitempty"`
}

// Registry is the in-memory partner-bank index. Construct once at startup with
// NewRegistryFromJSON and treat as read-only after that.
type Registry struct {
	mu               sync.RWMutex
	ownRouting       RoutingNumber
	byCode           map[RoutingNumber]*PartnerBank
	byInboundKey     map[string]*PartnerBank
}

// NewRegistryFromJSON parses a JSON array of PartnerBank entries and validates
// that none of them collide on Code or InboundKey with each other or with our
// own routing number.
func NewRegistryFromJSON(ownRouting RoutingNumber, partnersJSON string) (*Registry, error) {
	r := &Registry{
		ownRouting:   ownRouting,
		byCode:       make(map[RoutingNumber]*PartnerBank),
		byInboundKey: make(map[string]*PartnerBank),
	}
	if strings.TrimSpace(partnersJSON) == "" {
		return r, nil
	}
	var partners []PartnerBank
	if err := json.Unmarshal([]byte(partnersJSON), &partners); err != nil {
		return nil, fmt.Errorf("interbank.NewRegistryFromJSON: invalid JSON: %w", err)
	}
	for i := range partners {
		p := &partners[i]
		if p.Code == 0 {
			return nil, fmt.Errorf("interbank.NewRegistryFromJSON: partner #%d missing code", i)
		}
		if p.Code == ownRouting {
			return nil, fmt.Errorf("interbank.NewRegistryFromJSON: partner #%d code %d collides with own routing number", i, p.Code)
		}
		if strings.TrimSpace(p.BaseURL) == "" {
			return nil, fmt.Errorf("interbank.NewRegistryFromJSON: partner code %d missing baseUrl", p.Code)
		}
		p.BaseURL = strings.TrimRight(p.BaseURL, "/")
		if p.InboundKey == "" || p.OutboundKey == "" {
			return nil, fmt.Errorf("interbank.NewRegistryFromJSON: partner code %d missing inboundKey or outboundKey", p.Code)
		}
		if _, dup := r.byCode[p.Code]; dup {
			return nil, fmt.Errorf("interbank.NewRegistryFromJSON: duplicate partner code %d", p.Code)
		}
		if _, dup := r.byInboundKey[p.InboundKey]; dup {
			return nil, fmt.Errorf("interbank.NewRegistryFromJSON: duplicate inbound key (would let two partners impersonate each other)")
		}
		r.byCode[p.Code] = p
		r.byInboundKey[p.InboundKey] = p
	}
	return r, nil
}

// OwnRoutingNumber returns the routing number we identify as.
func (r *Registry) OwnRoutingNumber() RoutingNumber {
	return r.ownRouting
}

// Lookup returns the partner with the given routing number, or nil if not registered.
func (r *Registry) Lookup(code RoutingNumber) *PartnerBank {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byCode[code]
}

// LookupByInboundKey returns the partner that owns the given X-Api-Key header
// value (the key WE issued to them), or nil if the key is not recognised.
func (r *Registry) LookupByInboundKey(key string) *PartnerBank {
	if key == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byInboundKey[key]
}

// All returns a copy of every partner in the registry. Caller may mutate.
func (r *Registry) All() []PartnerBank {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PartnerBank, 0, len(r.byCode))
	for _, p := range r.byCode {
		out = append(out, *p)
	}
	return out
}

// RoutingNumberFromAccount returns the 3-digit routing prefix of an account
// number. Errors if the account number is shorter than 3 chars or its prefix
// is not numeric.
func RoutingNumberFromAccount(accountNumber string) (RoutingNumber, error) {
	s := strings.TrimSpace(accountNumber)
	if len(s) < 3 {
		return 0, fmt.Errorf("interbank: account number %q too short to extract routing number", accountNumber)
	}
	n, err := strconv.Atoi(s[:3])
	if err != nil {
		return 0, fmt.Errorf("interbank: account number %q does not start with a numeric routing prefix: %w", accountNumber, err)
	}
	return RoutingNumber(n), nil
}

// ResolveBankFromAccount inspects the first 3 digits of accountNumber and
// returns the matching routing number, the partner's base URL, and whether
// the account is local. If the account is non-local but no partner is
// registered for that prefix, returns an error.
func (r *Registry) ResolveBankFromAccount(accountNumber string) (code RoutingNumber, baseURL string, isLocal bool, err error) {
	code, err = RoutingNumberFromAccount(accountNumber)
	if err != nil {
		return 0, "", false, err
	}
	if code == r.ownRouting {
		return code, "", true, nil
	}
	p := r.Lookup(code)
	if p == nil {
		return code, "", false, fmt.Errorf("interbank: no partner registered for routing number %d (account %q)", code, accountNumber)
	}
	return code, p.BaseURL, false, nil
}
