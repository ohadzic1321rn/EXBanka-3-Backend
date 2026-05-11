package models

import "time"

const (
	InterbankInboundStatusReceived  = "received"
	InterbankInboundStatusProcessed = "processed"
	InterbankInboundStatusFailed    = "failed"
)

const (
	InterbankNegotiationRoleBuyer  = "buyer"
	InterbankNegotiationRoleSeller = "seller"
)

// InterbankInboundMessage is the audit + idempotence log for every
// /interbank request we accept from a partner bank. The composite
// (routing_number, locally_generated_key) is the protocol-defined
// idempotence key; the receiver MUST return an identical response on
// replay, so we persist the rendered response body alongside.
type InterbankInboundMessage struct {
	ID                  uint   `gorm:"primaryKey"`
	RoutingNumber       int    `gorm:"column:routing_number;not null;uniqueIndex:idx_interbank_idem_key,priority:1"`
	LocallyGeneratedKey string `gorm:"column:locally_generated_key;type:varchar(64);not null;uniqueIndex:idx_interbank_idem_key,priority:2"`

	MessageType string `gorm:"column:message_type;not null;index"`
	RequestBody string `gorm:"column:request_body;type:text;not null"`

	Status       string `gorm:"not null;default:'received';index"`
	HTTPStatus   int    `gorm:"column:http_status;not null;default:0"`
	ResponseBody string `gorm:"column:response_body;type:text"`
	Error        string `gorm:"type:text"`

	CreatedAt   time.Time  `gorm:"not null"`
	ProcessedAt *time.Time `gorm:"column:processed_at"`
}

func (InterbankInboundMessage) TableName() string { return "interbank_inbound_messages" }

// InterbankOtcNegotiation is our local copy of a cross-bank OTC option
// negotiation. The negotiation's globally-unique key is
// (NegotiationRoutingNumber, NegotiationID), which is the seller bank's
// routing number + the seller bank's locally-generated id (per spec
// §3.2 — the seller's bank mints the id when POST /negotiations
// arrives). Both banks store an identical copy and update it on each
// counter-offer.
type InterbankOtcNegotiation struct {
	ID uint `gorm:"primaryKey"`

	NegotiationRoutingNumber int    `gorm:"column:negotiation_routing_number;not null;uniqueIndex:idx_interbank_negotiation_key,priority:1"`
	NegotiationID            string `gorm:"column:negotiation_id;type:varchar(64);not null;uniqueIndex:idx_interbank_negotiation_key,priority:2"`

	// LocalRole is which side of the negotiation we play —
	// "buyer" if our client initiated, "seller" if a partner's
	// client posted to us.
	LocalRole string `gorm:"column:local_role;not null;index"`

	// CounterpartyRoutingNumber is the OTHER bank in this negotiation.
	// We always have exactly one counterparty (the buyer's bank or
	// the seller's bank — whichever isn't us).
	CounterpartyRoutingNumber int `gorm:"column:counterparty_routing_number;not null;index"`

	// Buyer / seller identities. The local-side identity is encoded
	// via interbank.EncodeLocalParticipantID; the remote-side is the
	// partner's opaque string.
	BuyerRoutingNumber  int    `gorm:"column:buyer_routing_number;not null"`
	BuyerID             string `gorm:"column:buyer_id;type:varchar(64);not null"`
	SellerRoutingNumber int    `gorm:"column:seller_routing_number;not null"`
	SellerID            string `gorm:"column:seller_id;type:varchar(64);not null"`

	StockTicker string `gorm:"column:stock_ticker;not null;index"`
	Amount      float64

	PricePerUnitCurrency string  `gorm:"column:price_per_unit_currency;not null"`
	PricePerUnitAmount   float64 `gorm:"column:price_per_unit_amount;not null"`
	PremiumCurrency      string  `gorm:"column:premium_currency;not null"`
	PremiumAmount        float64 `gorm:"column:premium_amount;not null"`

	// SettlementDate stores the ISO8601 timestamp as a string at the
	// DB boundary so we don't lose the partner's original timezone.
	SettlementDate string `gorm:"column:settlement_date;not null"`

	LastModifiedByRoutingNumber int    `gorm:"column:last_modified_by_routing_number;not null"`
	LastModifiedByID            string `gorm:"column:last_modified_by_id;type:varchar(64);not null"`

	IsOngoing bool `gorm:"column:is_ongoing;not null;default:true;index"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (InterbankOtcNegotiation) TableName() string { return "interbank_otc_negotiations" }
