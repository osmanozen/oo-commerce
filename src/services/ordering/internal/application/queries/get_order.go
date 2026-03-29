package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/domain"
)

type GetOrderByIDQuery struct {
	UserID  string `json:"-"`
	OrderID string `json:"-"`
}

func (q GetOrderByIDQuery) QueryName() string { return "GetOrderByIDQuery" }

type MoneyDTO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type OrderItemDTO struct {
	ID          string   `json:"id"`
	ProductID   string   `json:"productId"`
	ProductName string   `json:"productName"`
	Price       MoneyDTO `json:"price"`
	Quantity    int      `json:"quantity"`
	LineTotal   MoneyDTO `json:"lineTotal"`
}

type OrderDetailsDTO struct {
	ID              string         `json:"id"`
	OrderNumber     string         `json:"orderNumber"`
	BuyerID         string         `json:"buyerId"`
	Status          string         `json:"status"`
	SubTotal        MoneyDTO       `json:"subtotal"`
	Tax             MoneyDTO       `json:"tax"`
	Total           MoneyDTO       `json:"total"`
	Items           []OrderItemDTO `json:"items"`
	ShippingAddress domain.OrderAddress `json:"shippingAddress"`
	BillingAddress  domain.OrderAddress `json:"billingAddress"`
	PaymentMethod   string         `json:"paymentMethod"`
	PlacedAt        *string        `json:"placedAt,omitempty"`
	PaidAt          *string        `json:"paidAt,omitempty"`
	ConfirmedAt     *string        `json:"confirmedAt,omitempty"`
	ShippedAt       *string        `json:"shippedAt,omitempty"`
	DeliveredAt     *string        `json:"deliveredAt,omitempty"`
	CancelledAt     *string        `json:"cancelledAt,omitempty"`
	CreatedAt       string         `json:"createdAt"`
	UpdatedAt       string         `json:"updatedAt"`
}

type GetOrderByIDHandler struct {
	orders domain.OrderRepository
}

func NewGetOrderByIDHandler(orders domain.OrderRepository) *GetOrderByIDHandler {
	return &GetOrderByIDHandler{orders: orders}
}

func (h *GetOrderByIDHandler) Handle(ctx context.Context, query GetOrderByIDQuery) (*OrderDetailsDTO, error) {
	orderID, err := domain.OrderIDFromString(query.OrderID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid order id")
	}

	order, err := h.orders.GetByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("load order: %w", err)
	}
	if order == nil {
		return nil, bberrors.NotFoundError("order", query.OrderID)
	}
	if order.BuyerID != query.UserID {
		return nil, bberrors.NewDomainError(bberrors.ErrForbidden, "order does not belong to user")
	}

	items := make([]OrderItemDTO, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, OrderItemDTO{
			ID:          item.ID.String(),
			ProductID:   item.ProductID.String(),
			ProductName: item.ProductName,
			Price: MoneyDTO{
				Amount:   item.Price.Amount.StringFixed(2),
				Currency: item.Price.Currency,
			},
			Quantity: item.Quantity,
			LineTotal: MoneyDTO{
				Amount:   item.LineTotal.Amount.StringFixed(2),
				Currency: item.LineTotal.Currency,
			},
		})
	}

	dto := &OrderDetailsDTO{
		ID:          order.ID.String(),
		OrderNumber: order.OrderNumber,
		BuyerID:     order.BuyerID,
		Status:      order.Status.String(),
		SubTotal: MoneyDTO{
			Amount:   order.SubTotal.Amount.StringFixed(2),
			Currency: order.SubTotal.Currency,
		},
		Tax: MoneyDTO{
			Amount:   order.Tax.Amount.StringFixed(2),
			Currency: order.Tax.Currency,
		},
		Total: MoneyDTO{
			Amount:   order.Total.Amount.StringFixed(2),
			Currency: order.Total.Currency,
		},
		Items:           items,
		ShippingAddress: order.ShippingAddress,
		BillingAddress:  order.BillingAddress,
		PaymentMethod:   order.PaymentMethod.String(),
		CreatedAt:       order.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       order.UpdatedAt.UTC().Format(time.RFC3339),
	}
	dto.PlacedAt = formatTimePtr(order.PlacedAt)
	dto.PaidAt = formatTimePtr(order.PaidAt)
	dto.ConfirmedAt = formatTimePtr(order.ConfirmedAt)
	dto.ShippedAt = formatTimePtr(order.ShippedAt)
	dto.DeliveredAt = formatTimePtr(order.DeliveredAt)
	dto.CancelledAt = formatTimePtr(order.CancelledAt)

	return dto, nil
}

var _ cqrs.QueryHandler[GetOrderByIDQuery, *OrderDetailsDTO] = (*GetOrderByIDHandler)(nil)

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}
