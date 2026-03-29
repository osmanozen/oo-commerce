package commands

import (
	"context"

	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/cart/internal/domain"
)

// ─── Update Cart Item Quantity Command ──────────────────────────────────────

type UpdateCartItemQuantityCommand struct {
	CartID   string `json:"-"`
	ItemID   string `json:"itemId" validate:"required,uuid"`
	Quantity int    `json:"quantity" validate:"min=0,max=99"`
}

func (c UpdateCartItemQuantityCommand) CommandName() string {
	return "UpdateCartItemQuantityCommand"
}

type UpdateCartItemQuantityHandler struct {
	carts domain.CartRepository
}

func NewUpdateCartItemQuantityHandler(carts domain.CartRepository) *UpdateCartItemQuantityHandler {
	return &UpdateCartItemQuantityHandler{carts: carts}
}

func (h *UpdateCartItemQuantityHandler) Handle(ctx context.Context, cmd UpdateCartItemQuantityCommand) (struct{}, error) {
	cartID, err := domain.CartIDFromString(cmd.CartID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid cart id")
	}

	cart, err := h.carts.GetByID(ctx, cartID)
	if err != nil || cart == nil {
		return struct{}{}, bberrors.NotFoundError("cart", cmd.CartID)
	}

	itemID, err := domain.CartItemIDFromString(cmd.ItemID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid item id")
	}

	if err := cart.UpdateQuantity(itemID, cmd.Quantity); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	return struct{}{}, h.carts.Update(ctx, cart)
}

var _ cqrs.CommandHandler[UpdateCartItemQuantityCommand, struct{}] = (*UpdateCartItemQuantityHandler)(nil)

// ─── Clear Cart Command ──────────────────────────────────────────────────────

type ClearCartCommand struct {
	CartID string `json:"-"`
}

func (c ClearCartCommand) CommandName() string { return "ClearCartCommand" }

type ClearCartHandler struct {
	carts domain.CartRepository
}

func NewClearCartHandler(carts domain.CartRepository) *ClearCartHandler {
	return &ClearCartHandler{carts: carts}
}

func (h *ClearCartHandler) Handle(ctx context.Context, cmd ClearCartCommand) (struct{}, error) {
	cartID, err := domain.CartIDFromString(cmd.CartID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid cart id")
	}

	cart, err := h.carts.GetByID(ctx, cartID)
	if err != nil || cart == nil {
		return struct{}{}, bberrors.NotFoundError("cart", cmd.CartID)
	}

	cart.Clear()

	return struct{}{}, h.carts.Update(ctx, cart)
}

var _ cqrs.CommandHandler[ClearCartCommand, struct{}] = (*ClearCartHandler)(nil)
