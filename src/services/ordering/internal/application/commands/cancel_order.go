package commands

import (
	"context"
	"fmt"

	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/ordering/internal/domain"
)

type CancelOrderCommand struct {
	UserID  string `json:"-"`
	OrderID string `json:"-"`
	Reason  string `json:"reason"`
}

func (c CancelOrderCommand) CommandName() string { return "CancelOrderCommand" }

type CancelOrderHandler struct {
	orders domain.OrderRepository
}

func NewCancelOrderHandler(orders domain.OrderRepository) *CancelOrderHandler {
	return &CancelOrderHandler{orders: orders}
}

func (h *CancelOrderHandler) Handle(ctx context.Context, cmd CancelOrderCommand) (struct{}, error) {
	orderID, err := domain.OrderIDFromString(cmd.OrderID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid order id")
	}

	order, err := h.orders.GetByID(ctx, orderID)
	if err != nil {
		return struct{}{}, fmt.Errorf("load order: %w", err)
	}
	if order == nil {
		return struct{}{}, bberrors.NotFoundError("order", cmd.OrderID)
	}
	if order.BuyerID != cmd.UserID {
		return struct{}{}, bberrors.NewDomainError(bberrors.ErrForbidden, "order does not belong to user")
	}
	if !order.CanBeCancelled() {
		return struct{}{}, bberrors.NewDomainError(bberrors.ErrInvalidState, "order cannot be cancelled in current state")
	}

	if err := order.Cancel(cmd.Reason); err != nil {
		return struct{}{}, bberrors.NewDomainError(bberrors.ErrInvalidState, err.Error())
	}
	if err := h.orders.Update(ctx, order); err != nil {
		return struct{}{}, fmt.Errorf("save cancelled order: %w", err)
	}
	return struct{}{}, nil
}

var _ cqrs.CommandHandler[CancelOrderCommand, struct{}] = (*CancelOrderHandler)(nil)
