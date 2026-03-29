package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/osmanozen/oo-commerce/services/catalog/internal/domain"
	"github.com/shopspring/decimal"
)

// ─── Create Product Command ─────────────────────────────────────────────────

type CreateProductCommand struct {
	Name        string  `json:"name" validate:"required,min=2,max=200"`
	Description string  `json:"description" validate:"max=2000"`
	SKU         string  `json:"sku" validate:"required,min=2,max=100"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Currency    string  `json:"currency" validate:"required,len=3"`
	CategoryID  string  `json:"categoryId" validate:"required,uuid"`
}

func (c CreateProductCommand) CommandName() string { return "CreateProductCommand" }

type CreateProductResult struct {
	ID uuid.UUID `json:"id"`
}

type CreateProductHandler struct {
	products   domain.ProductRepository
	categories domain.CategoryRepository
}

func NewCreateProductHandler(products domain.ProductRepository, categories domain.CategoryRepository) *CreateProductHandler {
	return &CreateProductHandler{products: products, categories: categories}
}

func (h *CreateProductHandler) Handle(ctx context.Context, cmd CreateProductCommand) (CreateProductResult, error) {
	// 1. Verify category exists.
	catID, err := domain.CategoryIDFromString(cmd.CategoryID)
	if err != nil {
		return CreateProductResult{}, bberrors.ValidationError("invalid category id")
	}
	exists, err := h.categories.Exists(ctx, catID)
	if err != nil {
		return CreateProductResult{}, fmt.Errorf("checking category: %w", err)
	}
	if !exists {
		return CreateProductResult{}, bberrors.NotFoundError("category", cmd.CategoryID)
	}

	// 2. Check SKU uniqueness.
	skuExists, err := h.products.ExistsBySKU(ctx, cmd.SKU, nil)
	if err != nil {
		return CreateProductResult{}, fmt.Errorf("checking sku: %w", err)
	}
	if skuExists {
		return CreateProductResult{}, bberrors.ConflictError("product", "sku", cmd.SKU)
	}

	// 3. Create Money value object.
	price, err := types.NewMoney(decimal.NewFromFloat(cmd.Price), cmd.Currency)
	if err != nil {
		return CreateProductResult{}, bberrors.ValidationError(err.Error())
	}

	// 4. Create Product aggregate (raises ProductCreatedEvent).
	product, err := domain.NewProduct(cmd.Name, cmd.Description, cmd.SKU, price, catID)
	if err != nil {
		return CreateProductResult{}, bberrors.ValidationError(err.Error())
	}

	// 5. Persist (including outbox events in same transaction).
	if err := h.products.Create(ctx, product); err != nil {
		return CreateProductResult{}, fmt.Errorf("creating product: %w", err)
	}

	return CreateProductResult{ID: product.ID.Value()}, nil
}

// Compile-time check.
var _ cqrs.CommandHandler[CreateProductCommand, CreateProductResult] = (*CreateProductHandler)(nil)

// ─── Update Product Command ─────────────────────────────────────────────────

type UpdateProductCommand struct {
	ID          string  `json:"-"`
	Name        string  `json:"name" validate:"required,min=2,max=200"`
	Description string  `json:"description" validate:"max=2000"`
	SKU         string  `json:"sku" validate:"required,min=2,max=100"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Currency    string  `json:"currency" validate:"required,len=3"`
	CategoryID  string  `json:"categoryId" validate:"required,uuid"`
}

func (c UpdateProductCommand) CommandName() string { return "UpdateProductCommand" }

type UpdateProductHandler struct {
	products   domain.ProductRepository
	categories domain.CategoryRepository
}

func NewUpdateProductHandler(products domain.ProductRepository, categories domain.CategoryRepository) *UpdateProductHandler {
	return &UpdateProductHandler{products: products, categories: categories}
}

func (h *UpdateProductHandler) Handle(ctx context.Context, cmd UpdateProductCommand) (struct{}, error) {
	productID, err := domain.ProductIDFromString(cmd.ID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid product id")
	}

	product, err := h.products.GetByID(ctx, productID)
	if err != nil {
		return struct{}{}, fmt.Errorf("finding product: %w", err)
	}
	if product == nil {
		return struct{}{}, bberrors.NotFoundError("product", cmd.ID)
	}

	catID, err := domain.CategoryIDFromString(cmd.CategoryID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid category id")
	}
	exists, err := h.categories.Exists(ctx, catID)
	if err != nil {
		return struct{}{}, fmt.Errorf("checking category: %w", err)
	}
	if !exists {
		return struct{}{}, bberrors.NotFoundError("category", cmd.CategoryID)
	}

	// Check SKU uniqueness (excluding current product).
	skuExists, err := h.products.ExistsBySKU(ctx, cmd.SKU, &productID)
	if err != nil {
		return struct{}{}, fmt.Errorf("checking sku: %w", err)
	}
	if skuExists {
		return struct{}{}, bberrors.ConflictError("product", "sku", cmd.SKU)
	}

	price, err := types.NewMoney(decimal.NewFromFloat(cmd.Price), cmd.Currency)
	if err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	if err := product.Update(cmd.Name, cmd.Description, cmd.SKU, price, catID); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	if err := h.products.Update(ctx, product); err != nil {
		return struct{}{}, fmt.Errorf("updating product: %w", err)
	}

	return struct{}{}, nil
}

var _ cqrs.CommandHandler[UpdateProductCommand, struct{}] = (*UpdateProductHandler)(nil)

// ─── Delete Product Command ─────────────────────────────────────────────────

type DeleteProductCommand struct {
	ID string `json:"-"`
}

func (c DeleteProductCommand) CommandName() string { return "DeleteProductCommand" }

type DeleteProductHandler struct {
	products domain.ProductRepository
}

func NewDeleteProductHandler(products domain.ProductRepository) *DeleteProductHandler {
	return &DeleteProductHandler{products: products}
}

func (h *DeleteProductHandler) Handle(ctx context.Context, cmd DeleteProductCommand) (struct{}, error) {
	productID, err := domain.ProductIDFromString(cmd.ID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid product id")
	}

	if err := h.products.Delete(ctx, productID); err != nil {
		return struct{}{}, fmt.Errorf("deleting product: %w", err)
	}

	return struct{}{}, nil
}

var _ cqrs.CommandHandler[DeleteProductCommand, struct{}] = (*DeleteProductHandler)(nil)



// ─── Update Review Stats Command (Internal, consumed from Reviews) ──────────

type UpdateReviewStatsCommand struct {
	ProductID   string   `json:"productId" validate:"required,uuid"`
	AvgRating   *float64 `json:"averageRating"`
	ReviewCount int      `json:"reviewCount" validate:"gte=0"`
}

func (c UpdateReviewStatsCommand) CommandName() string { return "UpdateReviewStatsCommand" }

type UpdateReviewStatsHandler struct {
	products domain.ProductRepository
}

func NewUpdateReviewStatsHandler(products domain.ProductRepository) *UpdateReviewStatsHandler {
	return &UpdateReviewStatsHandler{products: products}
}

func (h *UpdateReviewStatsHandler) Handle(ctx context.Context, cmd UpdateReviewStatsCommand) (struct{}, error) {
	productID, err := domain.ProductIDFromString(cmd.ProductID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid product id")
	}

	var avgRating *decimal.Decimal
	if cmd.AvgRating != nil {
		d := decimal.NewFromFloat(*cmd.AvgRating)
		avgRating = &d
	}

	if err := h.products.UpdateReviewStats(ctx, productID, avgRating, cmd.ReviewCount); err != nil {
		return struct{}{}, fmt.Errorf("updating review stats: %w", err)
	}

	return struct{}{}, nil
}

var _ cqrs.CommandHandler[UpdateReviewStatsCommand, struct{}] = (*UpdateReviewStatsHandler)(nil)
