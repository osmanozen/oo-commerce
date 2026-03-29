package domain

import (
	"context"

	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/persistence"
)

// OrderRepository defines the persistence contract for Order aggregates.
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id OrderID) (*Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*Order, error)
	GetByBuyerID(ctx context.Context, buyerID string, params persistence.PaginationParams) (*persistence.PagedResult[OrderSummaryDTO], error)
	Update(ctx context.Context, order *Order) error
}

// OrderSummaryDTO is the read-model for order list queries.
type OrderSummaryDTO struct {
	ID          string `json:"id" db:"id"`
	OrderNumber string `json:"orderNumber" db:"order_number"`
	Status      string `json:"status" db:"status"`
	Total       string `json:"total" db:"total_amount"`
	Currency    string `json:"currency" db:"total_currency"`
	ItemCount   int    `json:"itemCount" db:"item_count"`
	PlacedAt    string `json:"placedAt" db:"placed_at"`
}
