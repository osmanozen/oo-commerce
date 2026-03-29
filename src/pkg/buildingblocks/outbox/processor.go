package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/messaging"
)

// Processor polls the outbox table for unsent messages and publishes them
// to Apache Kafka. This runs as a background goroutine per service.
//
// The Outbox Pattern guarantees that domain events are reliably delivered
// even if Kafka is temporarily unavailable. Events are stored in the same
// DB transaction as the business data, then forwarded asynchronously.
type Processor struct {
	store     MessageStore
	eventBus  messaging.EventBus
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
}

// MessageStore abstracts outbox table access.
type MessageStore interface {
	// FetchPending returns unsent outbox messages, oldest first.
	FetchPending(ctx context.Context, limit int) ([]OutboxMessage, error)
	// MarkSent marks a message as successfully published to Kafka.
	MarkSent(ctx context.Context, id int64) error
	// IncrementRetry increments the retry counter for failed messages.
	IncrementRetry(ctx context.Context, id int64) error
}

// NewProcessor creates a new outbox processor.
func NewProcessor(store MessageStore, bus messaging.EventBus, logger *slog.Logger) *Processor {
	return &Processor{
		store:     store,
		eventBus:  bus,
		logger:    logger,
		interval:  5 * time.Second,
		batchSize: 100,
	}
}

// Start runs the outbox processor loop. It blocks until context is cancelled.
func (p *Processor) Start(ctx context.Context) {
	p.logger.Info("outbox processor started",
		slog.Duration("interval", p.interval),
		slog.Int("batch_size", p.batchSize),
	)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox processor stopped")
			return
		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil {
				p.logger.ErrorContext(ctx, "outbox batch processing failed",
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

func (p *Processor) processBatch(ctx context.Context) error {
	messages, err := p.store.FetchPending(ctx, p.batchSize)
	if err != nil {
		return err
	}

	if len(messages) == 0 {
		return nil
	}

	p.logger.DebugContext(ctx, "processing outbox batch", slog.Int("count", len(messages)))

	for _, msg := range messages {
		// Use MessageType as the Kafka topic (e.g., "catalog.product.created")
		// and MessageID as the partition key for ordered delivery per event source.
		partitionKey := msg.MessageID.String()
		if msg.CorrelationID != nil {
			// Saga-correlated events use correlation ID as partition key
			// so all saga events for the same order land on the same partition.
			partitionKey = msg.CorrelationID.String()
		}

		if err := p.eventBus.Publish(ctx, msg.MessageType, partitionKey, []byte(msg.Payload)); err != nil {
			p.logger.WarnContext(ctx, "failed to produce to kafka",
				slog.String("topic", msg.MessageType),
				slog.String("message_id", msg.MessageID.String()),
				slog.Int("retry_count", msg.RetryCount),
				slog.String("error", err.Error()),
			)
			_ = p.store.IncrementRetry(ctx, msg.ID)
			continue
		}

		if err := p.store.MarkSent(ctx, msg.ID); err != nil {
			p.logger.ErrorContext(ctx, "failed to mark outbox message as sent",
				slog.Int64("id", msg.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}
