package domain

import (
	"context"
)

// CartRepository defines the persistence contract for Cart aggregates.
type CartRepository interface {
	Create(ctx context.Context, cart *Cart) error
	GetByID(ctx context.Context, id CartID) (*Cart, error)
	GetByUserID(ctx context.Context, userID string) (*Cart, error)
	GetByGuestID(ctx context.Context, guestID string) (*Cart, error)
	Update(ctx context.Context, cart *Cart) error
	Delete(ctx context.Context, id CartID) error
	// CleanupAbandoned deletes carts not updated within the given duration (hours).
	CleanupAbandoned(ctx context.Context, olderThanHours int) (int64, error)
}
