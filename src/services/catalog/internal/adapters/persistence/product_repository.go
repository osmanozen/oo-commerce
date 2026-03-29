package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/persistence"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/osmanozen/oo-commerce/services/catalog/internal/domain"
	"github.com/shopspring/decimal"
)

type ProductRepository struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewProductRepository(pool *pgxpool.Pool, logger *slog.Logger) *ProductRepository {
	return &ProductRepository{pool: pool, logger: logger}
}

func (r *ProductRepository) Create(ctx context.Context, product *domain.Product) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO catalog.products (
			id, name_value, description, sku_value, price_amount, price_currency,
			category_id, image_url, average_rating, review_count, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		product.ID.Value(),
		product.Name.String(),
		product.Description,
		product.SKU.String(),
		product.Price.Amount,
		product.Price.Currency,
		product.CategoryID.Value(),
		product.ImageURL,
		product.AverageRating,
		product.ReviewCount,
		product.CreatedAt,
		product.UpdatedAt,
		product.Version,
	)
	if err != nil {
		return fmt.Errorf("insert product: %w", err)
	}

	for _, e := range product.GetDomainEvents() {
		payload, err := jsonMarshalEvent(e)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO catalog.outbox_messages (message_id, message_type, payload, correlation_id, created_at, retry_count)
			VALUES ($1, $2, $3, $4, $5, 0)
		`, e.EventID(), e.EventType(), payload, nil, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("insert outbox message: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	product.ClearDomainEvents()
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id domain.ProductID) (*domain.Product, error) {
	var (
		dbID          uuid.UUID
		name          string
		description   string
		sku           string
		priceAmount   decimal.Decimal
		priceCurrency string
		categoryID    uuid.UUID
		imageURL      *string
		averageRating *decimal.Decimal
		reviewCount   int
		createdAt     time.Time
		updatedAt     time.Time
		version       int
	)

	err := r.pool.QueryRow(ctx, `
		SELECT id, name_value, description, sku_value, price_amount, price_currency,
			   category_id, image_url, average_rating, review_count, created_at, updated_at, version
		FROM catalog.products
		WHERE id = $1
	`, id.Value()).Scan(
		&dbID, &name, &description, &sku, &priceAmount, &priceCurrency,
		&categoryID, &imageURL, &averageRating, &reviewCount, &createdAt, &updatedAt, &version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select product by id: %w", err)
	}

	catID, err := domain.CategoryIDFromString(categoryID.String())
	if err != nil {
		return nil, fmt.Errorf("parse category id: %w", err)
	}
	money, err := types.NewMoney(priceAmount, priceCurrency)
	if err != nil {
		return nil, fmt.Errorf("build money: %w", err)
	}

	p, err := domain.NewProduct(name, description, sku, money, catID)
	if err != nil {
		return nil, fmt.Errorf("rehydrate product: %w", err)
	}
	pid, err := domain.ProductIDFromString(dbID.String())
	if err != nil {
		return nil, fmt.Errorf("parse product id: %w", err)
	}
	p.ID = pid
	p.ImageURL = imageURL
	p.AverageRating = averageRating
	p.ReviewCount = reviewCount
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	p.Version = version
	p.ClearDomainEvents()

	images, err := r.getImagesByProductID(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("get product images: %w", err)
	}
	p.Images = images

	return p, nil
}

func (r *ProductRepository) GetAll(ctx context.Context, filter domain.ProductFilter, params persistence.PaginationParams) (*persistence.PagedResult[domain.ProductDTO], error) {
	whereParts := []string{"1=1"}
	args := make([]interface{}, 0)
	argPos := 1

	if filter.CategoryID != nil {
		whereParts = append(whereParts, fmt.Sprintf("p.category_id = $%d", argPos))
		args = append(args, *filter.CategoryID)
		argPos++
	}
	if filter.MinPrice != nil {
		whereParts = append(whereParts, fmt.Sprintf("p.price_amount >= $%d", argPos))
		args = append(args, *filter.MinPrice)
		argPos++
	}
	if filter.MaxPrice != nil {
		whereParts = append(whereParts, fmt.Sprintf("p.price_amount <= $%d", argPos))
		args = append(args, *filter.MaxPrice)
		argPos++
	}
	if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
		whereParts = append(whereParts, fmt.Sprintf("(LOWER(p.name_value) LIKE $%d OR LOWER(COALESCE(p.description, '')) LIKE $%d)", argPos, argPos))
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(*filter.Search))+"%")
		argPos++
	}
	whereSQL := strings.Join(whereParts, " AND ")

	var totalCount int
	countSQL := fmt.Sprintf(`SELECT COUNT(1) FROM catalog.products p WHERE %s`, whereSQL)
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("count products: %w", err)
	}

	sortBy := "p.created_at"
	switch strings.ToLower(strings.TrimSpace(filter.SortBy)) {
	case "name":
		sortBy = "p.name_value"
	case "price":
		sortBy = "p.price_amount"
	case "rating":
		sortBy = "p.average_rating"
	case "newest":
		sortBy = "p.created_at"
	}
	sortOrder := "DESC"
	if strings.EqualFold(strings.TrimSpace(filter.SortOrder), "asc") {
		sortOrder = "ASC"
	}

	querySQL := fmt.Sprintf(`
		SELECT p.id, p.name_value, p.description, p.sku_value, p.price_amount, p.price_currency,
			   p.category_id, c.name_value AS category_name, p.image_url, p.average_rating, p.review_count,
			   p.created_at, p.updated_at
		FROM catalog.products p
		INNER JOIN catalog.categories c ON c.id = p.category_id
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, sortBy, sortOrder, argPos, argPos+1)
	args = append(args, params.Limit(), params.Offset())

	rows, err := r.pool.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	items := make([]domain.ProductDTO, 0)
	for rows.Next() {
		var (
			dto       domain.ProductDTO
			createdAt time.Time
			updatedAt time.Time
		)
		if err := rows.Scan(
			&dto.ID, &dto.Name, &dto.Description, &dto.SKU, &dto.Price, &dto.Currency,
			&dto.CategoryID, &dto.CategoryName, &dto.ImageURL, &dto.AverageRating, &dto.ReviewCount,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan product row: %w", err)
		}
		dto.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		dto.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		items = append(items, dto)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate product rows: %w", err)
	}

	result := persistence.NewPagedResult(items, totalCount, params)
	return &result, nil
}

func (r *ProductRepository) Update(ctx context.Context, product *domain.Product) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE catalog.products
		SET name_value = $2, description = $3, sku_value = $4, price_amount = $5, price_currency = $6,
			category_id = $7, image_url = $8, average_rating = $9, review_count = $10, updated_at = $11, version = $12
		WHERE id = $1
	`,
		product.ID.Value(),
		product.Name.String(),
		product.Description,
		product.SKU.String(),
		product.Price.Amount,
		product.Price.Currency,
		product.CategoryID.Value(),
		product.ImageURL,
		product.AverageRating,
		product.ReviewCount,
		product.UpdatedAt,
		product.Version,
	)
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("product", product.ID.String())
	}
	return nil
}

func (r *ProductRepository) Delete(ctx context.Context, id domain.ProductID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM catalog.products WHERE id = $1`, id.Value())
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("product", id.String())
	}
	return nil
}

func (r *ProductRepository) ExistsBySKU(ctx context.Context, sku string, excludeID *domain.ProductID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM catalog.products WHERE sku_value = $1`
	args := []interface{}{strings.ToUpper(strings.TrimSpace(sku))}

	if excludeID != nil {
		query += ` AND id <> $2`
		args = append(args, excludeID.Value())
	}
	query += `)`

	var exists bool
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("check sku exists: %w", err)
	}
	return exists, nil
}

func (r *ProductRepository) UpdateReviewStats(ctx context.Context, productID domain.ProductID, avgRating *decimal.Decimal, reviewCount int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE catalog.products
		SET average_rating = $2, review_count = $3, updated_at = $4
		WHERE id = $1
	`, productID.Value(), avgRating, reviewCount, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update review stats: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("product", productID.String())
	}
	return nil
}

func (r *ProductRepository) getImagesByProductID(ctx context.Context, productID domain.ProductID) ([]domain.ProductImage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, url, alt_text, display_order, created_at
		FROM catalog.product_images
		WHERE product_id = $1
		ORDER BY display_order ASC
	`, productID.Value())
	if err != nil {
		return nil, fmt.Errorf("query product images: %w", err)
	}
	defer rows.Close()

	images := make([]domain.ProductImage, 0)
	for rows.Next() {
		var image domain.ProductImage
		if err := rows.Scan(&image.ID, &image.ProductID, &image.URL, &image.AltText, &image.DisplayOrder, &image.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan image row: %w", err)
		}
		images = append(images, image)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate image rows: %w", err)
	}
	return images, nil
}

func jsonMarshalEvent(event interface{}) (string, error) {
	b, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var _ domain.ProductRepository = (*ProductRepository)(nil)

