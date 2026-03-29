package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osmanozen/oo-commerce/src/services/wishlists/internal/domain"
)

type WishlistRepository struct {
	pool *pgxpool.Pool
}

func NewWishlistRepository(pool *pgxpool.Pool) *WishlistRepository {
	return &WishlistRepository{pool: pool}
}

func (r *WishlistRepository) Add(ctx context.Context, item *domain.WishlistItem) (*domain.WishlistItem, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO wishlists.wishlist_items (id, user_id, product_id, added_at, created_at, updated_at, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`,
		item.ID.Value(),
		item.UserID,
		item.ProductID,
		item.AddedAt,
		item.CreatedAt,
		item.UpdatedAt,
		item.Version,
	)
	if err == nil {
		return item, nil
	}

	if isUniqueViolation(err, "uq_wishlist_user_product") {
		existing, getErr := r.getByUserAndProduct(ctx, item.UserID, item.ProductID)
		if getErr != nil {
			return nil, fmt.Errorf("load existing wishlist item: %w", getErr)
		}
		if existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("wishlist unique conflict but existing item not found")
	}
	return nil, fmt.Errorf("insert wishlist item: %w", err)
}

func (r *WishlistRepository) Remove(ctx context.Context, userID, productID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM wishlists.wishlist_items
		WHERE user_id = $1 AND product_id = $2
	`, userID, productID)
	if err != nil {
		return fmt.Errorf("delete wishlist item: %w", err)
	}
	return nil
}

func (r *WishlistRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]domain.WishlistItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, product_id, added_at, created_at, updated_at, version
		FROM wishlists.wishlist_items
		WHERE user_id = $1
		ORDER BY added_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query wishlist by user: %w", err)
	}
	defer rows.Close()

	result := make([]domain.WishlistItem, 0)
	for rows.Next() {
		item, scanErr := scanWishlistItem(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan wishlist item: %w", scanErr)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wishlist rows: %w", err)
	}

	return result, nil
}

func (r *WishlistRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(1) FROM wishlists.wishlist_items WHERE user_id = $1
	`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count wishlist items: %w", err)
	}
	return count, nil
}

func (r *WishlistRepository) GetProductIDsByUserID(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT product_id
		FROM wishlists.wishlist_items
		WHERE user_id = $1
		ORDER BY added_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query wishlist product ids: %w", err)
	}
	defer rows.Close()

	result := make([]uuid.UUID, 0)
	for rows.Next() {
		var productID uuid.UUID
		if err := rows.Scan(&productID); err != nil {
			return nil, fmt.Errorf("scan product id: %w", err)
		}
		result = append(result, productID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate product id rows: %w", err)
	}
	return result, nil
}

func (r *WishlistRepository) getByUserAndProduct(ctx context.Context, userID, productID uuid.UUID) (*domain.WishlistItem, error) {
	var (
		rawID      uuid.UUID
		addedAt    time.Time
		createdAt  time.Time
		updatedAt  time.Time
		version    int
	)
	err := r.pool.QueryRow(ctx, `
		SELECT id, added_at, created_at, updated_at, version
		FROM wishlists.wishlist_items
		WHERE user_id = $1 AND product_id = $2
	`, userID, productID).Scan(&rawID, &addedAt, &createdAt, &updatedAt, &version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select wishlist item: %w", err)
	}

	typedID, err := domain.WishlistItemIDFromString(rawID.String())
	if err != nil {
		return nil, fmt.Errorf("parse wishlist id: %w", err)
	}

	item := &domain.WishlistItem{
		ID:        typedID,
		UserID:    userID,
		ProductID: productID,
		AddedAt:   addedAt,
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	item.Version = version
	return item, nil
}

func scanWishlistItem(row pgx.Row) (domain.WishlistItem, error) {
	var (
		rawID      uuid.UUID
		userID     uuid.UUID
		productID  uuid.UUID
		addedAt    time.Time
		createdAt  time.Time
		updatedAt  time.Time
		version    int
	)
	if err := row.Scan(&rawID, &userID, &productID, &addedAt, &createdAt, &updatedAt, &version); err != nil {
		return domain.WishlistItem{}, err
	}

	typedID, err := domain.WishlistItemIDFromString(rawID.String())
	if err != nil {
		return domain.WishlistItem{}, err
	}

	item := domain.WishlistItem{
		ID:        typedID,
		UserID:    userID,
		ProductID: productID,
		AddedAt:   addedAt,
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	item.Version = version
	return item, nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && strings.EqualFold(pgErr.ConstraintName, constraint)
	}
	return false
}

var _ domain.WishlistRepository = (*WishlistRepository)(nil)
