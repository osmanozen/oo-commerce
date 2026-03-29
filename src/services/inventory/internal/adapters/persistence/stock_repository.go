package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/inventory/internal/domain"
)

type StockItemRepository struct {
	pool   *pgxpool.Pool
}

func NewStockItemRepository(pool *pgxpool.Pool, logger *slog.Logger) *StockItemRepository {
	_ = logger
	return &StockItemRepository{pool: pool}
}

func (r *StockItemRepository) Create(ctx context.Context, item *domain.StockItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO inventory.stock_items (
			id, product_id, sku, total_quantity, low_stock_level, created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		item.ID.Value(),
		item.ProductID,
		item.SKU,
		item.TotalQuantity,
		item.LowStockLevel,
		item.CreatedAt,
		item.UpdatedAt,
		item.Version,
	)
	if err != nil {
		return fmt.Errorf("insert stock item: %w", err)
	}

	if err := r.insertOutboxMessages(ctx, tx, item.GetDomainEvents(), nil); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	item.ClearDomainEvents()
	return nil
}

func (r *StockItemRepository) GetByID(ctx context.Context, id domain.StockItemID) (*domain.StockItem, error) {
	return r.getBy(ctx, "id = $1", id.Value())
}

func (r *StockItemRepository) GetByProductID(ctx context.Context, productID uuid.UUID) (*domain.StockItem, error) {
	return r.getBy(ctx, "product_id = $1", productID)
}

func (r *StockItemRepository) getBy(ctx context.Context, whereClause string, whereArg interface{}) (*domain.StockItem, error) {
	var (
		dbID          uuid.UUID
		productID     uuid.UUID
		sku           string
		totalQuantity int
		lowStockLevel int
		createdAt     time.Time
		updatedAt     time.Time
		version       int
	)

	query := fmt.Sprintf(`
		SELECT id, product_id, sku, total_quantity, low_stock_level, created_at, updated_at, version
		FROM inventory.stock_items
		WHERE %s
	`, whereClause)
	err := r.pool.QueryRow(ctx, query, whereArg).Scan(
		&dbID,
		&productID,
		&sku,
		&totalQuantity,
		&lowStockLevel,
		&createdAt,
		&updatedAt,
		&version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select stock item: %w", err)
	}

	typedID, err := domain.StockItemIDFromString(dbID.String())
	if err != nil {
		return nil, fmt.Errorf("parse stock item id: %w", err)
	}

	item := &domain.StockItem{
		ID:            typedID,
		ProductID:     productID,
		SKU:           sku,
		TotalQuantity: totalQuantity,
		LowStockLevel: lowStockLevel,
		Reservations:  make([]domain.StockReservation, 0),
		Adjustments:   make([]domain.StockAdjustment, 0),
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	item.Version = version

	reservations, err := r.getReservationsByStockItemID(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("load reservations: %w", err)
	}
	item.Reservations = reservations

	adjustments, err := r.getAdjustmentsByStockItemID(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("load adjustments: %w", err)
	}
	item.Adjustments = adjustments

	item.ClearDomainEvents()
	return item, nil
}

func (r *StockItemRepository) Update(ctx context.Context, item *domain.StockItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE inventory.stock_items
		SET sku = $2, total_quantity = $3, low_stock_level = $4, updated_at = $5, version = $6
		WHERE id = $1
	`,
		item.ID.Value(),
		item.SKU,
		item.TotalQuantity,
		item.LowStockLevel,
		item.UpdatedAt,
		item.Version,
	)
	if err != nil {
		return fmt.Errorf("update stock item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("stock item", item.ID.String())
	}

	if err := r.replaceReservations(ctx, tx, item); err != nil {
		return err
	}
	if err := r.replaceAdjustments(ctx, tx, item); err != nil {
		return err
	}
	if err := r.insertOutboxMessages(ctx, tx, item.GetDomainEvents(), nil); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	item.ClearDomainEvents()
	return nil
}

func (r *StockItemRepository) GetReservationsByOrderID(ctx context.Context, orderID uuid.UUID) ([]domain.StockReservation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stock_item_id, order_id, correlation_id, quantity, reserved_at, expires_at, is_committed, is_released
		FROM inventory.stock_reservations
		WHERE order_id = $1
		ORDER BY reserved_at ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("query reservations by order id: %w", err)
	}
	defer rows.Close()

	out := make([]domain.StockReservation, 0)
	for rows.Next() {
		res, err := scanReservationRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reservation: %w", err)
		}
		out = append(out, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reservations by order id: %w", err)
	}
	return out, nil
}

func (r *StockItemRepository) GetExpiredReservations(ctx context.Context) ([]domain.StockReservation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stock_item_id, order_id, correlation_id, quantity, reserved_at, expires_at, is_committed, is_released
		FROM inventory.stock_reservations
		WHERE expires_at <= now() AND is_committed = false AND is_released = false
		ORDER BY expires_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query expired reservations: %w", err)
	}
	defer rows.Close()

	out := make([]domain.StockReservation, 0)
	for rows.Next() {
		res, err := scanReservationRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expired reservation: %w", err)
		}
		out = append(out, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired reservations: %w", err)
	}
	return out, nil
}

func (r *StockItemRepository) CleanupExpiredReservations(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE inventory.stock_reservations
		SET is_released = true
		WHERE expires_at <= now() AND is_committed = false AND is_released = false
	`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired reservations: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *StockItemRepository) getReservationsByStockItemID(ctx context.Context, stockItemID domain.StockItemID) ([]domain.StockReservation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stock_item_id, order_id, correlation_id, quantity, reserved_at, expires_at, is_committed, is_released
		FROM inventory.stock_reservations
		WHERE stock_item_id = $1
		ORDER BY reserved_at ASC
	`, stockItemID.Value())
	if err != nil {
		return nil, fmt.Errorf("query reservations by stock item: %w", err)
	}
	defer rows.Close()

	out := make([]domain.StockReservation, 0)
	for rows.Next() {
		res, err := scanReservationRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reservation row: %w", err)
		}
		out = append(out, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reservations: %w", err)
	}

	return out, nil
}

func (r *StockItemRepository) getAdjustmentsByStockItemID(ctx context.Context, stockItemID domain.StockItemID) ([]domain.StockAdjustment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, stock_item_id, adjustment_type, quantity, reason, created_at, created_by
		FROM inventory.stock_adjustments
		WHERE stock_item_id = $1
		ORDER BY created_at DESC
	`, stockItemID.Value())
	if err != nil {
		return nil, fmt.Errorf("query adjustments by stock item: %w", err)
	}
	defer rows.Close()

	out := make([]domain.StockAdjustment, 0)
	for rows.Next() {
		var (
			idRaw, stockItemRaw uuid.UUID
			adjType             int
			quantity            int
			reason              *string
			createdAt           time.Time
			createdBy           string
		)

		if err := rows.Scan(&idRaw, &stockItemRaw, &adjType, &quantity, &reason, &createdAt, &createdBy); err != nil {
			return nil, fmt.Errorf("scan adjustment row: %w", err)
		}

		id, err := domain.AdjustmentIDFromString(idRaw.String())
		if err != nil {
			return nil, fmt.Errorf("parse adjustment id: %w", err)
		}
		stockItemIDTyped, err := domain.StockItemIDFromString(stockItemRaw.String())
		if err != nil {
			return nil, fmt.Errorf("parse stock item id for adjustment: %w", err)
		}

		reasonValue := ""
		if reason != nil {
			reasonValue = *reason
		}

		out = append(out, domain.StockAdjustment{
			ID:          id,
			StockItemID: stockItemIDTyped,
			Type:        domain.AdjustmentType(adjType),
			Quantity:    quantity,
			Reason:      reasonValue,
			CreatedAt:   createdAt,
			CreatedBy:   createdBy,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate adjustments: %w", err)
	}

	return out, nil
}

func (r *StockItemRepository) replaceReservations(ctx context.Context, tx pgx.Tx, item *domain.StockItem) error {
	if _, err := tx.Exec(ctx, `DELETE FROM inventory.stock_reservations WHERE stock_item_id = $1`, item.ID.Value()); err != nil {
		return fmt.Errorf("delete reservations: %w", err)
	}

	for _, res := range item.Reservations {
		_, err := tx.Exec(ctx, `
			INSERT INTO inventory.stock_reservations (
				id, stock_item_id, order_id, correlation_id, quantity, reserved_at, expires_at, is_committed, is_released
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`,
			res.ID.Value(),
			res.StockItemID.Value(),
			res.OrderID,
			res.CorrelationID,
			res.Quantity,
			res.ReservedAt,
			res.ExpiresAt,
			res.IsCommitted,
			res.IsReleased,
		)
		if err != nil {
			return fmt.Errorf("insert reservation: %w", err)
		}
	}

	return nil
}

func (r *StockItemRepository) replaceAdjustments(ctx context.Context, tx pgx.Tx, item *domain.StockItem) error {
	if _, err := tx.Exec(ctx, `DELETE FROM inventory.stock_adjustments WHERE stock_item_id = $1`, item.ID.Value()); err != nil {
		return fmt.Errorf("delete adjustments: %w", err)
	}

	for _, adj := range item.Adjustments {
		_, err := tx.Exec(ctx, `
			INSERT INTO inventory.stock_adjustments (
				id, stock_item_id, adjustment_type, quantity, reason, created_at, created_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7)
		`,
			adj.ID.Value(),
			adj.StockItemID.Value(),
			int(adj.Type),
			adj.Quantity,
			adj.Reason,
			adj.CreatedAt,
			adj.CreatedBy,
		)
		if err != nil {
			return fmt.Errorf("insert adjustment: %w", err)
		}
	}

	return nil
}

func (r *StockItemRepository) insertOutboxMessages(ctx context.Context, tx pgx.Tx, events []bbdomain.DomainEvent, correlationID *uuid.UUID) error {
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO inventory.outbox_messages (message_id, message_type, payload, correlation_id, created_at, retry_count)
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

func scanReservationRow(row pgx.Row) (domain.StockReservation, error) {
	var (
		idRaw, stockItemRaw uuid.UUID
		res                 domain.StockReservation
	)

	if err := row.Scan(
		&idRaw,
		&stockItemRaw,
		&res.OrderID,
		&res.CorrelationID,
		&res.Quantity,
		&res.ReservedAt,
		&res.ExpiresAt,
		&res.IsCommitted,
		&res.IsReleased,
	); err != nil {
		return domain.StockReservation{}, err
	}

	id, err := domain.ReservationIDFromUUID(idRaw)
	if err != nil {
		return domain.StockReservation{}, fmt.Errorf("parse reservation id: %w", err)
	}
	stockItemID, err := domain.StockItemIDFromString(stockItemRaw.String())
	if err != nil {
		return domain.StockReservation{}, fmt.Errorf("parse reservation stock item id: %w", err)
	}

	res.ID = id
	res.StockItemID = stockItemID
	return res, nil
}

var _ domain.StockItemRepository = (*StockItemRepository)(nil)
