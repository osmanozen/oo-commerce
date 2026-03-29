package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/messaging"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/adapters/persistence"
)

type SimulatePaymentCommand struct {
	OrderID string `json:"-"`
	Success bool   `json:"success"`
}

func (c SimulatePaymentCommand) CommandName() string { return "SimulatePaymentCommand" }

type SimulatePaymentHandler struct {
	bus       messaging.EventBus
	sagaStore *persistence.SagaStore
}

func NewSimulatePaymentHandler(bus messaging.EventBus, sagaStore *persistence.SagaStore) *SimulatePaymentHandler {
	return &SimulatePaymentHandler{
		bus:       bus,
		sagaStore: sagaStore,
	}
}

func (h *SimulatePaymentHandler) Handle(ctx context.Context, cmd SimulatePaymentCommand) (struct{}, error) {
	orderID, err := uuid.Parse(cmd.OrderID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid order id")
	}
	correlationID, err := h.sagaStore.GetCorrelationIDByOrderID(ctx, orderID)
	if err != nil {
		return struct{}{}, fmt.Errorf("find checkout saga for order: %w", err)
	}

	topic := "payment.failed"
	payload := map[string]interface{}{
		"correlationId": correlationID,
		"orderId":       orderID,
		"reason":        "simulated payment failure",
	}
	if cmd.Success {
		topic = "payment.processed"
		payload = map[string]interface{}{
			"correlationId": correlationID,
			"orderId":       orderID,
			"paymentId":     uuid.Must(uuid.NewV7()),
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return struct{}{}, fmt.Errorf("marshal payment simulation payload: %w", err)
	}
	if err := h.bus.Publish(ctx, topic, orderID.String(), data); err != nil {
		return struct{}{}, fmt.Errorf("publish payment simulation event: %w", err)
	}
	return struct{}{}, nil
}

var _ cqrs.CommandHandler[SimulatePaymentCommand, struct{}] = (*SimulatePaymentHandler)(nil)
