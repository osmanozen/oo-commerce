package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/messaging"
)

// SagaEventConsumer subscribes to Kafka topics and routes events to the saga orchestrator.
// Each consumer uses a dedicated consumer group for the ordering service.
type SagaEventConsumer struct {
	saga     *CheckoutSaga
	consumer messaging.EventSubscriber
	logger   *slog.Logger
}

// NewSagaEventConsumer creates a new saga event consumer.
func NewSagaEventConsumer(saga *CheckoutSaga, consumer messaging.EventSubscriber, logger *slog.Logger) *SagaEventConsumer {
	return &SagaEventConsumer{
		saga:     saga,
		consumer: consumer,
		logger:   logger,
	}
}

// Start subscribes to all saga-related Kafka topics.
func (c *SagaEventConsumer) Start(ctx context.Context) error {
	const groupID = "ordering-saga-consumer"

	// Stock reservation responses
	if err := c.consumer.Subscribe(ctx, "inventory.stock.reserved", groupID, c.handleStockReserved); err != nil {
		return fmt.Errorf("subscribing to stock reserved: %w", err)
	}
	if err := c.consumer.Subscribe(ctx, "inventory.stock.reservation-failed", groupID, c.handleStockReservationFailed); err != nil {
		return fmt.Errorf("subscribing to stock reservation failed: %w", err)
	}

	// Payment responses
	if err := c.consumer.Subscribe(ctx, "payment.processed", groupID, c.handlePaymentProcessed); err != nil {
		return fmt.Errorf("subscribing to payment processed: %w", err)
	}
	if err := c.consumer.Subscribe(ctx, "payment.failed", groupID, c.handlePaymentFailed); err != nil {
		return fmt.Errorf("subscribing to payment failed: %w", err)
	}

	c.logger.Info("saga event consumers started", slog.String("group_id", groupID))
	return nil
}

func (c *SagaEventConsumer) handleStockReserved(ctx context.Context, msg messaging.Message) error {
	var event struct {
		CorrelationID uuid.UUID `json:"correlationId"`
		ProductID     uuid.UUID `json:"productId"`
		ReservationID uuid.UUID `json:"reservationId"`
		Quantity      int       `json:"quantity"`
	}
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("deserializing stock reserved event: %w", err)
	}

	c.logger.InfoContext(ctx, "stock reserved event received",
		slog.String("correlation_id", event.CorrelationID.String()),
		slog.String("product_id", event.ProductID.String()),
	)

	return c.saga.HandleStockReserved(ctx, event.CorrelationID, event.ProductID, event.ReservationID)
}

func (c *SagaEventConsumer) handleStockReservationFailed(ctx context.Context, msg messaging.Message) error {
	var event struct {
		CorrelationID uuid.UUID `json:"correlationId"`
		ProductID     uuid.UUID `json:"productId"`
		Reason        string    `json:"reason"`
	}
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("deserializing stock reservation failed event: %w", err)
	}

	c.logger.WarnContext(ctx, "stock reservation failed event received",
		slog.String("correlation_id", event.CorrelationID.String()),
		slog.String("reason", event.Reason),
	)

	return c.saga.HandleStockReservationFailed(ctx, event.CorrelationID, event.Reason)
}

func (c *SagaEventConsumer) handlePaymentProcessed(ctx context.Context, msg messaging.Message) error {
	var event struct {
		CorrelationID uuid.UUID `json:"correlationId"`
		PaymentID     uuid.UUID `json:"paymentId"`
	}
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("deserializing payment processed event: %w", err)
	}

	c.logger.InfoContext(ctx, "payment processed event received",
		slog.String("correlation_id", event.CorrelationID.String()),
		slog.String("payment_id", event.PaymentID.String()),
	)

	return c.saga.HandlePaymentProcessed(ctx, event.CorrelationID, event.PaymentID)
}

func (c *SagaEventConsumer) handlePaymentFailed(ctx context.Context, msg messaging.Message) error {
	var event struct {
		CorrelationID uuid.UUID `json:"correlationId"`
		Reason        string    `json:"reason"`
	}
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("deserializing payment failed event: %w", err)
	}

	c.logger.WarnContext(ctx, "payment failed event received",
		slog.String("correlation_id", event.CorrelationID.String()),
		slog.String("reason", event.Reason),
	)

	return c.saga.HandlePaymentFailed(ctx, event.CorrelationID, event.Reason)
}
