package queries

import (
	"context"
	"time"

	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	"github.com/osmanozen/oo-commerce/src/services/cart/internal/domain"
	"github.com/shopspring/decimal"
)

// ─── Get Cart Query ─────────────────────────────────────────────────────────

type GetCartQuery struct {
	UserID  *string
	GuestID *string
}

func (q GetCartQuery) QueryName() string { return "GetCartQuery" }

type CartDTO struct {
	ID          string          `json:"id"`
	BuyerID     string          `json:"buyerId"`
	Items       []CartItemDTO   `json:"items"`
	Subtotal    decimal.Decimal `json:"subtotal"`
	ItemCount   int             `json:"itemCount"`
	LastUpdated time.Time       `json:"lastUpdated"`
}

type CartItemDTO struct {
	ID          string          `json:"id"`
	ProductID   string          `json:"productId"`
	ProductName string          `json:"productName"`
	UnitPrice   decimal.Decimal `json:"unitPrice"`
	Currency    string          `json:"currency"`
	Quantity    int             `json:"quantity"`
	ImageURL    *string         `json:"imageUrl,omitempty"`
	AddedAt     time.Time       `json:"addedAt"`
}

type GetCartHandler struct {
	carts domain.CartRepository
}

func NewGetCartHandler(carts domain.CartRepository) *GetCartHandler {
	return &GetCartHandler{carts: carts}
}

func (h *GetCartHandler) Handle(ctx context.Context, query GetCartQuery) (*CartDTO, error) {
	var cart *domain.Cart
	var err error

	if query.UserID != nil && *query.UserID != "" {
		cart, err = h.carts.GetByUserID(ctx, *query.UserID)
	} else if query.GuestID != nil && *query.GuestID != "" {
		cart, err = h.carts.GetByGuestID(ctx, *query.GuestID)
	}

	if err != nil || cart == nil {
		return nil, nil // Return empty (204 No Content typically mapped upstream)
	}

	dto := &CartDTO{
		ID:          cart.ID.String(),
		Subtotal:    cart.SubTotal(),
		ItemCount:   cart.ItemCount(),
		LastUpdated: cart.UpdatedAt,
		Items:       make([]CartItemDTO, 0, len(cart.Items)),
	}

	if cart.Buyer.UserID != nil {
		dto.BuyerID = "user:" + *cart.Buyer.UserID
	} else if cart.Buyer.GuestID != nil {
		dto.BuyerID = "guest:" + *cart.Buyer.GuestID
	}

	for _, item := range cart.Items {
		dto.Items = append(dto.Items, CartItemDTO{
			ID:          item.ID.String(),
			ProductID:   item.ProductID.String(),
			ProductName: item.ProductName,
			UnitPrice:   item.UnitPrice,
			Currency:    item.Currency,
			Quantity:    item.Quantity,
			ImageURL:    item.ImageURL,
			AddedAt:     item.AddedAt,
		})
	}

	return dto, nil
}

var _ cqrs.QueryHandler[GetCartQuery, *CartDTO] = (*GetCartHandler)(nil)
