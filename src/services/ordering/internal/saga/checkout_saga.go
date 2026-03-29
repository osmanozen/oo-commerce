package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/looplab/fsm"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/messaging"
	"github.com/shopspring/decimal"
)

// ─── Saga States ─────────────────────────────────────────────────────────────

const (
	StateInitialized            = "Initialized"
	StateReservingStock         = "ReservingStock"
	StateStockReserved          = "StockReserved"
	StateStockReservationFailed = "StockReservationFailed"
	StateProcessingPayment      = "ProcessingPayment"
	StatePaymentProcessed       = "PaymentProcessed"
	StatePaymentFailed          = "PaymentFailed"
	StateConfirming             = "Confirming"
	StateCompleted              = "Completed"
	StateFailed                 = "Failed"
	StateCompensating           = "Compensating"
	StateCancelled              = "Cancelled"
)

// ─── Saga Data ───────────────────────────────────────────────────────────────

// CheckoutSagaData holds the state of a checkout saga instance.
type CheckoutSagaData struct {
	CorrelationID uuid.UUID               `json:"correlationId"`
	OrderID       uuid.UUID               `json:"orderId"`
	BuyerID       string                  `json:"buyerId"`
	Items         []CheckoutItemData      `json:"items"`
	TotalAmount   decimal.Decimal         `json:"totalAmount"`
	Currency      string                  `json:"currency"`
	CurrentState  string                  `json:"currentState"`
	Reservations  map[uuid.UUID]uuid.UUID `json:"reservations"` // ProductID → ReservationID
	PaymentID     *uuid.UUID              `json:"paymentId,omitempty"`
	StartedAt     time.Time               `json:"startedAt"`
	CompletedAt   *time.Time              `json:"completedAt,omitempty"`
	FailedAt      *time.Time              `json:"failedAt,omitempty"`
	FailReason    string                  `json:"failReason,omitempty"`
}

// CheckoutItemData represents an item in the checkout process.
type CheckoutItemData struct {
	ProductID uuid.UUID       `json:"productId"`
	Quantity  int             `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
}

// ─── Saga Commands (published to Kafka) ──────────────────────────────────────

type ReserveStockCommand struct {
	CorrelationID uuid.UUID `json:"correlationId"`
	OrderID       uuid.UUID `json:"orderId"`
	ProductID     uuid.UUID `json:"productId"`
	Quantity      int       `json:"quantity"`
}

type ReleaseStockCommand struct {
	CorrelationID uuid.UUID `json:"correlationId"`
	ReservationID uuid.UUID `json:"reservationId"`
	ProductID     uuid.UUID `json:"productId"`
}

type ProcessPaymentCommand struct {
	CorrelationID uuid.UUID       `json:"correlationId"`
	OrderID       uuid.UUID       `json:"orderId"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	PaymentMethod string          `json:"paymentMethod"`
}

type RefundPaymentCommand struct {
	CorrelationID uuid.UUID       `json:"correlationId"`
	PaymentID     uuid.UUID       `json:"paymentId"`
	Amount        decimal.Decimal `json:"amount"`
	Reason        string          `json:"reason"`
}

// ─── Saga Orchestrator ───────────────────────────────────────────────────────

// CheckoutSaga orchestrates the distributed checkout transaction using FSM + Kafka.
//
// Flow: Checkout → Reserve Stock → Process Payment → Confirm Order
// On failure at any step, compensation logic runs in reverse order.
type CheckoutSaga struct {
	producer messaging.EventBus
	store    SagaStore
	logger   *slog.Logger
}

// SagaStore persists saga state for crash recovery.
type SagaStore interface {
	Save(ctx context.Context, data *CheckoutSagaData) error
	GetByCorrelationID(ctx context.Context, id uuid.UUID) (*CheckoutSagaData, error)
	Update(ctx context.Context, data *CheckoutSagaData) error
}

// NewCheckoutSaga creates a new saga orchestrator.
func NewCheckoutSaga(producer messaging.EventBus, store SagaStore, logger *slog.Logger) *CheckoutSaga {
	return &CheckoutSaga{
		producer: producer,
		store:    store,
		logger:   logger,
	}
}

// Start initiates a new checkout saga instance.
func (s *CheckoutSaga) Start(ctx context.Context, data *CheckoutSagaData) error {
	data.CurrentState = StateInitialized
	data.StartedAt = time.Now().UTC()
	data.Reservations = make(map[uuid.UUID]uuid.UUID)

	// Persist initial saga state.
	if err := s.store.Save(ctx, data); err != nil {
		return fmt.Errorf("saving saga state: %w", err)
	}

	s.logger.InfoContext(ctx, "checkout saga started",
		slog.String("correlation_id", data.CorrelationID.String()),
		slog.String("order_id", data.OrderID.String()),
		slog.Int("item_count", len(data.Items)),
	)

	// Transition to ReservingStock and publish commands.
	return s.reserveStock(ctx, data)
}

// reserveStock publishes ReserveStockCommand for each item to Kafka.
func (s *CheckoutSaga) reserveStock(ctx context.Context, data *CheckoutSagaData) error {
	data.CurrentState = StateReservingStock
	if err := s.store.Update(ctx, data); err != nil {
		return fmt.Errorf("updating saga state: %w", err)
	}

	for _, item := range data.Items {
		cmd := ReserveStockCommand{
			CorrelationID: data.CorrelationID,
			OrderID:       data.OrderID,
			ProductID:     item.ProductID,
			Quantity:      item.Quantity,
		}

		payload, err := json.Marshal(cmd)
		if err != nil {
			return fmt.Errorf("serializing reserve stock command: %w", err)
		}

		// Partition by correlation ID — all saga events on same partition for ordering.
		if err := s.producer.Publish(ctx, "saga.reserve-stock", data.CorrelationID.String(), payload); err != nil {
			return fmt.Errorf("publishing reserve stock command: %w", err)
		}

		s.logger.DebugContext(ctx, "reserve stock command published",
			slog.String("product_id", item.ProductID.String()),
			slog.Int("quantity", item.Quantity),
		)
	}

	return nil
}

// HandleStockReserved processes a successful stock reservation event.
func (s *CheckoutSaga) HandleStockReserved(ctx context.Context, correlationID, productID, reservationID uuid.UUID) error {
	data, err := s.store.GetByCorrelationID(ctx, correlationID)
	if err != nil {
		return fmt.Errorf("loading saga: %w", err)
	}

	// Record the reservation.
	data.Reservations[productID] = reservationID

	// Check if all items are reserved.
	allReserved := len(data.Reservations) == len(data.Items)
	if !allReserved {
		return s.store.Update(ctx, data)
	}

	s.logger.InfoContext(ctx, "all stock reserved, proceeding to payment",
		slog.String("correlation_id", correlationID.String()),
	)

	data.CurrentState = StateStockReserved
	return s.processPayment(ctx, data)
}

// processPayment publishes ProcessPaymentCommand to Kafka.
func (s *CheckoutSaga) processPayment(ctx context.Context, data *CheckoutSagaData) error {
	data.CurrentState = StateProcessingPayment
	if err := s.store.Update(ctx, data); err != nil {
		return err
	}

	cmd := ProcessPaymentCommand{
		CorrelationID: data.CorrelationID,
		OrderID:       data.OrderID,
		Amount:        data.TotalAmount,
		Currency:      data.Currency,
		PaymentMethod: "CreditCard",
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("serializing payment command: %w", err)
	}

	return s.producer.Publish(ctx, "saga.process-payment", data.CorrelationID.String(), payload)
}

// HandlePaymentProcessed processes a successful payment event.
func (s *CheckoutSaga) HandlePaymentProcessed(ctx context.Context, correlationID, paymentID uuid.UUID) error {
	data, err := s.store.GetByCorrelationID(ctx, correlationID)
	if err != nil {
		return fmt.Errorf("loading saga: %w", err)
	}

	data.PaymentID = &paymentID
	data.CurrentState = StatePaymentProcessed

	s.logger.InfoContext(ctx, "payment processed, confirming order",
		slog.String("correlation_id", correlationID.String()),
		slog.String("payment_id", paymentID.String()),
	)

	// Final step: confirm the order.
	return s.confirmOrder(ctx, data)
}

// confirmOrder marks the saga as completed and publishes OrderConfirmedEvent.
func (s *CheckoutSaga) confirmOrder(ctx context.Context, data *CheckoutSagaData) error {
	now := time.Now().UTC()
	data.CurrentState = StateCompleted
	data.CompletedAt = &now

	if err := s.store.Update(ctx, data); err != nil {
		return err
	}

	// Publish order confirmed event for downstream services.
	event := map[string]interface{}{
		"correlationId": data.CorrelationID,
		"orderId":       data.OrderID,
		"buyerId":       data.BuyerID,
		"confirmedAt":   now,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	s.logger.InfoContext(ctx, "checkout saga completed",
		slog.String("correlation_id", data.CorrelationID.String()),
		slog.String("order_id", data.OrderID.String()),
	)

	return s.producer.Publish(ctx, "ordering.order.confirmed", data.OrderID.String(), payload)
}

// ─── Compensation (Failure Handlers) ─────────────────────────────────────────

// HandleStockReservationFailed compensates by marking the saga as failed.
func (s *CheckoutSaga) HandleStockReservationFailed(ctx context.Context, correlationID uuid.UUID, reason string) error {
	data, err := s.store.GetByCorrelationID(ctx, correlationID)
	if err != nil {
		return fmt.Errorf("loading saga: %w", err)
	}

	now := time.Now().UTC()
	data.CurrentState = StateFailed
	data.FailedAt = &now
	data.FailReason = reason

	s.logger.WarnContext(ctx, "checkout saga failed: stock reservation",
		slog.String("correlation_id", correlationID.String()),
		slog.String("reason", reason),
	)

	// Release any reservations that were made before the failure.
	for productID, reservationID := range data.Reservations {
		if err := s.compensateReservation(ctx, data, productID, reservationID); err != nil {
			s.logger.ErrorContext(ctx, "failed to compensate reservation",
				slog.String("product_id", productID.String()),
				slog.String("error", err.Error()),
			)
		}
	}

	return s.store.Update(ctx, data)
}

// HandlePaymentFailed compensates by releasing all stock reservations.
func (s *CheckoutSaga) HandlePaymentFailed(ctx context.Context, correlationID uuid.UUID, reason string) error {
	data, err := s.store.GetByCorrelationID(ctx, correlationID)
	if err != nil {
		return fmt.Errorf("loading saga: %w", err)
	}

	now := time.Now().UTC()
	data.CurrentState = StateCompensating
	data.FailedAt = &now
	data.FailReason = reason

	s.logger.WarnContext(ctx, "checkout saga compensating: payment failed",
		slog.String("correlation_id", correlationID.String()),
		slog.String("reason", reason),
	)

	// Compensate: release all stock reservations.
	for productID, reservationID := range data.Reservations {
		if err := s.compensateReservation(ctx, data, productID, reservationID); err != nil {
			s.logger.ErrorContext(ctx, "compensation failed for reservation",
				slog.String("product_id", productID.String()),
				slog.String("error", err.Error()),
			)
		}
	}

	data.CurrentState = StateFailed
	return s.store.Update(ctx, data)
}

func (s *CheckoutSaga) compensateReservation(ctx context.Context, data *CheckoutSagaData, productID, reservationID uuid.UUID) error {
	cmd := ReleaseStockCommand{
		CorrelationID: data.CorrelationID,
		ReservationID: reservationID,
		ProductID:     productID,
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return s.producer.Publish(ctx, "saga.release-stock", data.CorrelationID.String(), payload)
}

// ─── FSM Builder (for validation) ────────────────────────────────────────────

// NewCheckoutFSM creates a finite state machine for validating saga state transitions.
func NewCheckoutFSM(initialState string) *fsm.FSM {
	return fsm.NewFSM(
		initialState,
		fsm.Events{
			{Name: "reserve_stock", Src: []string{StateInitialized}, Dst: StateReservingStock},
			{Name: "stock_reserved", Src: []string{StateReservingStock}, Dst: StateStockReserved},
			{Name: "stock_failed", Src: []string{StateReservingStock}, Dst: StateFailed},
			{Name: "process_payment", Src: []string{StateStockReserved}, Dst: StateProcessingPayment},
			{Name: "payment_success", Src: []string{StateProcessingPayment}, Dst: StatePaymentProcessed},
			{Name: "payment_failed", Src: []string{StateProcessingPayment}, Dst: StateCompensating},
			{Name: "confirm", Src: []string{StatePaymentProcessed}, Dst: StateCompleted},
			{Name: "compensate", Src: []string{StateCompensating}, Dst: StateFailed},
			{Name: "cancel", Src: []string{StateInitialized, StateReservingStock, StateStockReserved}, Dst: StateCancelled},
		},
		fsm.Callbacks{},
	)
}
