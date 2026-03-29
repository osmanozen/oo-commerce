package queries

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/persistence"
	"github.com/osmanozen/oo-commerce/src/services/catalog/internal/domain"
	"github.com/shopspring/decimal"
)

// ─── Get Products Query (Paginated + Filtered) ──────────────────────────────

type GetProductsQuery struct {
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
	CategoryID *string  `json:"categoryId,omitempty"`
	MinPrice   *float64 `json:"minPrice,omitempty"`
	MaxPrice   *float64 `json:"maxPrice,omitempty"`
	Search     *string  `json:"search,omitempty"`
	SortBy     string   `json:"sortBy,omitempty"`
	SortOrder  string   `json:"sortOrder,omitempty"`
}

func (q GetProductsQuery) QueryName() string { return "GetProductsQuery" }

type GetProductsHandler struct {
	products domain.ProductRepository
}

func NewGetProductsHandler(products domain.ProductRepository) *GetProductsHandler {
	return &GetProductsHandler{products: products}
}

func (h *GetProductsHandler) Handle(ctx context.Context, query GetProductsQuery) (persistence.PagedResult[domain.ProductDTO], error) {
	params := persistence.NewPaginationParams(query.Page, query.PageSize)

	filter := domain.ProductFilter{
		SortBy:    query.SortBy,
		SortOrder: query.SortOrder,
	}

	if query.CategoryID != nil {
		id, err := uuid.Parse(*query.CategoryID)
		if err == nil {
			filter.CategoryID = &id
		}
	}
	if query.MinPrice != nil {
		d := decimal.NewFromFloat(*query.MinPrice)
		filter.MinPrice = &d
	}
	if query.MaxPrice != nil {
		d := decimal.NewFromFloat(*query.MaxPrice)
		filter.MaxPrice = &d
	}
	if query.Search != nil && *query.Search != "" {
		filter.Search = query.Search
	}

	result, err := h.products.GetAll(ctx, filter, params)
	if err != nil {
		return persistence.PagedResult[domain.ProductDTO]{}, fmt.Errorf("querying products: %w", err)
	}

	return *result, nil
}

var _ cqrs.QueryHandler[GetProductsQuery, persistence.PagedResult[domain.ProductDTO]] = (*GetProductsHandler)(nil)

// ─── Get Product By ID Query ────────────────────────────────────────────────

type GetProductByIDQuery struct {
	ID string `json:"id"`
}

func (q GetProductByIDQuery) QueryName() string { return "GetProductByIDQuery" }

type ProductDetailDTO struct {
	domain.ProductDTO
	Images []domain.ProductImage `json:"images"`
}

type GetProductByIDHandler struct {
	products domain.ProductRepository
}

func NewGetProductByIDHandler(products domain.ProductRepository) *GetProductByIDHandler {
	return &GetProductByIDHandler{products: products}
}

func (h *GetProductByIDHandler) Handle(ctx context.Context, query GetProductByIDQuery) (*ProductDetailDTO, error) {
	productID, err := domain.ProductIDFromString(query.ID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid product id")
	}

	product, err := h.products.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("finding product: %w", err)
	}
	if product == nil {
		return nil, bberrors.NotFoundError("product", query.ID)
	}

	dto := &ProductDetailDTO{
		ProductDTO: domain.ProductDTO{
			ID:            product.ID.Value(),
			Name:          product.Name.String(),
			Description:   product.Description,
			SKU:           product.SKU.String(),
			Price:         product.Price.Amount,
			Currency:      product.Price.Currency,
			CategoryID:    product.CategoryID.Value(),
			ImageURL:      product.ImageURL,
			AverageRating: product.AverageRating,
			ReviewCount:   product.ReviewCount,
		},
		Images: product.Images,
	}

	return dto, nil
}

var _ cqrs.QueryHandler[GetProductByIDQuery, *ProductDetailDTO] = (*GetProductByIDHandler)(nil)

