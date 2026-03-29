package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/wishlists/internal/domain"
)

type AddToWishlistCommand struct {
	UserID    string `json:"-"`
	ProductID string `json:"-"`
}

func (c AddToWishlistCommand) CommandName() string { return "AddToWishlistCommand" }

type AddToWishlistResult struct {
	ID string `json:"id"`
}

type AddToWishlistHandler struct {
	repo domain.WishlistRepository
}

func NewAddToWishlistHandler(repo domain.WishlistRepository) *AddToWishlistHandler {
	return &AddToWishlistHandler{repo: repo}
}

func (h *AddToWishlistHandler) Handle(ctx context.Context, cmd AddToWishlistCommand) (*AddToWishlistResult, error) {
	userID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid user id")
	}
	productID, err := uuid.Parse(cmd.ProductID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid product id")
	}

	item, err := domain.NewWishlistItem(userID, productID)
	if err != nil {
		return nil, bberrors.ValidationError(err.Error())
	}

	saved, err := h.repo.Add(ctx, item)
	if err != nil {
		return nil, fmt.Errorf("add wishlist item: %w", err)
	}

	return &AddToWishlistResult{ID: saved.ID.String()}, nil
}

var _ cqrs.CommandHandler[AddToWishlistCommand, *AddToWishlistResult] = (*AddToWishlistHandler)(nil)

type RemoveFromWishlistCommand struct {
	UserID    string `json:"-"`
	ProductID string `json:"-"`
}

func (c RemoveFromWishlistCommand) CommandName() string { return "RemoveFromWishlistCommand" }

type RemoveFromWishlistHandler struct {
	repo domain.WishlistRepository
}

func NewRemoveFromWishlistHandler(repo domain.WishlistRepository) *RemoveFromWishlistHandler {
	return &RemoveFromWishlistHandler{repo: repo}
}

func (h *RemoveFromWishlistHandler) Handle(ctx context.Context, cmd RemoveFromWishlistCommand) (struct{}, error) {
	userID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid user id")
	}
	productID, err := uuid.Parse(cmd.ProductID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid product id")
	}

	if err := h.repo.Remove(ctx, userID, productID); err != nil {
		return struct{}{}, fmt.Errorf("remove wishlist item: %w", err)
	}
	return struct{}{}, nil
}

var _ cqrs.CommandHandler[RemoveFromWishlistCommand, struct{}] = (*RemoveFromWishlistHandler)(nil)
