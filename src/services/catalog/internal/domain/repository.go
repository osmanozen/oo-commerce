package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/persistence"
	"github.com/shopspring/decimal"
)

// ProductRepository defines the persistence contract for Product aggregates.
// Interface is defined in the domain layer — implementations live in adapters.
type ProductRepository interface {
	// Create persists a new product aggregate (including outbox event).
	Create(ctx context.Context, product *Product) error

	// GetByID retrieves a product by its ID, including images.
	GetByID(ctx context.Context, id ProductID) (*Product, error)

	// GetAll retrieves products with filtering, sorting, and pagination.
	GetAll(ctx context.Context, filter ProductFilter, params persistence.PaginationParams) (*persistence.PagedResult[ProductDTO], error)

	// Update persists changes to an existing product (with optimistic locking).
	Update(ctx context.Context, product *Product) error

	// Delete removes a product by ID.
	Delete(ctx context.Context, id ProductID) error

	// ExistsBySKU checks if a product with the given SKU already exists.
	ExistsBySKU(ctx context.Context, sku string, excludeID *ProductID) (bool, error)

	// UpdateReviewStats updates cached review statistics.
	UpdateReviewStats(ctx context.Context, productID ProductID, avgRating *decimal.Decimal, reviewCount int) error
}

// CategoryRepository defines the persistence contract for Category aggregates.
type CategoryRepository interface {
	// Create persists a new category.
	Create(ctx context.Context, category *Category) error

	// GetByID retrieves a category by ID.
	GetByID(ctx context.Context, id CategoryID) (*Category, error)

	// GetAll retrieves all categories ordered by name.
	GetAll(ctx context.Context) ([]CategoryDTO, error)

	// Update persists changes to a category.
	Update(ctx context.Context, category *Category) error

	// Delete removes a category and cascades to products.
	Delete(ctx context.Context, id CategoryID) error

	// Exists checks if a category exists by ID.
	Exists(ctx context.Context, id CategoryID) (bool, error)
}

// ProductImageRepository defines persistence for product images.
type ProductImageRepository interface {
	// Create persists a new product image.
	Create(ctx context.Context, image *ProductImage) error

	// Delete removes a product image by ID.
	Delete(ctx context.Context, id ProductImageID) (*ProductImage, error)

	// GetByProductID returns all images for a product, ordered by display order.
	GetByProductID(ctx context.Context, productID ProductID) ([]ProductImage, error)

	// UpdateDisplayOrders batch-updates image display orders.
	UpdateDisplayOrders(ctx context.Context, orders map[ProductImageID]int) error
}

// ─── Filter & DTOs ───────────────────────────────────────────────────────────

// ProductFilter represents the specification pattern for product queries.
type ProductFilter struct {
	CategoryID *uuid.UUID       `json:"categoryId,omitempty"`
	MinPrice   *decimal.Decimal `json:"minPrice,omitempty"`
	MaxPrice   *decimal.Decimal `json:"maxPrice,omitempty"`
	Search     *string          `json:"search,omitempty"`
	SortBy     string           `json:"sortBy,omitempty"`    // "name", "price", "newest", "rating"
	SortOrder  string           `json:"sortOrder,omitempty"` // "asc", "desc"
}

// ProductDTO is the read-model for product list queries.
type ProductDTO struct {
	ID            uuid.UUID        `json:"id" db:"id"`
	Name          string           `json:"name" db:"name_value"`
	Description   string           `json:"description" db:"description"`
	SKU           string           `json:"sku" db:"sku_value"`
	Price         decimal.Decimal  `json:"price" db:"price_amount"`
	Currency      string           `json:"currency" db:"price_currency"`
	CategoryID    uuid.UUID        `json:"categoryId" db:"category_id"`
	CategoryName  string           `json:"categoryName" db:"category_name"`
	ImageURL      *string          `json:"imageUrl" db:"image_url"`
	AverageRating *decimal.Decimal `json:"averageRating" db:"average_rating"`
	ReviewCount   int              `json:"reviewCount" db:"review_count"`
	CreatedAt     string           `json:"createdAt" db:"created_at"`
	UpdatedAt     string           `json:"updatedAt" db:"updated_at"`
}

// CategoryDTO is the read-model for category list queries.
type CategoryDTO struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Name         string    `json:"name" db:"name_value"`
	Description  string    `json:"description" db:"description"`
	ProductCount int       `json:"productCount" db:"product_count"`
	CreatedAt    string    `json:"createdAt" db:"created_at"`
	UpdatedAt    string    `json:"updatedAt" db:"updated_at"`
}

// ImageStorage defines the contract for blob storage operations.
type ImageStorage interface {
	// Upload stores an image and returns the public URL.
	Upload(ctx context.Context, filename string, data []byte, contentType string) (string, error)

	// Delete removes an image by URL. Fails gracefully (best-effort).
	Delete(ctx context.Context, url string) error
}
