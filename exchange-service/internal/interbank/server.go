package interbank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/repository"
)

// TxProcessor is the contract between the inter-bank wire layer and
// whatever local subsystem actually executes 2PC. Implementations are
// responsible for ledger effects (debits, credits, holds), validation
// (balance checks, asset existence), and idempotence at the BUSINESS
// layer. The /interbank handler already deduplicates by idempotence
// key, so implementations may assume each method is invoked at most
// once per (NEW_TX, transactionID) tuple.
type TxProcessor interface {
	OnNewTx(ctx context.Context, partner *PartnerBank, tx *Transaction) (*TransactionVote, error)
	OnCommitTx(ctx context.Context, partner *PartnerBank, transactionID ForeignBankId) error
	OnRollbackTx(ctx context.Context, partner *PartnerBank, transactionID ForeignBankId) error
}

// NoopProcessor is a placeholder TxProcessor that votes NO on every
// NEW_TX and accepts COMMIT/ROLLBACK as no-ops. It exists so we can
// wire the /interbank route end-to-end before the real ledger
// integration lands. Switch this out via Server.SetProcessor when the
// executor is ready.
type NoopProcessor struct{}

func (NoopProcessor) OnNewTx(_ context.Context, _ *PartnerBank, _ *Transaction) (*TransactionVote, error) {
	return &TransactionVote{
		Vote: VoteNo,
		Reasons: []NoVoteReason{
			{Reason: ReasonUnacceptableAsset},
		},
	}, nil
}

func (NoopProcessor) OnCommitTx(_ context.Context, _ *PartnerBank, _ ForeignBankId) error   { return nil }
func (NoopProcessor) OnRollbackTx(_ context.Context, _ *PartnerBank, _ ForeignBankId) error { return nil }

// Server hosts the inbound /interbank endpoint. It owns auth, parse,
// dedup, and dispatch. The actual business logic lives behind a
// TxProcessor — see the type above.
type Server struct {
	registry  *Registry
	inbound   *repository.InterbankInboundRepository
	processor TxProcessor
}

// NewServer wires up the inbound message endpoint. processor may be a
// NoopProcessor while the real executor is being built.
func NewServer(registry *Registry, inbound *repository.InterbankInboundRepository, processor TxProcessor) *Server {
	return &Server{
		registry:  registry,
		inbound:   inbound,
		processor: processor,
	}
}

// SetProcessor swaps the active TxProcessor at runtime. Useful for
// tests and for late-wiring the real executor after construction.
func (s *Server) SetProcessor(p TxProcessor) { s.processor = p }

// ServeHTTP implements POST /interbank.
//
// The flow is:
//  1. Authenticate by X-Api-Key (resolves the inbound partner identity).
//  2. Read + parse the envelope.
//  3. Cross-check envelope.idempotenceKey.routingNumber == partner.Code.
//     This prevents partner A from sending messages tagged as partner B.
//  4. TryRecordOrFetch on the inbound idempotence log; on cache hit,
//     replay the cached HTTP status + body verbatim.
//  5. On cache miss, dispatch by messageType to the TxProcessor, then
//     finalise the log row with the rendered response.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := r.Header.Get(HeaderAPIKey)
	partner := s.registry.LookupByInboundKey(apiKey)
	if partner == nil {
		writeProblemJSON(w, http.StatusUnauthorized, "unrecognised X-Api-Key")
		return
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeProblemJSON(w, http.StatusBadRequest, fmt.Sprintf("reading body: %v", err))
		return
	}

	var msg Message
	if err := json.Unmarshal(rawBody, &msg); err != nil {
		writeProblemJSON(w, http.StatusBadRequest, fmt.Sprintf("malformed envelope: %v", err))
		return
	}

	if msg.IdempotenceKey.RoutingNumber != partner.Code {
		writeProblemJSON(w, http.StatusForbidden,
			fmt.Sprintf("idempotenceKey.routingNumber %d does not match the partner this X-Api-Key identifies (%d)",
				msg.IdempotenceKey.RoutingNumber, partner.Code))
		return
	}
	if msg.IdempotenceKey.LocallyGeneratedKey == "" {
		writeProblemJSON(w, http.StatusBadRequest, "idempotenceKey.locallyGeneratedKey is required")
		return
	}
	if len(msg.IdempotenceKey.LocallyGeneratedKey) > 64 {
		writeProblemJSON(w, http.StatusBadRequest, "idempotenceKey.locallyGeneratedKey exceeds 64 bytes")
		return
	}

	isNew, existing, err := s.inbound.TryRecordOrFetch(
		int(partner.Code),
		msg.IdempotenceKey.LocallyGeneratedKey,
		string(msg.MessageType),
		string(rawBody),
	)
	if err != nil {
		slog.Error("interbank: idempotence record failed", "err", err, "partner", partner.Code)
		writeProblemJSON(w, http.StatusInternalServerError, "idempotence store failed")
		return
	}

	if !isNew {
		s.replayCached(w, existing)
		return
	}

	status, body, finalState, processErr := s.dispatch(r.Context(), partner, &msg)
	if finalizeErr := s.inbound.FinalizeWithResponse(
		int(partner.Code),
		msg.IdempotenceKey.LocallyGeneratedKey,
		status,
		string(body),
		finalState,
		errString(processErr),
	); finalizeErr != nil {
		slog.Error("interbank: finalize idempotence row failed",
			"err", finalizeErr, "partner", partner.Code, "message_type", msg.MessageType)
	}

	writeRaw(w, status, body)
}

// replayCached is the second-and-subsequent-time path: we already
// processed this idempotence key, so we re-emit the exact same response
// bytes and status. Per the protocol, this MUST be byte-identical.
func (s *Server) replayCached(w http.ResponseWriter, existing *models.InterbankInboundMessage) {
	switch existing.Status {
	case models.InterbankInboundStatusReceived:
		// Another worker is still processing. Tell the partner to retry —
		// they'll re-POST the same idempotence key and eventually catch
		// the finalised row.
		w.WriteHeader(http.StatusAccepted)
		return
	case models.InterbankInboundStatusProcessed, models.InterbankInboundStatusFailed:
		writeRaw(w, existing.HTTPStatus, []byte(existing.ResponseBody))
		return
	default:
		slog.Error("interbank: cached row has unknown status",
			"status", existing.Status, "id", existing.ID)
		writeProblemJSON(w, http.StatusInternalServerError, "cached row in unknown state")
		return
	}
}

// dispatch routes by messageType. Returns the HTTP status to send back,
// the response body bytes, the idempotence row's final status string,
// and any error encountered (captured into the row).
func (s *Server) dispatch(ctx context.Context, partner *PartnerBank, msg *Message) (int, []byte, string, error) {
	switch msg.MessageType {
	case MessageTypeNewTx:
		tx, err := msg.DecodeNewTx()
		if err != nil {
			body := problemJSON(fmt.Sprintf("malformed NEW_TX body: %v", err))
			return http.StatusBadRequest, body, models.InterbankInboundStatusFailed, err
		}
		vote, err := s.processor.OnNewTx(ctx, partner, tx)
		if err != nil {
			slog.Error("interbank: NEW_TX processor error",
				"err", err, "partner", partner.Code, "tx", tx.TransactionID.ID)
			body := problemJSON("internal error processing NEW_TX")
			return http.StatusInternalServerError, body, models.InterbankInboundStatusFailed, err
		}
		out, err := json.Marshal(vote)
		if err != nil {
			body := problemJSON(fmt.Sprintf("encoding vote: %v", err))
			return http.StatusInternalServerError, body, models.InterbankInboundStatusFailed, err
		}
		return http.StatusOK, out, models.InterbankInboundStatusProcessed, nil

	case MessageTypeCommitTx:
		commit, err := msg.DecodeCommitTx()
		if err != nil {
			body := problemJSON(fmt.Sprintf("malformed COMMIT_TX body: %v", err))
			return http.StatusBadRequest, body, models.InterbankInboundStatusFailed, err
		}
		if err := s.processor.OnCommitTx(ctx, partner, commit.TransactionID); err != nil {
			slog.Error("interbank: COMMIT_TX processor error",
				"err", err, "partner", partner.Code, "tx", commit.TransactionID.ID)
			body := problemJSON("internal error processing COMMIT_TX")
			return http.StatusInternalServerError, body, models.InterbankInboundStatusFailed, err
		}
		return http.StatusNoContent, nil, models.InterbankInboundStatusProcessed, nil

	case MessageTypeRollbackTx:
		rollback, err := msg.DecodeRollbackTx()
		if err != nil {
			body := problemJSON(fmt.Sprintf("malformed ROLLBACK_TX body: %v", err))
			return http.StatusBadRequest, body, models.InterbankInboundStatusFailed, err
		}
		if err := s.processor.OnRollbackTx(ctx, partner, rollback.TransactionID); err != nil {
			slog.Error("interbank: ROLLBACK_TX processor error",
				"err", err, "partner", partner.Code, "tx", rollback.TransactionID.ID)
			body := problemJSON("internal error processing ROLLBACK_TX")
			return http.StatusInternalServerError, body, models.InterbankInboundStatusFailed, err
		}
		return http.StatusNoContent, nil, models.InterbankInboundStatusProcessed, nil

	default:
		body := problemJSON(fmt.Sprintf("unknown messageType %q", msg.MessageType))
		return http.StatusBadRequest, body, models.InterbankInboundStatusFailed, errors.New("unknown messageType")
	}
}

func writeRaw(w http.ResponseWriter, status int, body []byte) {
	if len(body) > 0 {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if len(body) > 0 {
		_, _ = w.Write(body)
	}
}

func writeProblemJSON(w http.ResponseWriter, status int, message string) {
	writeRaw(w, status, problemJSON(message))
}

func problemJSON(message string) []byte {
	b, _ := json.Marshal(map[string]string{"message": message})
	return b
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
