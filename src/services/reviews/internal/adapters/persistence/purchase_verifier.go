package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osmanozen/oo-commerce/src/services/reviews/internal/domain"
)

type PurchaseVerifier struct {
	pool *pgxpool.Pool
}

func NewPurchaseVerifier(pool *pgxpool.Pool) *PurchaseVerifier {
	return &PurchaseVerifier{pool: pool}
}

func (v *PurchaseVerifier) HasPurchased(ctx context.Context, userID string, productID uuid.UUID) (bool, error) {
	var exists bool
	if err := v.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM ordering.orders o
			JOIN ordering.order_items oi ON oi.order_id = o.id
			WHERE o.buyer_id = $1
			  AND oi.product_id = $2
			  AND o.status IN (3,4,5,6)
		)
	`, userID, productID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check purchase existence: %w", err)
	}
	return exists, nil
}

func (v *PurchaseVerifier) GetVerifiedUserIDs(ctx context.Context, productID uuid.UUID, userIDs []string) (map[string]struct{}, error) {
	out := make(map[string]struct{}, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	rows, err := v.pool.Query(ctx, `
		SELECT DISTINCT o.buyer_id
		FROM ordering.orders o
		JOIN ordering.order_items oi ON oi.order_id = o.id
		WHERE oi.product_id = $1
		  AND o.buyer_id = ANY($2)
		  AND o.status IN (3,4,5,6)
	`, productID, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query verified user ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan verified user id: %w", err)
		}
		out[userID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate verified user ids: %w", err)
	}
	return out, nil
}

var _ domain.PurchaseVerifier = (*PurchaseVerifier)(nil)
