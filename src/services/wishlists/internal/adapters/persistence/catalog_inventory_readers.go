package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/osmanozen/oo-commerce/services/wishlists/internal/application/queries"
)

type CatalogReader struct {
	pool *pgxpool.Pool
}

func NewCatalogReader(pool *pgxpool.Pool) *CatalogReader {
	return &CatalogReader{pool: pool}
}

func (r *CatalogReader) GetProductsByIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]queries.CatalogProduct, error) {
	if len(productIDs) == 0 {
		return map[uuid.UUID]queries.CatalogProduct{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, name_value, price_amount, price_currency, image_url, average_rating, review_count
		FROM catalog.products
		WHERE id = ANY($1::uuid[])
	`, productIDs)
	if err != nil {
		return nil, fmt.Errorf("query catalog products: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]queries.CatalogProduct, len(productIDs))
	for rows.Next() {
		var (
			id            uuid.UUID
			name          string
			priceAmount   decimal.Decimal
			priceCurrency string
			imageURL      *string
			averageRating *decimal.Decimal
			reviewCount   int
		)
		if err := rows.Scan(&id, &name, &priceAmount, &priceCurrency, &imageURL, &averageRating, &reviewCount); err != nil {
			return nil, fmt.Errorf("scan catalog product row: %w", err)
		}
		result[id] = queries.CatalogProduct{
			ID:            id,
			Name:          name,
			PriceAmount:   priceAmount,
			PriceCurrency: priceCurrency,
			ImageURL:      imageURL,
			AverageRating: averageRating,
			ReviewCount:   reviewCount,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate catalog rows: %w", err)
	}
	return result, nil
}

type InventoryReader struct {
	pool *pgxpool.Pool
}

func NewInventoryReader(pool *pgxpool.Pool) *InventoryReader {
	return &InventoryReader{pool: pool}
}

func (r *InventoryReader) GetStockByProductIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]queries.StockInfo, error) {
	if len(productIDs) == 0 {
		return map[uuid.UUID]queries.StockInfo{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			si.product_id,
			si.total_quantity,
			COALESCE((
				SELECT SUM(sr.quantity)
				FROM inventory.stock_reservations sr
				WHERE sr.stock_item_id = si.id
					AND sr.is_committed = false
					AND sr.is_released = false
					AND sr.expires_at > now()
			), 0) AS reserved_quantity
		FROM inventory.stock_items si
		WHERE si.product_id = ANY($1::uuid[])
	`, productIDs)
	if err != nil {
		return nil, fmt.Errorf("query inventory stock: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]queries.StockInfo, len(productIDs))
	for rows.Next() {
		var (
			productID        uuid.UUID
			totalQuantity    int
			reservedQuantity int
		)
		if err := rows.Scan(&productID, &totalQuantity, &reservedQuantity); err != nil {
			return nil, fmt.Errorf("scan inventory stock row: %w", err)
		}
		result[productID] = queries.StockInfo{
			ProductID:        productID,
			TotalQuantity:    totalQuantity,
			ReservedQuantity: reservedQuantity,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inventory stock rows: %w", err)
	}
	return result, nil
}
