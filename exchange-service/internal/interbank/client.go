package interbank

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// HeaderAPIKey is the header name carrying the per-partner X-Api-Key
// token (the key the PARTNER issued to US). Per spec §1.
const HeaderAPIKey = "X-Api-Key"

// ErrNoSuchPartner is returned when an outbound call targets a routing
// number we don't have registered.
var ErrNoSuchPartner = errors.New("interbank: no partner registered for routing number")

// ErrAcceptedTimeout is returned when the partner kept replying 202
// Accepted for longer than the client's polling budget. The caller's
// 2PC state machine should retry the same idempotence key later — the
// partner will eventually return the same final response.
var ErrAcceptedTimeout = errors.New("interbank: partner kept replying 202 Accepted past the polling deadline")

// RemoteError is the error type returned when a partner replies with a
// 4xx/5xx status. The body is captured verbatim so callers can decide
// whether to surface it.
type RemoteError struct {
	StatusCode int
	Status     string
	Body       []byte
}

func (e *RemoteError) Error() string {
	bodyPreview := string(e.Body)
	if len(bodyPreview) > 256 {
		bodyPreview = bodyPreview[:256] + "..."
	}
	return fmt.Sprintf("interbank: partner returned HTTP %d %s: %s", e.StatusCode, e.Status, bodyPreview)
}

// ClientOption customises Client behaviour at construction time.
type ClientOption func(*Client)

// WithHTTPClient lets tests inject a stub *http.Client.
func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *Client) { c.http = h }
}

// WithRetryPolicy sets the polling cadence used when a partner replies
// 202 Accepted. The slice's length is the maximum number of polls; each
// element is the delay BEFORE that poll. Default: 7 polls with backoff
// from 250ms up to 8s (≈18s total).
func WithRetryPolicy(delays []time.Duration) ClientOption {
	return func(c *Client) { c.retryDelays = delays }
}

// WithSleepFunc overrides time.Sleep — wired by tests so retry loops
// can be exercised without real wall clock waits.
func WithSleepFunc(f func(time.Duration)) ClientOption {
	return func(c *Client) { c.sleep = f }
}

// Client is the outbound transport for /interbank messages. One Client
// can talk to any partner in its Registry; routing is by RoutingNumber.
type Client struct {
	registry    *Registry
	http        *http.Client
	retryDelays []time.Duration
	sleep       func(time.Duration)
}

// NewClient builds an outbound client around a Registry.
func NewClient(registry *Registry, opts ...ClientOption) *Client {
	c := &Client{
		registry: registry,
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
		retryDelays: []time.Duration{
			250 * time.Millisecond,
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			4 * time.Second,
			6 * time.Second,
		},
		sleep: time.Sleep,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewIdempotenceKey returns a fresh idempotence key tagged with our own
// routing number. The locally-generated half is a UUID v4 hex string
// (32 chars — well under the protocol's 64-byte cap).
func (c *Client) NewIdempotenceKey() IdempotenceKey {
	return IdempotenceKey{
		RoutingNumber:       c.registry.OwnRoutingNumber(),
		LocallyGeneratedKey: uuid.NewString(),
	}
}

// SendNewTx POSTs a NEW_TX message to the partner that owns
// partnerCode and returns the partner's TransactionVote. On a 202
// Accepted, the same idempotence key is re-POSTed per retryDelays
// until the partner returns a definitive response.
func (c *Client) SendNewTx(ctx context.Context, partnerCode RoutingNumber, key IdempotenceKey, tx *Transaction) (*TransactionVote, error) {
	msg, err := NewMessage(key, tx)
	if err != nil {
		return nil, err
	}
	respBody, err := c.sendEnvelope(ctx, partnerCode, msg)
	if err != nil {
		return nil, err
	}
	if len(respBody) == 0 {
		return nil, fmt.Errorf("interbank: partner returned empty body for NEW_TX (expected TransactionVote)")
	}
	var vote TransactionVote
	if err := json.Unmarshal(respBody, &vote); err != nil {
		return nil, fmt.Errorf("interbank: decoding NEW_TX response: %w", err)
	}
	return &vote, nil
}

// SendCommitTx POSTs a COMMIT_TX message. The protocol body is empty on
// success (204 No Content), so the return value is just error.
func (c *Client) SendCommitTx(ctx context.Context, partnerCode RoutingNumber, key IdempotenceKey, transactionID ForeignBankId) error {
	msg, err := NewMessage(key, CommitTransaction{TransactionID: transactionID})
	if err != nil {
		return err
	}
	_, err = c.sendEnvelope(ctx, partnerCode, msg)
	return err
}

// SendRollbackTx POSTs a ROLLBACK_TX message. Same shape as commit.
func (c *Client) SendRollbackTx(ctx context.Context, partnerCode RoutingNumber, key IdempotenceKey, transactionID ForeignBankId) error {
	msg, err := NewMessage(key, RollbackTransaction{TransactionID: transactionID})
	if err != nil {
		return err
	}
	_, err = c.sendEnvelope(ctx, partnerCode, msg)
	return err
}

// sendEnvelope is the workhorse: serialises the envelope, POSTs to
// {partner.BaseURL}/interbank with the partner's OutboundKey, and
// retries the SAME request (same idempotence key, same body) while the
// partner returns 202 Accepted. The protocol guarantees the partner
// will replay the cached final response, so we just keep asking.
func (c *Client) sendEnvelope(ctx context.Context, partnerCode RoutingNumber, msg *Message) ([]byte, error) {
	partner := c.registry.Lookup(partnerCode)
	if partner == nil {
		return nil, fmt.Errorf("%w: %d", ErrNoSuchPartner, partnerCode)
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("interbank: marshalling envelope: %w", err)
	}
	endpoint := partner.BaseURL + "/interbank"

	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("interbank: building request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set(HeaderAPIKey, partner.OutboundKey)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("interbank: POST %s: %w", endpoint, err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("interbank: reading response body from %s: %w", endpoint, readErr)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return respBody, nil
		case http.StatusNoContent:
			return nil, nil
		case http.StatusAccepted:
			if attempt >= len(c.retryDelays) {
				slog.Warn("interbank: partner kept replying 202 Accepted past polling budget",
					"partner", partnerCode, "attempts", attempt, "message_type", msg.MessageType)
				return nil, ErrAcceptedTimeout
			}
			slog.Debug("interbank: 202 Accepted, will re-POST same idempotence key",
				"partner", partnerCode, "attempt", attempt+1, "delay", c.retryDelays[attempt])
			c.sleep(c.retryDelays[attempt])
			continue
		default:
			return nil, &RemoteError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       respBody,
			}
		}
	}
}
