package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
)

// ─── Strongly-Typed IDs ──────────────────────────────────────────────────────

type wishlistItemTag struct{}

type WishlistItemID = types.TypedID[wishlistItemTag]

func NewWishlistItemID() WishlistItemID { return types.NewTypedID[wishlistItemTag]() }
func WishlistItemIDFromString(s string) (WishlistItemID, error) {
	return types.TypedIDFromString[wishlistItemTag](s)
}

// ─── Wishlist Item (Aggregate Root — simple domain) ─────────────────────────

// WishlistItem is the aggregate root for the wishlists bounded context.
// Simplified domain model — each item is independent (no parent Wishlist entity).
type WishlistItem struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID        WishlistItemID `json:"id" db:"id"`
	UserID    uuid.UUID      `json:"userId" db:"user_id"`
	ProductID uuid.UUID      `json:"productId" db:"product_id"`
	AddedAt   time.Time      `json:"addedAt" db:"added_at"`
}

// NewWishlistItem creates a new wishlist item.
func NewWishlistItem(userID uuid.UUID, productID uuid.UUID) (*WishlistItem, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id cannot be nil")
	}
	if productID == uuid.Nil {
		return nil, errors.New("product id cannot be nil")
	}

	item := &WishlistItem{
		ID:        NewWishlistItemID(),
		UserID:    userID,
		ProductID: productID,
		AddedAt:   time.Now().UTC(),
	}
	item.SetCreated()
	return item, nil
}

// ─── Repository ──────────────────────────────────────────────────────────────

type WishlistRepository interface {
	Add(ctx context.Context, item *WishlistItem) (*WishlistItem, error)
	Remove(ctx context.Context, userID, productID uuid.UUID) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]WishlistItem, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	GetProductIDsByUserID(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}
