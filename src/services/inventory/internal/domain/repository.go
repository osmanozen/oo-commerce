package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/types"
)

// StockItemRepository defines the persistence contract for StockItem aggregates.
type StockItemRepository interface {
	// Create persists a new stock item.
	Create(ctx context.Context, item *StockItem) error

	// GetByID retrieves a stock item by its ID.
	GetByID(ctx context.Context, id StockItemID) (*StockItem, error)

	// GetByProductID retrieves a stock item by its product reference.
	GetByProductID(ctx context.Context, productID uuid.UUID) (*StockItem, error)

	// Update persists changes with optimistic locking.
	Update(ctx context.Context, item *StockItem) error

	// GetReservationsByOrderID returns all reservations for a given order.
	GetReservationsByOrderID(ctx context.Context, orderID uuid.UUID) ([]StockReservation, error)

	// GetExpiredReservations returns reservations that have passed their TTL.
	GetExpiredReservations(ctx context.Context) ([]StockReservation, error)
}

// ReservationIDFromUUID converts a uuid.UUID into a typed ReservationID.
func ReservationIDFromUUID(id uuid.UUID) (ReservationID, error) {
	return types.TypedIDFrom[reservationTag](id)
}
