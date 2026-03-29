package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/domain"
)

// OutboxMessage represents a domain event stored for reliable delivery.
// Events are written to the outbox table within the same DB transaction
// as the business data — guaranteeing atomicity (no dual-write problem).
type OutboxMessage struct {
	ID            int64      `db:"id"`
	MessageID     uuid.UUID  `db:"message_id"`
	MessageType   string     `db:"message_type"`
	Payload       string     `db:"payload"`
	CorrelationID *uuid.UUID `db:"correlation_id"`
	CreatedAt     time.Time  `db:"created_at"`
	SentAt        *time.Time `db:"sent_at"`
	RetryCount    int        `db:"retry_count"`
}

// Publisher writes domain events to the outbox table within a transaction.
type Publisher interface {
	Publish(ctx context.Context, tx TxHandle, events ...domain.DomainEvent) error
}

// TxHandle is an abstraction over database transaction handles.
type TxHandle interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (Result, error)
}

// Result abstracts sql.Result.
type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

// outboxPublisher implements Publisher.
type outboxPublisher struct {
	logger *slog.Logger
}

// NewPublisher creates a new outbox publisher.
func NewPublisher(logger *slog.Logger) Publisher {
	return &outboxPublisher{logger: logger}
}

const insertOutboxSQL = `
INSERT INTO outbox_messages (message_id, message_type, payload, correlation_id, created_at, retry_count)
VALUES ($1, $2, $3, $4, $5, 0)
`

// Publish serializes domain events and inserts them into the outbox table
// within the provided transaction — ensuring atomicity with business data.
func (p *outboxPublisher) Publish(ctx context.Context, tx TxHandle, events ...domain.DomainEvent) error {
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("serializing event %s: %w", event.EventType(), err)
		}

		_, err = tx.ExecContext(ctx, insertOutboxSQL,
			event.EventID(),
			event.EventType(),
			string(payload),
			nil, // correlation_id — set by saga when needed
			time.Now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("inserting outbox message for %s: %w", event.EventType(), err)
		}

		p.logger.DebugContext(ctx, "outbox message stored",
			slog.String("event_type", event.EventType()),
			slog.String("event_id", event.EventID().String()),
		)
	}
	return nil
}
