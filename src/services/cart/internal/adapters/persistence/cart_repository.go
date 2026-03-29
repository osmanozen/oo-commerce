package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/cart/internal/domain"
	"github.com/shopspring/decimal"
)

type CartRepository struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewCartRepository(pool *pgxpool.Pool, logger *slog.Logger) *CartRepository {
	return &CartRepository{pool: pool, logger: logger}
}

func (r *CartRepository) GetByID(ctx context.Context, id domain.CartID) (*domain.Cart, error) {
	return r.getBy(ctx, "id = $1", id.Value())
}

func (r *CartRepository) GetByUserID(ctx context.Context, userID string) (*domain.Cart, error) {
	return r.getBy(ctx, "user_id = $1", userID)
}

func (r *CartRepository) GetByGuestID(ctx context.Context, guestID string) (*domain.Cart, error) {
	return r.getBy(ctx, "guest_id = $1", guestID)
}

func (r *CartRepository) Create(ctx context.Context, cart *domain.Cart) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO cart.carts (id, user_id, guest_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`,
		cart.ID.Value(),
		nullableString(cart.Buyer.UserID),
		nullableString(cart.Buyer.GuestID),
		cart.CreatedAt,
		cart.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, "idx_carts_user_id") {
			return bberrors.ConflictError("cart", "userId", safeString(cart.Buyer.UserID))
		}
		if isUniqueViolation(err, "idx_carts_guest_id") {
			return bberrors.ConflictError("cart", "guestId", safeString(cart.Buyer.GuestID))
		}
		return fmt.Errorf("insert cart: %w", err)
	}

	if err := r.replaceItems(ctx, tx, cart); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *CartRepository) Update(ctx context.Context, cart *domain.Cart) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE cart.carts
		SET user_id = $2, guest_id = $3, updated_at = $4
		WHERE id = $1
	`,
		cart.ID.Value(),
		nullableString(cart.Buyer.UserID),
		nullableString(cart.Buyer.GuestID),
		cart.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update cart: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("cart", cart.ID.String())
	}

	if err := r.replaceItems(ctx, tx, cart); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *CartRepository) Delete(ctx context.Context, id domain.CartID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM cart.carts WHERE id = $1`, id.Value())
	if err != nil {
		return fmt.Errorf("delete cart: %w", err)
	}
	return nil
}

func (r *CartRepository) CleanupAbandoned(ctx context.Context, olderThanHours int) (int64, error) {
	if olderThanHours <= 0 {
		return 0, bberrors.ValidationError("olderThanHours must be positive")
	}

	tag, err := r.pool.Exec(ctx, `
		DELETE FROM cart.carts
		WHERE updated_at < NOW() - make_interval(hours => $1)
	`, olderThanHours)
	if err != nil {
		return 0, fmt.Errorf("cleanup abandoned carts: %w", err)
	}
	return tag.RowsAffected(), nil
}

var _ domain.CartRepository = (*CartRepository)(nil)

func (r *CartRepository) getBy(ctx context.Context, whereClause string, args ...interface{}) (*domain.Cart, error) {
	query := fmt.Sprintf(`
		SELECT id, user_id, guest_id, created_at, updated_at
		FROM cart.carts
		WHERE %s
	`, whereClause)

	var (
		idRaw     uuid.UUID
		userID    *string
		guestID   *string
		createdAt time.Time
		updatedAt time.Time
	)

	err := r.pool.QueryRow(ctx, query, args...).Scan(&idRaw, &userID, &guestID, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select cart: %w", err)
	}

	cartID, err := domain.CartIDFromString(idRaw.String())
	if err != nil {
		return nil, fmt.Errorf("parse cart id: %w", err)
	}

	cart := &domain.Cart{
		ID:    cartID,
		Buyer: domain.BuyerIdentity{UserID: userID, GuestID: guestID},
		Items: make([]domain.CartItem, 0),
	}
	cart.CreatedAt = createdAt
	cart.UpdatedAt = updatedAt
	cart.ClearDomainEvents()

	items, err := r.loadItems(ctx, cart.ID)
	if err != nil {
		return nil, err
	}
	cart.Items = items

	return cart, nil
}

func (r *CartRepository) loadItems(ctx context.Context, cartID domain.CartID) ([]domain.CartItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, cart_id, product_id, product_name, image_url, unit_price, currency, quantity, added_at
		FROM cart.cart_items
		WHERE cart_id = $1
		ORDER BY added_at ASC
	`, cartID.Value())
	if err != nil {
		return nil, fmt.Errorf("query cart items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.CartItem, 0)
	for rows.Next() {
		var (
			idRaw       uuid.UUID
			cartIDRaw   uuid.UUID
			productID   uuid.UUID
			productName string
			imageURL    *string
			unitPrice   decimal.Decimal
			currency    string
			quantity    int
			addedAt     time.Time
		)
		if err := rows.Scan(&idRaw, &cartIDRaw, &productID, &productName, &imageURL, &unitPrice, &currency, &quantity, &addedAt); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}

		itemID, err := domain.CartItemIDFromString(idRaw.String())
		if err != nil {
			return nil, fmt.Errorf("parse cart item id: %w", err)
		}
		itemCartID, err := domain.CartIDFromString(cartIDRaw.String())
		if err != nil {
			return nil, fmt.Errorf("parse item cart id: %w", err)
		}

		items = append(items, domain.CartItem{
			ID:          itemID,
			CartID:      itemCartID,
			ProductID:   productID,
			ProductName: productName,
			ImageURL:    imageURL,
			UnitPrice:   unitPrice,
			Currency:    currency,
			Quantity:    quantity,
			AddedAt:     addedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cart items: %w", err)
	}
	return items, nil
}

func (r *CartRepository) replaceItems(ctx context.Context, tx pgx.Tx, cart *domain.Cart) error {
	if _, err := tx.Exec(ctx, `DELETE FROM cart.cart_items WHERE cart_id = $1`, cart.ID.Value()); err != nil {
		return fmt.Errorf("delete existing cart items: %w", err)
	}

	for _, item := range cart.Items {
		_, err := tx.Exec(ctx, `
			INSERT INTO cart.cart_items (id, cart_id, product_id, product_name, image_url, unit_price, currency, quantity, added_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`,
			item.ID.Value(),
			cart.ID.Value(),
			item.ProductID,
			item.ProductName,
			item.ImageURL,
			item.UnitPrice,
			item.Currency,
			item.Quantity,
			item.AddedAt,
		)
		if err != nil {
			return fmt.Errorf("insert cart item %s: %w", item.ID.String(), err)
		}
	}

	return nil
}

func nullableString(v *string) interface{} {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}

func safeString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && strings.EqualFold(pgErr.ConstraintName, constraint)
	}
	return false
}
