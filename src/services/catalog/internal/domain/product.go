package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/shopspring/decimal"
)

// ─── Strongly-Typed IDs ──────────────────────────────────────────────────────

type productTag struct{}
type categoryTag struct{}
type productImageTag struct{}

type ProductID = types.TypedID[productTag]
type CategoryID = types.TypedID[categoryTag]
type ProductImageID = types.TypedID[productImageTag]

func NewProductID() ProductID           { return types.NewTypedID[productTag]() }
func NewCategoryID() CategoryID         { return types.NewTypedID[categoryTag]() }
func NewProductImageID() ProductImageID { return types.NewTypedID[productImageTag]() }

func ProductIDFromString(s string) (ProductID, error) { return types.TypedIDFromString[productTag](s) }
func CategoryIDFromString(s string) (CategoryID, error) {
	return types.TypedIDFromString[categoryTag](s)
}
func ProductImageIDFromString(s string) (ProductImageID, error) {
	return types.TypedIDFromString[productImageTag](s)
}

// ─── Value Objects ───────────────────────────────────────────────────────────

// ProductName is a validated product name (2-200 chars).
type ProductName struct {
	value string
}

func NewProductName(name string) (ProductName, error) {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) < 2 || len(trimmed) > 200 {
		return ProductName{}, errors.New("product name must be 2-200 characters")
	}
	return ProductName{value: trimmed}, nil
}

func (p ProductName) String() string { return p.value }

// CategoryName is a validated category name (2-100 chars).
type CategoryName struct {
	value string
}

func NewCategoryName(name string) (CategoryName, error) {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) < 2 || len(trimmed) > 100 {
		return CategoryName{}, errors.New("category name must be 2-100 characters")
	}
	return CategoryName{value: trimmed}, nil
}

func (c CategoryName) String() string { return c.value }

// SKU is a validated stock keeping unit (alphanumeric + dash/underscore, 2-100 chars).
var skuRegex = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type SKU struct {
	value string
}

func NewSKU(sku string) (SKU, error) {
	trimmed := strings.TrimSpace(sku)
	if len(trimmed) < 2 || len(trimmed) > 100 {
		return SKU{}, errors.New("sku must be 2-100 characters")
	}
	if !skuRegex.MatchString(trimmed) {
		return SKU{}, errors.New("sku contains invalid characters")
	}
	return SKU{value: strings.ToUpper(trimmed)}, nil
}

func (s SKU) String() string { return s.value }

// ─── Product Aggregate Root ──────────────────────────────────────────────────

// Product is the aggregate root for the product bounded context.
// All mutations to Product and its owned entities (ProductImage) go through this root.
type Product struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID            ProductID        `json:"id" db:"id"`
	Name          ProductName      `json:"name" db:"name_value"`
	Description   string           `json:"description" db:"description"`
	SKU           SKU              `json:"sku" db:"sku_value"`
	Price         types.Money      `json:"price"`
	CategoryID    CategoryID       `json:"categoryId" db:"category_id"`
	ImageURL      *string          `json:"imageUrl" db:"image_url"`
	AverageRating *decimal.Decimal `json:"averageRating" db:"average_rating"`
	ReviewCount   int              `json:"reviewCount" db:"review_count"`
	Images        []ProductImage   `json:"images,omitempty"`
}

// NewProduct creates a new Product aggregate with validated fields.
func NewProduct(name, description, sku string, price types.Money, categoryID CategoryID) (*Product, error) {
	productName, err := NewProductName(name)
	if err != nil {
		return nil, err
	}
	productSKU, err := NewSKU(sku)
	if err != nil {
		return nil, err
	}

	p := &Product{
		ID:          NewProductID(),
		Name:        productName,
		Description: truncate(description, 2000),
		SKU:         productSKU,
		Price:       price,
		CategoryID:  categoryID,
		ReviewCount: 0,
	}
	p.SetCreated()

	// Raise domain event for cross-service coordination.
	p.AddDomainEvent(&ProductCreatedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ProductID:       p.ID.Value(),
		SKU:             p.SKU.String(),
	})

	return p, nil
}

// Update modifies the product's mutable fields.
func (p *Product) Update(name, description, sku string, price types.Money, categoryID CategoryID) error {
	productName, err := NewProductName(name)
	if err != nil {
		return err
	}
	productSKU, err := NewSKU(sku)
	if err != nil {
		return err
	}

	p.Name = productName
	p.Description = truncate(description, 2000)
	p.SKU = productSKU
	p.Price = price
	p.CategoryID = categoryID
	p.SetUpdated()
	p.IncrementVersion()
	return nil
}

// UpdateReviewStats updates the cached review statistics from the Reviews service.
func (p *Product) UpdateReviewStats(avgRating *decimal.Decimal, count int) {
	p.AverageRating = avgRating
	p.ReviewCount = count
	p.SetUpdated()
}

// AddImage adds a new image to the product's collection.
func (p *Product) AddImage(url, altText string, displayOrder int) *ProductImage {
	img := NewProductImage(p.ID, url, altText, displayOrder)
	p.Images = append(p.Images, *img)

	// Set as primary image if it's the first one.
	if p.ImageURL == nil {
		p.ImageURL = &url
		p.SetUpdated()
	}

	return img
}

// RemoveImage removes an image by ID from the collection.
func (p *Product) RemoveImage(imageID ProductImageID) {
	for i, img := range p.Images {
		if img.ID == imageID {
			p.Images = append(p.Images[:i], p.Images[i+1:]...)
			break
		}
	}
	p.SetUpdated()
}

// ReorderImages sets new display orders for images.
func (p *Product) ReorderImages(orders map[ProductImageID]int) {
	for i := range p.Images {
		if order, ok := orders[p.Images[i].ID]; ok {
			p.Images[i].DisplayOrder = order
		}
	}
	p.SetUpdated()
}

// ─── Product Image (Owned Entity) ───────────────────────────────────────────

// ProductImage is an owned entity of Product — no independent lifecycle.
type ProductImage struct {
	ID           ProductImageID `json:"id" db:"id"`
	ProductID    ProductID      `json:"productId" db:"product_id"`
	URL          string         `json:"url" db:"url"`
	AltText      string         `json:"altText" db:"alt_text"`
	DisplayOrder int            `json:"displayOrder" db:"display_order"`
	CreatedAt    time.Time      `json:"createdAt" db:"created_at"`
}

// NewProductImage creates a new product image.
func NewProductImage(productID ProductID, url, altText string, displayOrder int) *ProductImage {
	return &ProductImage{
		ID:           NewProductImageID(),
		ProductID:    productID,
		URL:          url,
		AltText:      altText,
		DisplayOrder: displayOrder,
		CreatedAt:    time.Now().UTC(),
	}
}

// ─── Category Aggregate Root ─────────────────────────────────────────────────

// Category is a standalone aggregate root for product categorization.
type Category struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID          CategoryID   `json:"id" db:"id"`
	Name        CategoryName `json:"name" db:"name_value"`
	Description string       `json:"description" db:"description"`
}

// NewCategory creates a new Category aggregate.
func NewCategory(name, description string) (*Category, error) {
	categoryName, err := NewCategoryName(name)
	if err != nil {
		return nil, err
	}

	c := &Category{
		ID:          NewCategoryID(),
		Name:        categoryName,
		Description: truncate(description, 500),
	}
	c.SetCreated()
	return c, nil
}

// Update modifies the category's mutable fields.
func (c *Category) Update(name, description string) error {
	categoryName, err := NewCategoryName(name)
	if err != nil {
		return err
	}
	c.Name = categoryName
	c.Description = truncate(description, 500)
	c.SetUpdated()
	c.IncrementVersion()
	return nil
}

// ─── Domain Events ───────────────────────────────────────────────────────────

// ProductCreatedEvent is published when a new product is created.
// Consumed by Inventory Service to auto-create a StockItem.
type ProductCreatedEvent struct {
	bbdomain.BaseDomainEvent
	ProductID uuid.UUID `json:"productId"`
	SKU       string    `json:"sku"`
}

func (e *ProductCreatedEvent) EventType() string { return "catalog.product.created" }

// ProductUpdatedEvent is published when a product is modified.
type ProductUpdatedEvent struct {
	bbdomain.BaseDomainEvent
	ProductID uuid.UUID `json:"productId"`
}

func (e *ProductUpdatedEvent) EventType() string { return "catalog.product.updated" }

// ProductDeletedEvent is published when a product is deleted.
type ProductDeletedEvent struct {
	bbdomain.BaseDomainEvent
	ProductID uuid.UUID `json:"productId"`
}

func (e *ProductDeletedEvent) EventType() string { return "catalog.product.deleted" }

// ─── Helpers ─────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
