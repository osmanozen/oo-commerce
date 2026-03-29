package queries

import (
	"context"
	"fmt"
	"strings"

	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/persistence"
	"github.com/osmanozen/oo-commerce/src/services/ordering/internal/domain"
)

type GetUserOrdersQuery struct {
	UserID   string `json:"-"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

func (q GetUserOrdersQuery) QueryName() string { return "GetUserOrdersQuery" }

type GetUserOrdersHandler struct {
	orders domain.OrderRepository
}

func NewGetUserOrdersHandler(orders domain.OrderRepository) *GetUserOrdersHandler {
	return &GetUserOrdersHandler{orders: orders}
}

func (h *GetUserOrdersHandler) Handle(
	ctx context.Context,
	query GetUserOrdersQuery,
) (*persistence.PagedResult[domain.OrderSummaryDTO], error) {
	if strings.TrimSpace(query.UserID) == "" {
		return nil, fmt.Errorf("user id is required")
	}

	params := persistence.NewPaginationParams(query.Page, query.PageSize)
	result, err := h.orders.GetByBuyerID(ctx, query.UserID, params)
	if err != nil {
		return nil, fmt.Errorf("load user orders: %w", err)
	}
	return result, nil
}

var _ cqrs.QueryHandler[GetUserOrdersQuery, *persistence.PagedResult[domain.OrderSummaryDTO]] = (*GetUserOrdersHandler)(nil)
