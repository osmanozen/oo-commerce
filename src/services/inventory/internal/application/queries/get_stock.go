package queries

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/inventory/internal/domain"
)

// ─── Get Stock Query ─────────────────────────────────────────────────────────

type GetStockQuery struct {
	ProductID string `json:"productId" validate:"required,uuid"`
}

func (q GetStockQuery) QueryName() string { return "GetStockQuery" }

type StockDTO struct {
	ProductID         string `json:"productId"`
	SKU               string `json:"sku"`
	TotalQuantity     int    `json:"totalQuantity"`
	AvailableQuantity int    `json:"availableQuantity"`
	ReservedQuantity  int    `json:"reservedQuantity"`
	LowStockLevel     int    `json:"lowStockLevel"`
	IsLowStock        bool   `json:"isLowStock"`
}

type GetStockHandler struct {
	stockRepo domain.StockItemRepository
}

func NewGetStockHandler(stockRepo domain.StockItemRepository) *GetStockHandler {
	return &GetStockHandler{stockRepo: stockRepo}
}

func (h *GetStockHandler) Handle(ctx context.Context, query GetStockQuery) (*StockDTO, error) {
	productID, err := uuid.Parse(query.ProductID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid product id")
	}

	stockItem, err := h.stockRepo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("finding stock item: %w", err)
	}
	if stockItem == nil {
		return nil, bberrors.NotFoundError("stock item", query.ProductID)
	}

	available := stockItem.AvailableQuantity()
	reserved := stockItem.TotalQuantity - available

	return &StockDTO{
		ProductID:         stockItem.ProductID.String(),
		SKU:               stockItem.SKU,
		TotalQuantity:     stockItem.TotalQuantity,
		AvailableQuantity: available,
		ReservedQuantity:  reserved,
		LowStockLevel:     stockItem.LowStockLevel,
		IsLowStock:        available <= stockItem.LowStockLevel,
	}, nil
}

var _ cqrs.QueryHandler[GetStockQuery, *StockDTO] = (*GetStockHandler)(nil)

// ─── Batch Get Stock Levels ─────────────────────────────────────────────────

type GetStockLevelsQuery struct {
	ProductIDs []string `json:"productIds" validate:"required,min=1,max=100"`
}

func (q GetStockLevelsQuery) QueryName() string { return "GetStockLevelsQuery" }

type StockLevelDTO struct {
	ProductID string `json:"productId"`
	Available int    `json:"available"`
	InStock   bool   `json:"inStock"`
}

type GetStockLevelsHandler struct {
	stockRepo domain.StockItemRepository
}

func NewGetStockLevelsHandler(stockRepo domain.StockItemRepository) *GetStockLevelsHandler {
	return &GetStockLevelsHandler{stockRepo: stockRepo}
}

func (h *GetStockLevelsHandler) Handle(ctx context.Context, query GetStockLevelsQuery) ([]StockLevelDTO, error) {
	productIDs := make([]uuid.UUID, 0, len(query.ProductIDs))
	for _, idStr := range query.ProductIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		productIDs = append(productIDs, id)
	}

	results := make([]StockLevelDTO, 0, len(productIDs))
	for _, pid := range productIDs {
		stockItem, err := h.stockRepo.GetByProductID(ctx, pid)
		if err != nil || stockItem == nil {
			results = append(results, StockLevelDTO{
				ProductID: pid.String(),
				Available: 0,
				InStock:   false,
			})
			continue
		}

		available := stockItem.AvailableQuantity()
		results = append(results, StockLevelDTO{
			ProductID: pid.String(),
			Available: available,
			InStock:   available > 0,
		})
	}

	return results, nil
}

var _ cqrs.QueryHandler[GetStockLevelsQuery, []StockLevelDTO] = (*GetStockLevelsHandler)(nil)
