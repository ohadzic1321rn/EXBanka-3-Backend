package repository

import (
	"errors"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
	"gorm.io/gorm"
)

// InterbankInboundRepository is the persistence shim for the inbound
// idempotence + audit log. The protocol requires us to return an
// identical response on every replay of the same idempotence key, so
// every accepted /interbank request gets one row here.
type InterbankInboundRepository struct {
	db *gorm.DB
}

func NewInterbankInboundRepository(db *gorm.DB) *InterbankInboundRepository {
	return &InterbankInboundRepository{db: db}
}

// TryRecordOrFetch is the single hot-path call for inbound dedup.
//
// On first sight of an idempotence key it INSERTs a new row in status
// "received" and returns (isNew=true, existing=nil, nil). On a replay
// it returns (isNew=false, existing=<cached row>, nil) so the caller
// can serve the cached response without re-running the handler.
//
// The INSERT relies on the (routing_number, locally_generated_key)
// unique index to race-safely arbitrate between concurrent first-sight
// attempts: the loser of the race sees a unique-violation, then re-
// reads and returns the winner's row as if it had been a cache hit.
func (r *InterbankInboundRepository) TryRecordOrFetch(routingNumber int, locallyGeneratedKey, messageType, requestBody string) (isNew bool, existing *models.InterbankInboundMessage, err error) {
	row := &models.InterbankInboundMessage{
		RoutingNumber:       routingNumber,
		LocallyGeneratedKey: locallyGeneratedKey,
		MessageType:         messageType,
		RequestBody:         requestBody,
		Status:              models.InterbankInboundStatusReceived,
		CreatedAt:           time.Now().UTC(),
	}
	createErr := r.db.Create(row).Error
	if createErr == nil {
		return true, row, nil
	}

	var found models.InterbankInboundMessage
	lookupErr := r.db.
		Where("routing_number = ? AND locally_generated_key = ?", routingNumber, locallyGeneratedKey).
		First(&found).Error
	if lookupErr == nil {
		return false, &found, nil
	}
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return false, nil, createErr
	}
	return false, nil, lookupErr
}

// FinalizeWithResponse stamps the cached response so future replays
// of the same idempotence key get the same bytes back. Pass status =
// InterbankInboundStatusProcessed for a normal completion or
// InterbankInboundStatusFailed when we returned a 4xx/5xx that we want
// to remember verbatim.
func (r *InterbankInboundRepository) FinalizeWithResponse(routingNumber int, locallyGeneratedKey string, httpStatus int, responseBody, status, errMsg string) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":        status,
		"http_status":   httpStatus,
		"response_body": responseBody,
		"processed_at":  now,
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}
	return r.db.Model(&models.InterbankInboundMessage{}).
		Where("routing_number = ? AND locally_generated_key = ?", routingNumber, locallyGeneratedKey).
		Updates(updates).Error
}

// Get looks up a single inbound message by idempotence key. Returns
// (nil, nil) when no row exists.
func (r *InterbankInboundRepository) Get(routingNumber int, locallyGeneratedKey string) (*models.InterbankInboundMessage, error) {
	var row models.InterbankInboundMessage
	err := r.db.
		Where("routing_number = ? AND locally_generated_key = ?", routingNumber, locallyGeneratedKey).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}
