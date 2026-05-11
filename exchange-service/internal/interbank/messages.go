// Package interbank holds the wire-level types and transport for bank-to-bank
// asset exchange, per https://arsen.srht.site/si-tx-proto/notes.html.
//
// Type names mirror the protocol's TypeScript definitions one-to-one so that a
// reader can cross-reference the spec without translation. JSON struct tags
// are the source of truth for the wire format; Go-side field names are
// PascalCase per Go convention.
//
// All monetary `amount` fields use float64 here. The protocol spec recommends
// BigDecimal-style precision and warns "Do not interpret amount as a float64";
// the existing exchange-service codebase uses float64 throughout for prices,
// and switching to an arbitrary-precision decimal type is a larger change
// best done after we have a precision-sensitive test that fails.
package interbank

import (
	"encoding/json"
	"errors"
	"fmt"
)

// =====================
// Atomic types (spec §2.1–§2.5)
// =====================

// RoutingNumber is the 3-digit bank identifier — the first three digits of an
// account number, well-known and assigned ahead of time.
type RoutingNumber int

// IdempotenceKey deduplicates inter-bank messages. The receiver MUST persist
// these indefinitely and replay the cached response on a duplicate. The sender
// MUST set its own RoutingNumber and generate LocallyGeneratedKey (≤ 64 bytes)
// in any way it sees fit.
type IdempotenceKey struct {
	RoutingNumber       RoutingNumber `json:"routingNumber"`
	LocallyGeneratedKey string        `json:"locallyGeneratedKey"`
}

// ForeignBankId names an object owned by a specific bank. The "id" field is
// opaque — only the bank whose routingNumber equals RoutingNumber may
// interpret it. Max 64 bytes.
type ForeignBankId struct {
	RoutingNumber RoutingNumber `json:"routingNumber"`
	ID            string        `json:"id"`
}

// ISO8601DateTimeWithTimeZone is a string like "2025-04-16T15:32:44+02:00".
// Kept as a string at the protocol boundary; callers parse via time.RFC3339.
type ISO8601DateTimeWithTimeZone = string

// CurrencyCode is a closed enum (spec §2.7.1).
type CurrencyCode string

const (
	CurrencyRSD CurrencyCode = "RSD"
	CurrencyEUR CurrencyCode = "EUR"
	CurrencyUSD CurrencyCode = "USD"
	CurrencyCHF CurrencyCode = "CHF"
	CurrencyJPY CurrencyCode = "JPY"
	CurrencyAUD CurrencyCode = "AUD"
	CurrencyCAD CurrencyCode = "CAD"
	CurrencyGBP CurrencyCode = "GBP"
)

func IsKnownCurrency(c CurrencyCode) bool {
	switch c {
	case CurrencyRSD, CurrencyEUR, CurrencyUSD, CurrencyCHF, CurrencyJPY,
		CurrencyAUD, CurrencyCAD, CurrencyGBP:
		return true
	}
	return false
}

// MonetaryValue is an amount in a given currency (spec §2.5).
type MonetaryValue struct {
	Currency CurrencyCode `json:"currency"`
	Amount   float64      `json:"amount"`
}

// =====================
// Accounts (spec §2.6)
// =====================

// CurrencyAccountNumber is a local currency-account number string.
type CurrencyAccountNumber = string

// TxAccountType discriminates between the three TxAccount variants.
type TxAccountType string

const (
	TxAccountPerson  TxAccountType = "PERSON"
	TxAccountAccount TxAccountType = "ACCOUNT"
	TxAccountOption  TxAccountType = "OPTION"
)

// TxAccount is a tagged union. For PERSON and OPTION, ID is populated.
// For ACCOUNT, Num is populated. Exactly one applies — Type is the discriminator.
type TxAccount struct {
	Type TxAccountType         `json:"type"`
	ID   *ForeignBankId        `json:"id,omitempty"`
	Num  CurrencyAccountNumber `json:"num,omitempty"`
}

// =====================
// Assets (spec §2.7)
// =====================

type AssetType string

const (
	AssetMonas  AssetType = "MONAS"
	AssetStock  AssetType = "STOCK"
	AssetOption AssetType = "OPTION"
)

type MonetaryAsset struct {
	Currency CurrencyCode `json:"currency"`
}

type StockDescription struct {
	Ticker string `json:"ticker"`
}

// OptionDescription describes an OTC option contract.
// NegotiationID always points at the seller's bank — see §2.7.2.
type OptionDescription struct {
	NegotiationID  ForeignBankId               `json:"negotiationId"`
	Stock          StockDescription            `json:"stock"`
	PricePerUnit   MonetaryValue               `json:"pricePerUnit"`
	SettlementDate ISO8601DateTimeWithTimeZone `json:"settlementDate"`
	Amount         float64                     `json:"amount"`
}

// Asset is a tagged union whose wire form is { "type": ..., "asset": ... }.
// Exactly one of Monas/Stock/Option is populated, determined by Type.
type Asset struct {
	Type   AssetType
	Monas  *MonetaryAsset
	Stock  *StockDescription
	Option *OptionDescription
}

type assetWire struct {
	Type  AssetType       `json:"type"`
	Asset json.RawMessage `json:"asset"`
}

func (a Asset) MarshalJSON() ([]byte, error) {
	var inner any
	switch a.Type {
	case AssetMonas:
		if a.Monas == nil {
			return nil, errors.New("interbank.Asset: type=MONAS but Monas is nil")
		}
		inner = a.Monas
	case AssetStock:
		if a.Stock == nil {
			return nil, errors.New("interbank.Asset: type=STOCK but Stock is nil")
		}
		inner = a.Stock
	case AssetOption:
		if a.Option == nil {
			return nil, errors.New("interbank.Asset: type=OPTION but Option is nil")
		}
		inner = a.Option
	default:
		return nil, fmt.Errorf("interbank.Asset: unknown type %q", string(a.Type))
	}
	body, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}
	return json.Marshal(assetWire{Type: a.Type, Asset: body})
}

func (a *Asset) UnmarshalJSON(b []byte) error {
	var wire assetWire
	if err := json.Unmarshal(b, &wire); err != nil {
		return err
	}
	a.Type = wire.Type
	a.Monas, a.Stock, a.Option = nil, nil, nil
	switch wire.Type {
	case AssetMonas:
		var v MonetaryAsset
		if err := json.Unmarshal(wire.Asset, &v); err != nil {
			return err
		}
		a.Monas = &v
	case AssetStock:
		var v StockDescription
		if err := json.Unmarshal(wire.Asset, &v); err != nil {
			return err
		}
		a.Stock = &v
	case AssetOption:
		var v OptionDescription
		if err := json.Unmarshal(wire.Asset, &v); err != nil {
			return err
		}
		a.Option = &v
	default:
		return fmt.Errorf("interbank.Asset: unknown type %q", string(wire.Type))
	}
	return nil
}

// =====================
// Postings & Transactions (spec §2.8)
// =====================

type Posting struct {
	Account TxAccount `json:"account"`
	Amount  float64   `json:"amount"`
	Asset   Asset     `json:"asset"`
}

// Transaction is a balanced collection of postings, with metadata.
// CallNumber is optional per the spec (the only "?:" field).
type Transaction struct {
	Postings       []Posting     `json:"postings"`
	TransactionID  ForeignBankId `json:"transactionId"`
	Message        string        `json:"message"`
	CallNumber     string        `json:"callNumber,omitempty"`
	PaymentCode    string        `json:"paymentCode"`
	PaymentPurpose string        `json:"paymentPurpose"`
}

type CommitTransaction struct {
	TransactionID ForeignBankId `json:"transactionId"`
}

type RollbackTransaction struct {
	TransactionID ForeignBankId `json:"transactionId"`
}

// =====================
// Message envelope (spec §2.9–§2.12)
// =====================

type MessageType string

const (
	MessageTypeNewTx      MessageType = "NEW_TX"
	MessageTypeCommitTx   MessageType = "COMMIT_TX"
	MessageTypeRollbackTx MessageType = "ROLLBACK_TX"
)

// Message is the envelope POSTed to /interbank. The Body field is left as
// RawMessage at the envelope level; callers decode it into the type indicated
// by MessageType using the Decode* helpers.
type Message struct {
	IdempotenceKey IdempotenceKey  `json:"idempotenceKey"`
	MessageType    MessageType     `json:"messageType"`
	Body           json.RawMessage `json:"message"`
}

func (m *Message) DecodeNewTx() (*Transaction, error) {
	if m.MessageType != MessageTypeNewTx {
		return nil, fmt.Errorf("interbank: envelope messageType is %q, expected NEW_TX", m.MessageType)
	}
	var tx Transaction
	if err := json.Unmarshal(m.Body, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (m *Message) DecodeCommitTx() (*CommitTransaction, error) {
	if m.MessageType != MessageTypeCommitTx {
		return nil, fmt.Errorf("interbank: envelope messageType is %q, expected COMMIT_TX", m.MessageType)
	}
	var c CommitTransaction
	if err := json.Unmarshal(m.Body, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (m *Message) DecodeRollbackTx() (*RollbackTransaction, error) {
	if m.MessageType != MessageTypeRollbackTx {
		return nil, fmt.Errorf("interbank: envelope messageType is %q, expected ROLLBACK_TX", m.MessageType)
	}
	var r RollbackTransaction
	if err := json.Unmarshal(m.Body, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// NewMessage builds an envelope around any of the three message bodies.
// Returns an error if body is not a Transaction / CommitTransaction / RollbackTransaction.
func NewMessage(key IdempotenceKey, body any) (*Message, error) {
	var mt MessageType
	switch body.(type) {
	case Transaction, *Transaction:
		mt = MessageTypeNewTx
	case CommitTransaction, *CommitTransaction:
		mt = MessageTypeCommitTx
	case RollbackTransaction, *RollbackTransaction:
		mt = MessageTypeRollbackTx
	default:
		return nil, fmt.Errorf("interbank.NewMessage: unsupported body type %T", body)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &Message{
		IdempotenceKey: key,
		MessageType:    mt,
		Body:           raw,
	}, nil
}

// =====================
// Vote types (spec §2.12.1)
// =====================

type Vote string

const (
	VoteYes Vote = "YES"
	VoteNo  Vote = "NO"
)

// TransactionVote is the response body to a NEW_TX message. Reasons is
// populated only when Vote == VoteNo.
type TransactionVote struct {
	Vote    Vote           `json:"vote"`
	Reasons []NoVoteReason `json:"reasons,omitempty"`
}

type NoVoteReasonCode string

const (
	ReasonUnbalancedTx              NoVoteReasonCode = "UNBALANCED_TX"
	ReasonNoSuchAccount             NoVoteReasonCode = "NO_SUCH_ACCOUNT"
	ReasonNoSuchAsset               NoVoteReasonCode = "NO_SUCH_ASSET"
	ReasonUnacceptableAsset         NoVoteReasonCode = "UNACCEPTABLE_ASSET"
	ReasonInsufficientAsset         NoVoteReasonCode = "INSUFFICIENT_ASSET"
	ReasonOptionAmountIncorrect     NoVoteReasonCode = "OPTION_AMOUNT_INCORRECT"
	ReasonOptionUsedOrExpired       NoVoteReasonCode = "OPTION_USED_OR_EXPIRED"
	ReasonOptionNegotiationNotFound NoVoteReasonCode = "OPTION_NEGOTIATION_NOT_FOUND"
)

// NoVoteReason carries a reason code and (for reasons that reference a
// specific posting) the offending posting. UNBALANCED_TX has no posting;
// the others do per the spec, though we keep Posting optional to be lenient.
type NoVoteReason struct {
	Reason  NoVoteReasonCode `json:"reason"`
	Posting *Posting         `json:"posting,omitempty"`
}

// =====================
// OTC negotiation types (spec §3)
// =====================

type OtcOffer struct {
	Stock          StockDescription            `json:"stock"`
	SettlementDate ISO8601DateTimeWithTimeZone `json:"settlementDate"`
	PricePerUnit   MonetaryValue               `json:"pricePerUnit"`
	Premium        MonetaryValue               `json:"premium"`
	BuyerID        ForeignBankId               `json:"buyerId"`
	SellerID       ForeignBankId               `json:"sellerId"`
	Amount         float64                     `json:"amount"`
	LastModifiedBy ForeignBankId               `json:"lastModifiedBy"`
}

type OtcNegotiation struct {
	OtcOffer
	IsOngoing bool `json:"isOngoing"`
}

type PublicStockSeller struct {
	Seller ForeignBankId `json:"seller"`
	Amount float64       `json:"amount"`
}

type PublicStock struct {
	Stock   StockDescription    `json:"stock"`
	Sellers []PublicStockSeller `json:"sellers"`
}

type PublicStocksResponse = []PublicStock

// UserInformation is the response body of GET /user/{routingNumber}/{id}.
type UserInformation struct {
	BankDisplayName string `json:"bankDisplayName"`
	DisplayName     string `json:"displayName"`
}
