package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osmanozen/oo-commerce/services/reviews/internal/domain"
	"github.com/shopspring/decimal"
)

type ReviewRepository struct {
	pool *pgxpool.Pool
}

func NewReviewRepository(pool *pgxpool.Pool) *ReviewRepository {
	return &ReviewRepository{pool: pool}
}

func (r *ReviewRepository) Create(ctx context.Context, review *domain.Review) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO reviews.reviews (
			id, product_id, user_id, rating, review_text, created_at, updated_at, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		review.ID.Value(),
		review.ProductID,
		review.UserID,
		review.Rating.Value(),
		review.Text.String(),
		review.CreatedAt,
		review.UpdatedAt,
		review.Version,
	)
	if err != nil {
		if isUniqueViolation(err, "uq_reviews_user_product") {
			return bberrors.ConflictError("review", "userId+productId", review.UserID.String())
		}
		return fmt.Errorf("insert review: %w", err)
	}

	if err := r.insertOutboxMessages(ctx, tx, review.GetDomainEvents(), nil); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	review.ClearDomainEvents()
	return nil
}

func (r *ReviewRepository) GetByID(ctx context.Context, id domain.ReviewID) (*domain.Review, error) {
	return r.getBy(ctx, "id = $1", id.Value())
}

func (r *ReviewRepository) GetByUserAndProduct(ctx context.Context, userID, productID uuid.UUID) (*domain.Review, error) {
	return r.getBy(ctx, "user_id = $1 AND product_id = $2", userID, productID)
}

func (r *ReviewRepository) getBy(ctx context.Context, whereClause string, args ...interface{}) (*domain.Review, error) {
	query := fmt.Sprintf(`
		SELECT id, product_id, user_id, rating, review_text, created_at, updated_at, version
		FROM reviews.reviews
		WHERE %s
	`, whereClause)

	var (
		idRaw     uuid.UUID
		productID uuid.UUID
		userID    uuid.UUID
		ratingVal int
		textVal   string
		createdAt time.Time
		updatedAt time.Time
		version   int
	)

	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&idRaw,
		&productID,
		&userID,
		&ratingVal,
		&textVal,
		&createdAt,
		&updatedAt,
		&version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select review: %w", err)
	}

	rating, err := domain.NewRating(ratingVal)
	if err != nil {
		return nil, fmt.Errorf("hydrate rating: %w", err)
	}
	text, err := domain.NewReviewText(textVal)
	if err != nil {
		return nil, fmt.Errorf("hydrate review text: %w", err)
	}
	typedID, err := domain.ReviewIDFromString(idRaw.String())
	if err != nil {
		return nil, fmt.Errorf("hydrate review id: %w", err)
	}

	review := &domain.Review{
		ID:        typedID,
		ProductID: productID,
		UserID:    userID,
		Rating:    rating,
		Text:      text,
	}
	review.CreatedAt = createdAt
	review.UpdatedAt = updatedAt
	review.Version = version
	review.ClearDomainEvents()
	return review, nil
}

func (r *ReviewRepository) GetByProductID(ctx context.Context, productID uuid.UUID, offset, limit int) ([]domain.Review, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(1) FROM reviews.reviews WHERE product_id = $1`, productID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reviews: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, user_id, rating, review_text, created_at, updated_at, version
		FROM reviews.reviews
		WHERE product_id = $1
		ORDER BY created_at DESC
		OFFSET $2 LIMIT $3
	`, productID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("query reviews by product: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Review, 0, limit)
	for rows.Next() {
		var (
			idRaw     uuid.UUID
			productID uuid.UUID
			userID    uuid.UUID
			ratingVal int
			textVal   string
			createdAt time.Time
			updatedAt time.Time
			version   int
		)

		if err := rows.Scan(&idRaw, &productID, &userID, &ratingVal, &textVal, &createdAt, &updatedAt, &version); err != nil {
			return nil, 0, fmt.Errorf("scan review row: %w", err)
		}

		rating, err := domain.NewRating(ratingVal)
		if err != nil {
			return nil, 0, fmt.Errorf("hydrate rating: %w", err)
		}
		text, err := domain.NewReviewText(textVal)
		if err != nil {
			return nil, 0, fmt.Errorf("hydrate review text: %w", err)
		}
		typedID, err := domain.ReviewIDFromString(idRaw.String())
		if err != nil {
			return nil, 0, fmt.Errorf("hydrate review id: %w", err)
		}

		review := domain.Review{
			ID:        typedID,
			ProductID: productID,
			UserID:    userID,
			Rating:    rating,
			Text:      text,
		}
		review.CreatedAt = createdAt
		review.UpdatedAt = updatedAt
		review.Version = version
		review.ClearDomainEvents()
		items = append(items, review)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate reviews rows: %w", err)
	}
	return items, total, nil
}

func (r *ReviewRepository) Update(ctx context.Context, review *domain.Review) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE reviews.reviews
		SET rating = $2, review_text = $3, updated_at = $4, version = $5
		WHERE id = $1
	`, review.ID.Value(), review.Rating.Value(), review.Text.String(), review.UpdatedAt, review.Version)
	if err != nil {
		return fmt.Errorf("update review: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("review", review.ID.String())
	}

	if err := r.insertOutboxMessages(ctx, tx, review.GetDomainEvents(), nil); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	review.ClearDomainEvents()
	return nil
}

func (r *ReviewRepository) Delete(ctx context.Context, review *domain.Review) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM reviews.reviews WHERE id = $1`, review.ID.Value())
	if err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("review", review.ID.String())
	}

	if err := r.insertOutboxMessages(ctx, tx, review.GetDomainEvents(), nil); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	review.ClearDomainEvents()
	return nil
}

func (r *ReviewRepository) GetRatingStats(ctx context.Context, productID uuid.UUID) (*domain.RatingStats, error) {
	var (
		avg   *decimal.Decimal
		count int
	)
	if err := r.pool.QueryRow(ctx, `
		SELECT ROUND(AVG(rating)::numeric, 2), COUNT(1)
		FROM reviews.reviews
		WHERE product_id = $1
	`, productID).Scan(&avg, &count); err != nil {
		return nil, fmt.Errorf("select rating stats: %w", err)
	}

	return &domain.RatingStats{
		ProductID:     productID,
		AverageRating: avg,
		ReviewCount:   count,
	}, nil
}

func (r *ReviewRepository) ExistsByUserAndProduct(ctx context.Context, userID, productID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM reviews.reviews
			WHERE user_id = $1 AND product_id = $2
		)
	`, userID, productID).Scan(&exists); err != nil {
		return false, fmt.Errorf("exists review by user and product: %w", err)
	}
	return exists, nil
}

func (r *ReviewRepository) insertOutboxMessages(ctx context.Context, tx pgx.Tx, events []bbdomain.DomainEvent, correlationID *uuid.UUID) error {
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO reviews.outbox_messages (message_id, message_type, payload, correlation_id, created_at, retry_count)
			VALUES ($1, $2, $3, $4, $5, 0)
		`,
			e.EventID(),
			e.EventType(),
			string(payload),
			correlationID,
			time.Now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("insert outbox message: %w", err)
		}
	}
	return nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && strings.EqualFold(pgErr.ConstraintName, constraint)
	}
	return false
}

var _ domain.ReviewRepository = (*ReviewRepository)(nil)
