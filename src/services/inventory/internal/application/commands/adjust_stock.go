package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/inventory/internal/domain"
)

// ─── Adjust Stock Command ───────────────────────────────────────────────────

type AdjustStockCommand struct {
	ProductID      string `json:"productId" validate:"required,uuid"`
	AdjustmentType int    `json:"adjustmentType" validate:"required,min=1,max=5"`
	Quantity       int    `json:"quantity" validate:"required,ne=0"`
	Reason         string `json:"reason" validate:"max=500"`
	CreatedBy      string `json:"-"`
}

func (c AdjustStockCommand) CommandName() string { return "AdjustStockCommand" }

type AdjustStockHandler struct {
	stockRepo domain.StockItemRepository
}

func NewAdjustStockHandler(stockRepo domain.StockItemRepository) *AdjustStockHandler {
	return &AdjustStockHandler{stockRepo: stockRepo}
}

func (h *AdjustStockHandler) Handle(ctx context.Context, cmd AdjustStockCommand) (struct{}, error) {
	productID, err := uuid.Parse(cmd.ProductID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid product id")
	}

	stockItem, err := h.stockRepo.GetByProductID(ctx, productID)
	if err != nil {
		return struct{}{}, fmt.Errorf("finding stock item: %w", err)
	}
	if stockItem == nil {
		return struct{}{}, bberrors.NotFoundError("stock item", cmd.ProductID)
	}

	adjustType := domain.AdjustmentType(cmd.AdjustmentType)
	if err := stockItem.AdjustStock(adjustType, cmd.Quantity, cmd.Reason, cmd.CreatedBy); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	if err := h.stockRepo.Update(ctx, stockItem); err != nil {
		return struct{}{}, fmt.Errorf("saving stock adjustment: %w", err)
	}

	return struct{}{}, nil
}

var _ cqrs.CommandHandler[AdjustStockCommand, struct{}] = (*AdjustStockHandler)(nil)
