package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/shopspring/decimal"

	"github.com/osmanozen/oo-commerce/services/wishlists/internal/domain"
)

type CatalogProduct struct {
	ID            uuid.UUID
	Name          string
	PriceAmount   decimal.Decimal
	PriceCurrency string
	ImageURL      *string
	AverageRating *decimal.Decimal
	ReviewCount   int
}

type StockInfo struct {
	ProductID       uuid.UUID
	TotalQuantity   int
	ReservedQuantity int
}

func (s StockInfo) AvailableQuantity() int {
	available := s.TotalQuantity - s.ReservedQuantity
	if available < 0 {
		return 0
	}
	return available
}

type CatalogReader interface {
	GetProductsByIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]CatalogProduct, error)
}

type InventoryReader interface {
	GetStockByProductIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]StockInfo, error)
}

type GetUserWishlistQuery struct {
	UserID string `json:"-"`
}

func (q GetUserWishlistQuery) QueryName() string { return "GetUserWishlistQuery" }

type WishlistItemDTO struct {
	ID                string    `json:"id"`
	ProductID         string    `json:"productId"`
	ProductName       string    `json:"productName"`
	Price             float64   `json:"price"`
	Currency          string    `json:"currency"`
	ImageURL          *string   `json:"imageUrl,omitempty"`
	AverageRating     *float64  `json:"averageRating,omitempty"`
	ReviewCount       int       `json:"reviewCount"`
	AvailableQuantity int       `json:"availableQuantity"`
	AddedAt           time.Time `json:"addedAt"`
}

type GetUserWishlistHandler struct {
	repo      domain.WishlistRepository
	catalog   CatalogReader
	inventory InventoryReader
}

func NewGetUserWishlistHandler(repo domain.WishlistRepository, catalog CatalogReader, inventory InventoryReader) *GetUserWishlistHandler {
	return &GetUserWishlistHandler{
		repo:      repo,
		catalog:   catalog,
		inventory: inventory,
	}
}

func (h *GetUserWishlistHandler) Handle(ctx context.Context, query GetUserWishlistQuery) ([]WishlistItemDTO, error) {
	userID, err := uuid.Parse(query.UserID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid user id")
	}

	items, err := h.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get wishlist items: %w", err)
	}
	if len(items) == 0 {
		return []WishlistItemDTO{}, nil
	}

	productIDs := make([]uuid.UUID, 0, len(items))
	seen := make(map[uuid.UUID]struct{}, len(items))
	for _, item := range items {
		if _, ok := seen[item.ProductID]; ok {
			continue
		}
		seen[item.ProductID] = struct{}{}
		productIDs = append(productIDs, item.ProductID)
	}

	products, err := h.catalog.GetProductsByIDs(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("get catalog products: %w", err)
	}

	stocks, err := h.inventory.GetStockByProductIDs(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("get inventory stocks: %w", err)
	}

	result := make([]WishlistItemDTO, 0, len(items))
	for _, item := range items {
		product, ok := products[item.ProductID]
		if !ok {
			continue
		}

		var avgRating *float64
		if product.AverageRating != nil {
			v, _ := product.AverageRating.Float64()
			avgRating = &v
		}

		stock := stocks[item.ProductID]
		price, _ := product.PriceAmount.Float64()

		result = append(result, WishlistItemDTO{
			ID:                item.ID.String(),
			ProductID:         item.ProductID.String(),
			ProductName:       product.Name,
			Price:             price,
			Currency:          product.PriceCurrency,
			ImageURL:          product.ImageURL,
			AverageRating:     avgRating,
			ReviewCount:       product.ReviewCount,
			AvailableQuantity: stock.AvailableQuantity(),
			AddedAt:           item.AddedAt,
		})
	}

	return result, nil
}

var _ cqrs.QueryHandler[GetUserWishlistQuery, []WishlistItemDTO] = (*GetUserWishlistHandler)(nil)

type GetWishlistCountQuery struct {
	UserID string `json:"-"`
}

func (q GetWishlistCountQuery) QueryName() string { return "GetWishlistCountQuery" }

type GetWishlistCountHandler struct {
	repo domain.WishlistRepository
}

func NewGetWishlistCountHandler(repo domain.WishlistRepository) *GetWishlistCountHandler {
	return &GetWishlistCountHandler{repo: repo}
}

func (h *GetWishlistCountHandler) Handle(ctx context.Context, query GetWishlistCountQuery) (int, error) {
	userID, err := uuid.Parse(query.UserID)
	if err != nil {
		return 0, bberrors.ValidationError("invalid user id")
	}
	count, err := h.repo.CountByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count wishlist items: %w", err)
	}
	return count, nil
}

var _ cqrs.QueryHandler[GetWishlistCountQuery, int] = (*GetWishlistCountHandler)(nil)

type GetWishlistProductIDsQuery struct {
	UserID string `json:"-"`
}

func (q GetWishlistProductIDsQuery) QueryName() string { return "GetWishlistProductIDsQuery" }

type GetWishlistProductIDsHandler struct {
	repo domain.WishlistRepository
}

func NewGetWishlistProductIDsHandler(repo domain.WishlistRepository) *GetWishlistProductIDsHandler {
	return &GetWishlistProductIDsHandler{repo: repo}
}

func (h *GetWishlistProductIDsHandler) Handle(ctx context.Context, query GetWishlistProductIDsQuery) ([]string, error) {
	userID, err := uuid.Parse(query.UserID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid user id")
	}

	productIDs, err := h.repo.GetProductIDsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get wishlist product ids: %w", err)
	}

	result := make([]string, 0, len(productIDs))
	for _, id := range productIDs {
		result = append(result, id.String())
	}
	return result, nil
}

var _ cqrs.QueryHandler[GetWishlistProductIDsQuery, []string] = (*GetWishlistProductIDsHandler)(nil)
