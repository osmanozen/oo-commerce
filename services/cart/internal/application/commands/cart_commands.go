package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/cart/internal/domain"
	"github.com/shopspring/decimal"
)

// ─── Add To Cart Command ────────────────────────────────────────────────────

type AddToCartCommand struct {
	UserID      *string `json:"-"`
	GuestID     *string `json:"-"`
	ProductID   string  `json:"productId" validate:"required,uuid"`
	ProductName string  `json:"productName" validate:"required,max=200"`
	ImageURL    *string `json:"imageUrl,omitempty"`
	UnitPrice   float64 `json:"unitPrice" validate:"required,gt=0"`
	Currency    string  `json:"currency" validate:"required,len=3"`
	Quantity    int     `json:"quantity" validate:"required,min=1,max=99"`
}

func (c AddToCartCommand) CommandName() string { return "AddToCartCommand" }

type AddToCartHandler struct {
	carts domain.CartRepository
}

func NewAddToCartHandler(carts domain.CartRepository) *AddToCartHandler {
	return &AddToCartHandler{carts: carts}
}

func (h *AddToCartHandler) Handle(ctx context.Context, cmd AddToCartCommand) (struct{}, error) {
	productID, err := uuid.Parse(cmd.ProductID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid product id")
	}

	// Find or create cart for buyer.
	cart, isNew, err := h.findOrCreateCart(ctx, cmd.UserID, cmd.GuestID)
	if err != nil {
		return struct{}{}, fmt.Errorf("finding cart: %w", err)
	}

	// Add item to cart.
	unitPrice := decimal.NewFromFloat(cmd.UnitPrice)
	if err := cart.AddItem(productID, cmd.ProductName, cmd.ImageURL, unitPrice, cmd.Currency, cmd.Quantity); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	// Persist.
	if isNew {
		if err := h.carts.Create(ctx, cart); err != nil {
			return struct{}{}, fmt.Errorf("saving cart: %w", err)
		}
		return struct{}{}, nil
	}

	if err := h.carts.Update(ctx, cart); err != nil {
		return struct{}{}, fmt.Errorf("saving cart: %w", err)
	}

	return struct{}{}, nil
}

func (h *AddToCartHandler) findOrCreateCart(ctx context.Context, userID, guestID *string) (*domain.Cart, bool, error) {
	if userID != nil && *userID != "" {
		cart, err := h.carts.GetByUserID(ctx, *userID)
		if err == nil && cart != nil {
			return cart, false, nil
		}
		created, createErr := domain.NewCart(domain.BuyerIdentity{UserID: userID})
		return created, true, createErr
	}
	if guestID != nil && *guestID != "" {
		cart, err := h.carts.GetByGuestID(ctx, *guestID)
		if err == nil && cart != nil {
			return cart, false, nil
		}
		created, createErr := domain.NewCart(domain.BuyerIdentity{GuestID: guestID})
		return created, true, createErr
	}
	return nil, false, bberrors.ValidationError("either user id or guest id is required")
}

var _ cqrs.CommandHandler[AddToCartCommand, struct{}] = (*AddToCartHandler)(nil)

// ─── Remove Item Command ───────────────────────────────────────────────────

type RemoveFromCartCommand struct {
	CartID string `json:"-"`
	ItemID string `json:"itemId" validate:"required,uuid"`
}

func (c RemoveFromCartCommand) CommandName() string { return "RemoveFromCartCommand" }

type RemoveFromCartHandler struct {
	carts domain.CartRepository
}

func NewRemoveFromCartHandler(carts domain.CartRepository) *RemoveFromCartHandler {
	return &RemoveFromCartHandler{carts: carts}
}

func (h *RemoveFromCartHandler) Handle(ctx context.Context, cmd RemoveFromCartCommand) (struct{}, error) {
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

	if err := cart.RemoveItem(itemID); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	return struct{}{}, h.carts.Update(ctx, cart)
}

var _ cqrs.CommandHandler[RemoveFromCartCommand, struct{}] = (*RemoveFromCartHandler)(nil)

// ─── Merge Cart Command ─────────────────────────────────────────────────────

// MergeCartCommand transfers guest cart items to the user's cart on login.
type MergeCartCommand struct {
	UserID  string `json:"-" validate:"required"`
	GuestID string `json:"-" validate:"required"`
}

func (c MergeCartCommand) CommandName() string { return "MergeCartCommand" }

type MergeCartHandler struct {
	carts domain.CartRepository
}

func NewMergeCartHandler(carts domain.CartRepository) *MergeCartHandler {
	return &MergeCartHandler{carts: carts}
}

func (h *MergeCartHandler) Handle(ctx context.Context, cmd MergeCartCommand) (struct{}, error) {
	guestCart, err := h.carts.GetByGuestID(ctx, cmd.GuestID)
	if err != nil || guestCart == nil || len(guestCart.Items) == 0 {
		return struct{}{}, nil // nothing to merge
	}

	// Find or create user cart.
	userCart, err := h.carts.GetByUserID(ctx, cmd.UserID)
	if err != nil || userCart == nil {
		userCart, err = domain.NewCart(domain.BuyerIdentity{UserID: &cmd.UserID})
		if err != nil {
			return struct{}{}, fmt.Errorf("creating user cart: %w", err)
		}
		if err := h.carts.Create(ctx, userCart); err != nil {
			return struct{}{}, fmt.Errorf("saving new user cart: %w", err)
		}
	}

	userCart.MergeFrom(guestCart)

	if err := h.carts.Update(ctx, userCart); err != nil {
		return struct{}{}, fmt.Errorf("saving merged cart: %w", err)
	}

	// Delete guest cart.
	_ = h.carts.Delete(ctx, guestCart.ID)

	return struct{}{}, nil
}

var _ cqrs.CommandHandler[MergeCartCommand, struct{}] = (*MergeCartHandler)(nil)

// Helper for parsing typed IDs from strings.
func CartItemIDFromString(s string) (domain.CartItemID, error) {
	return domain.CartItemIDFromString(s)
}
