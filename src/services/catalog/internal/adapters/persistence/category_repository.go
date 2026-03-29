package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/catalog/internal/domain"
)

type CategoryRepository struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewCategoryRepository(pool *pgxpool.Pool, logger *slog.Logger) *CategoryRepository {
	return &CategoryRepository{pool: pool, logger: logger}
}

func (r *CategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO catalog.categories (id, name_value, description, created_at, updated_at, version)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		category.ID.Value(),
		category.Name.String(),
		category.Description,
		category.CreatedAt,
		category.UpdatedAt,
		category.Version,
	)
	if err != nil {
		return fmt.Errorf("insert category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) GetByID(ctx context.Context, id domain.CategoryID) (*domain.Category, error) {
	var (
		dbID        uuid.UUID
		name        string
		description string
		createdAt   time.Time
		updatedAt   time.Time
		version     int
	)

	err := r.pool.QueryRow(ctx, `
		SELECT id, name_value, description, created_at, updated_at, version
		FROM catalog.categories
		WHERE id = $1
	`, id.Value()).Scan(&dbID, &name, &description, &createdAt, &updatedAt, &version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select category by id: %w", err)
	}

	c, err := domain.NewCategory(name, description)
	if err != nil {
		return nil, fmt.Errorf("rehydrate category: %w", err)
	}
	catID, err := domain.CategoryIDFromString(dbID.String())
	if err != nil {
		return nil, fmt.Errorf("parse category id: %w", err)
	}
	c.ID = catID
	c.CreatedAt = createdAt
	c.UpdatedAt = updatedAt
	c.Version = version
	c.ClearDomainEvents()

	return c, nil
}

func (r *CategoryRepository) GetAll(ctx context.Context) ([]domain.CategoryDTO, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name_value, c.description,
			   (SELECT COUNT(1) FROM catalog.products p WHERE p.category_id = c.id) AS product_count,
			   c.created_at, c.updated_at
		FROM catalog.categories c
		ORDER BY c.name_value ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	items := make([]domain.CategoryDTO, 0)
	for rows.Next() {
		var (
			dto       domain.CategoryDTO
			createdAt time.Time
			updatedAt time.Time
		)
		if err := rows.Scan(&dto.ID, &dto.Name, &dto.Description, &dto.ProductCount, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan category row: %w", err)
		}
		dto.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		dto.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		items = append(items, dto)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category rows: %w", err)
	}

	return items, nil
}

func (r *CategoryRepository) Update(ctx context.Context, category *domain.Category) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE catalog.categories
		SET name_value = $2, description = $3, updated_at = $4, version = $5
		WHERE id = $1
	`, category.ID.Value(), category.Name.String(), category.Description, category.UpdatedAt, category.Version)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("category", category.ID.String())
	}
	return nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id domain.CategoryID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM catalog.categories WHERE id = $1`, id.Value())
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("category", id.String())
	}
	return nil
}

func (r *CategoryRepository) Exists(ctx context.Context, id domain.CategoryID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog.categories WHERE id = $1)`, id.Value()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check category exists: %w", err)
	}
	return exists, nil
}

var _ domain.CategoryRepository = (*CategoryRepository)(nil)

