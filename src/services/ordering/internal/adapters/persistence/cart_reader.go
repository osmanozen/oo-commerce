package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type CartItemSnapshot struct {
	ProductID   uuid.UUID
	ProductName string
	UnitPrice   decimal.Decimal
	Currency    string
	Quantity    int
}

type CartSnapshot struct {
	ID     uuid.UUID
	UserID *string
	Items  []CartItemSnapshot
}

type CartReader struct {
	pool *pgxpool.Pool
}

func NewCartReader(pool *pgxpool.Pool) *CartReader {
	return &CartReader{pool: pool}
}

func (r *CartReader) GetByID(ctx context.Context, id uuid.UUID) (*CartSnapshot, error) {
	var (
		cartID uuid.UUID
		userID *string
	)

	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id
		FROM cart.carts
		WHERE id = $1
	`, id).Scan(&cartID, &userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select cart: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT product_id, product_name, unit_price, currency, quantity
		FROM cart.cart_items
		WHERE cart_id = $1
		ORDER BY added_at ASC
	`, cartID)
	if err != nil {
		return nil, fmt.Errorf("query cart items: %w", err)
	}
	defer rows.Close()

	items := make([]CartItemSnapshot, 0)
	for rows.Next() {
		var item CartItemSnapshot
		if err := rows.Scan(
			&item.ProductID,
			&item.ProductName,
			&item.UnitPrice,
			&item.Currency,
			&item.Quantity,
		); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cart items: %w", err)
	}

	return &CartSnapshot{
		ID:     cartID,
		UserID: userID,
		Items:  items,
	}, nil
}

func (r *CartReader) Clear(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM cart.carts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete cart: %w", err)
	}
	return nil
}
