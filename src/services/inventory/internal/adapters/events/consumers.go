package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/messaging"
	"github.com/osmanozen/oo-commerce/services/inventory/internal/domain"
)

// InventoryEventConsumer handles incoming Kafka events from other services.
// Uses consumer groups for horizontal scaling across multiple Inventory instances.
type InventoryEventConsumer struct {
	stockRepo domain.StockItemRepository
	consumer  messaging.EventSubscriber
	producer  messaging.EventBus
	logger    *slog.Logger
}

// NewInventoryEventConsumer creates a new event consumer for the Inventory service.
func NewInventoryEventConsumer(
	stockRepo domain.StockItemRepository,
	consumer messaging.EventSubscriber,
	producer messaging.EventBus,
	logger *slog.Logger,
) *InventoryEventConsumer {
	return &InventoryEventConsumer{
		stockRepo: stockRepo,
		consumer:  consumer,
		producer:  producer,
		logger:    logger,
	}
}

// Start subscribes to relevant Kafka topics.
func (c *InventoryEventConsumer) Start(ctx context.Context) error {
	const groupID = "inventory-service"

	// Auto-create StockItem when a new product is created in Catalog.
	if err := c.consumer.Subscribe(ctx, "catalog.product.created", groupID, c.handleProductCreated); err != nil {
		return fmt.Errorf("subscribing to product created: %w", err)
	}

	// Saga commands from the Checkout Saga orchestrator.
	if err := c.consumer.Subscribe(ctx, "saga.reserve-stock", groupID, c.handleReserveStock); err != nil {
		return fmt.Errorf("subscribing to reserve stock: %w", err)
	}
	if err := c.consumer.Subscribe(ctx, "saga.release-stock", groupID, c.handleReleaseStock); err != nil {
		return fmt.Errorf("subscribing to release stock: %w", err)
	}

	// Commit stock when order is confirmed.
	if err := c.consumer.Subscribe(ctx, "ordering.order.confirmed", groupID, c.handleOrderConfirmed); err != nil {
		return fmt.Errorf("subscribing to order confirmed: %w", err)
	}

	c.logger.Info("inventory event consumers started", slog.String("group_id", groupID))
	return nil
}

// handleProductCreated auto-creates a StockItem with 0 quantity when a product is created.
func (c *InventoryEventConsumer) handleProductCreated(ctx context.Context, msg messaging.Message) error {
	var event struct {
		ProductID uuid.UUID `json:"productId"`
		SKU       string    `json:"sku"`
	}
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("deserializing product created event: %w", err)
	}

	c.logger.InfoContext(ctx, "auto-creating stock item for new product",
		slog.String("product_id", event.ProductID.String()),
		slog.String("sku", event.SKU),
	)

	// Check if stock item already exists (idempotency).
	existing, _ := c.stockRepo.GetByProductID(ctx, event.ProductID)
	if existing != nil {
		c.logger.DebugContext(ctx, "stock item already exists, skipping",
			slog.String("product_id", event.ProductID.String()),
		)
		return nil
	}

	stockItem := domain.NewStockItem(event.ProductID, event.SKU)
	return c.stockRepo.Create(ctx, stockItem)
}

// handleReserveStock processes a stock reservation command from the Checkout Saga.
func (c *InventoryEventConsumer) handleReserveStock(ctx context.Context, msg messaging.Message) error {
	var cmd struct {
		CorrelationID uuid.UUID `json:"correlationId"`
		OrderID       uuid.UUID `json:"orderId"`
		ProductID     uuid.UUID `json:"productId"`
		Quantity      int       `json:"quantity"`
	}
	if err := json.Unmarshal(msg.Value, &cmd); err != nil {
		return fmt.Errorf("deserializing reserve stock command: %w", err)
	}

	c.logger.InfoContext(ctx, "processing stock reservation",
		slog.String("correlation_id", cmd.CorrelationID.String()),
		slog.String("product_id", cmd.ProductID.String()),
		slog.Int("quantity", cmd.Quantity),
	)

	stockItem, err := c.stockRepo.GetByProductID(ctx, cmd.ProductID)
	if err != nil || stockItem == nil {
		// Publish failure event for saga.
		return c.publishReservationFailed(ctx, cmd.CorrelationID, cmd.ProductID, "stock item not found")
	}

	// Try to reserve stock — the aggregate enforces the invariant.
	reservation, err := stockItem.Reserve(cmd.OrderID, cmd.CorrelationID, cmd.Quantity)
	if err != nil {
		// Domain event already raised by the aggregate for saga.
		// Persist the state change and publish events.
		_ = c.stockRepo.Update(ctx, stockItem)
		return c.publishDomainEvents(ctx, stockItem)
	}

	// Persist reservation.
	if err := c.stockRepo.Update(ctx, stockItem); err != nil {
		return fmt.Errorf("saving reservation: %w", err)
	}

	// Publish success event.
	return c.publishStockReserved(ctx, cmd.CorrelationID, cmd.ProductID, reservation.ID.Value(), cmd.Quantity)
}

// handleReleaseStock processes a stock release command (saga compensation).
func (c *InventoryEventConsumer) handleReleaseStock(ctx context.Context, msg messaging.Message) error {
	var cmd struct {
		CorrelationID uuid.UUID `json:"correlationId"`
		ReservationID uuid.UUID `json:"reservationId"`
		ProductID     uuid.UUID `json:"productId"`
	}
	if err := json.Unmarshal(msg.Value, &cmd); err != nil {
		return fmt.Errorf("deserializing release stock command: %w", err)
	}

	c.logger.InfoContext(ctx, "releasing stock reservation (compensation)",
		slog.String("correlation_id", cmd.CorrelationID.String()),
		slog.String("reservation_id", cmd.ReservationID.String()),
	)

	stockItem, err := c.stockRepo.GetByProductID(ctx, cmd.ProductID)
	if err != nil || stockItem == nil {
		return fmt.Errorf("stock item not found for product %s", cmd.ProductID)
	}

	resID, err := domain.ReservationIDFromUUID(cmd.ReservationID)
	if err != nil {
		return fmt.Errorf("invalid reservation id: %w", err)
	}

	if err := stockItem.ReleaseReservation(resID); err != nil {
		return fmt.Errorf("releasing reservation: %w", err)
	}

	return c.stockRepo.Update(ctx, stockItem)
}

// handleOrderConfirmed commits stock reservations when order is finalized.
func (c *InventoryEventConsumer) handleOrderConfirmed(ctx context.Context, msg messaging.Message) error {
	var event struct {
		OrderID uuid.UUID `json:"orderId"`
	}
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("deserializing order confirmed event: %w", err)
	}

	c.logger.InfoContext(ctx, "committing stock for confirmed order",
		slog.String("order_id", event.OrderID.String()),
	)

	// Find and commit all reservations for this order.
	reservations, err := c.stockRepo.GetReservationsByOrderID(ctx, event.OrderID)
	if err != nil {
		return fmt.Errorf("finding reservations for order: %w", err)
	}

	for _, res := range reservations {
		stockItem, err := c.stockRepo.GetByID(ctx, res.StockItemID)
		if err != nil {
			c.logger.ErrorContext(ctx, "failed to load stock item for commit",
				slog.String("stock_item_id", res.StockItemID.String()),
			)
			continue
		}

		if err := stockItem.CommitReservation(res.ID); err != nil {
			c.logger.ErrorContext(ctx, "failed to commit reservation",
				slog.String("reservation_id", res.ID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		if err := c.stockRepo.Update(ctx, stockItem); err != nil {
			c.logger.ErrorContext(ctx, "failed to save committed stock",
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// ─── Event Publishing Helpers ────────────────────────────────────────────────

func (c *InventoryEventConsumer) publishStockReserved(ctx context.Context, correlationID, productID, reservationID uuid.UUID, quantity int) error {
	event := map[string]interface{}{
		"correlationId": correlationID,
		"productId":     productID,
		"reservationId": reservationID,
		"quantity":      quantity,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.producer.Publish(ctx, "inventory.stock.reserved", correlationID.String(), payload)
}

func (c *InventoryEventConsumer) publishReservationFailed(ctx context.Context, correlationID, productID uuid.UUID, reason string) error {
	event := map[string]interface{}{
		"correlationId": correlationID,
		"productId":     productID,
		"reason":        reason,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.producer.Publish(ctx, "inventory.stock.reservation-failed", correlationID.String(), payload)
}

func (c *InventoryEventConsumer) publishDomainEvents(ctx context.Context, stockItem *domain.StockItem) error {
	for _, event := range stockItem.GetDomainEvents() {
		payload, err := json.Marshal(event)
		if err != nil {
			continue
		}
		_ = c.producer.Publish(ctx, event.EventType(), stockItem.ProductID.String(), payload)
	}
	stockItem.ClearDomainEvents()
	return nil
}
