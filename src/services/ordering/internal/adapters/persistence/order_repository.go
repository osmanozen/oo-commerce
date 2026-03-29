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
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/persistence"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/domain"
	"github.com/shopspring/decimal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewOrderRepository(pool *pgxpool.Pool, logger *slog.Logger) *OrderRepository {
	return &OrderRepository{
		pool:   pool,
		logger: logger,
	}
}

func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	if err := order.Validate(); err != nil {
		return bberrors.ValidationError(err.Error())
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO ordering.orders (
			id, order_number, buyer_id, status, payment_method,
			shipping_first_name, shipping_last_name, shipping_street, shipping_city, shipping_state, shipping_zip_code, shipping_country, shipping_phone,
			billing_first_name, billing_last_name, billing_street, billing_city, billing_state, billing_zip_code, billing_country, billing_phone,
			subtotal_amount, subtotal_currency, tax_amount, tax_currency, total_amount, total_currency,
			placed_at, paid_at, confirmed_at, shipped_at, delivered_at, cancelled_at, cancel_reason,
			created_at, updated_at, version
		)
		VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,$9,$10,$11,$12,$13,
			$14,$15,$16,$17,$18,$19,$20,$21,
			$22,$23,$24,$25,$26,$27,
			$28,$29,$30,$31,$32,$33,$34,
			$35,$36,$37
		)
	`,
		order.ID.Value(),
		order.OrderNumber,
		order.BuyerID,
		int(order.Status),
		int(order.PaymentMethod),
		order.ShippingAddress.FirstName,
		order.ShippingAddress.LastName,
		order.ShippingAddress.Street,
		order.ShippingAddress.City,
		order.ShippingAddress.State,
		order.ShippingAddress.ZipCode,
		order.ShippingAddress.Country,
		order.ShippingAddress.Phone,
		order.BillingAddress.FirstName,
		order.BillingAddress.LastName,
		order.BillingAddress.Street,
		order.BillingAddress.City,
		order.BillingAddress.State,
		order.BillingAddress.ZipCode,
		order.BillingAddress.Country,
		order.BillingAddress.Phone,
		order.SubTotal.Amount,
		order.SubTotal.Currency,
		order.Tax.Amount,
		order.Tax.Currency,
		order.Total.Amount,
		order.Total.Currency,
		order.PlacedAt,
		order.PaidAt,
		order.ConfirmedAt,
		order.ShippedAt,
		order.DeliveredAt,
		order.CancelledAt,
		nullableString(order.CancelReason),
		order.CreatedAt,
		order.UpdatedAt,
		order.Version,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	if err := r.replaceItems(ctx, tx, order); err != nil {
		return err
	}
	if err := r.insertOutboxMessages(ctx, tx, order.GetDomainEvents(), nil); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	order.ClearDomainEvents()
	return nil
}

func (r *OrderRepository) Update(ctx context.Context, order *domain.Order) error {
	if err := order.Validate(); err != nil {
		return bberrors.ValidationError(err.Error())
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	previousVersion := order.Version
	if previousVersion > 0 {
		previousVersion--
	}

	tag, err := tx.Exec(ctx, `
		UPDATE ordering.orders
		SET
			status = $2,
			payment_method = $3,
			shipping_first_name = $4,
			shipping_last_name = $5,
			shipping_street = $6,
			shipping_city = $7,
			shipping_state = $8,
			shipping_zip_code = $9,
			shipping_country = $10,
			shipping_phone = $11,
			billing_first_name = $12,
			billing_last_name = $13,
			billing_street = $14,
			billing_city = $15,
			billing_state = $16,
			billing_zip_code = $17,
			billing_country = $18,
			billing_phone = $19,
			subtotal_amount = $20,
			subtotal_currency = $21,
			tax_amount = $22,
			tax_currency = $23,
			total_amount = $24,
			total_currency = $25,
			placed_at = $26,
			paid_at = $27,
			confirmed_at = $28,
			shipped_at = $29,
			delivered_at = $30,
			cancelled_at = $31,
			cancel_reason = $32,
			updated_at = $33,
			version = $34
		WHERE id = $1 AND version = $35
	`,
		order.ID.Value(),
		int(order.Status),
		int(order.PaymentMethod),
		order.ShippingAddress.FirstName,
		order.ShippingAddress.LastName,
		order.ShippingAddress.Street,
		order.ShippingAddress.City,
		order.ShippingAddress.State,
		order.ShippingAddress.ZipCode,
		order.ShippingAddress.Country,
		order.ShippingAddress.Phone,
		order.BillingAddress.FirstName,
		order.BillingAddress.LastName,
		order.BillingAddress.Street,
		order.BillingAddress.City,
		order.BillingAddress.State,
		order.BillingAddress.ZipCode,
		order.BillingAddress.Country,
		order.BillingAddress.Phone,
		order.SubTotal.Amount,
		order.SubTotal.Currency,
		order.Tax.Amount,
		order.Tax.Currency,
		order.Total.Amount,
		order.Total.Currency,
		order.PlacedAt,
		order.PaidAt,
		order.ConfirmedAt,
		order.ShippedAt,
		order.DeliveredAt,
		order.CancelledAt,
		nullableString(order.CancelReason),
		order.UpdatedAt,
		order.Version,
		previousVersion,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NewDomainError(bberrors.ErrConcurrencyConflict, "order was updated by another process")
	}

	if err := r.replaceItems(ctx, tx, order); err != nil {
		return err
	}
	if err := r.insertOutboxMessages(ctx, tx, order.GetDomainEvents(), nil); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	order.ClearDomainEvents()
	return nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id domain.OrderID) (*domain.Order, error) {
	return r.getBy(ctx, "id = $1", id.Value())
}

func (r *OrderRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (*domain.Order, error) {
	return r.getBy(ctx, "order_number = $1", orderNumber)
}

func (r *OrderRepository) getBy(ctx context.Context, whereClause string, arg interface{}) (*domain.Order, error) {
	query := fmt.Sprintf(`
		SELECT
			id, order_number, buyer_id, status, payment_method,
			shipping_first_name, shipping_last_name, shipping_street, shipping_city, shipping_state, shipping_zip_code, shipping_country, shipping_phone,
			billing_first_name, billing_last_name, billing_street, billing_city, billing_state, billing_zip_code, billing_country, billing_phone,
			subtotal_amount, subtotal_currency, tax_amount, tax_currency, total_amount, total_currency,
			placed_at, paid_at, confirmed_at, shipped_at, delivered_at, cancelled_at, cancel_reason,
			created_at, updated_at, version
		FROM ordering.orders
		WHERE %s
	`, whereClause)

	var (
		dbID            uuid.UUID
		orderNumber     string
		buyerID         string
		status          int
		paymentMethod   int
		shipping        domain.OrderAddress
		billing         domain.OrderAddress
		subtotalAmount  decimal.Decimal
		subtotalCur     string
		taxAmount       decimal.Decimal
		taxCur          string
		totalAmount     decimal.Decimal
		totalCur        string
		placedAt        *time.Time
		paidAt          *time.Time
		confirmedAt     *time.Time
		shippedAt       *time.Time
		deliveredAt     *time.Time
		cancelledAt     *time.Time
		cancelReason    *string
		createdAt       time.Time
		updatedAt       time.Time
		version         int
	)

	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&dbID, &orderNumber, &buyerID, &status, &paymentMethod,
		&shipping.FirstName, &shipping.LastName, &shipping.Street, &shipping.City, &shipping.State, &shipping.ZipCode, &shipping.Country, &shipping.Phone,
		&billing.FirstName, &billing.LastName, &billing.Street, &billing.City, &billing.State, &billing.ZipCode, &billing.Country, &billing.Phone,
		&subtotalAmount, &subtotalCur, &taxAmount, &taxCur, &totalAmount, &totalCur,
		&placedAt, &paidAt, &confirmedAt, &shippedAt, &deliveredAt, &cancelledAt, &cancelReason,
		&createdAt, &updatedAt, &version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select order: %w", err)
	}

	orderID, err := domain.OrderIDFromString(dbID.String())
	if err != nil {
		return nil, fmt.Errorf("parse order id: %w", err)
	}
	items, err := r.loadItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	order := &domain.Order{
		ID:              orderID,
		OrderNumber:     orderNumber,
		BuyerID:         buyerID,
		Status:          domain.OrderStatus(status),
		ShippingAddress: shipping,
		BillingAddress:  billing,
		Items:           items,
		SubTotal: types.Money{
			Amount:   subtotalAmount,
			Currency: subtotalCur,
		},
		Tax: types.Money{
			Amount:   taxAmount,
			Currency: taxCur,
		},
		Total: types.Money{
			Amount:   totalAmount,
			Currency: totalCur,
		},
		PaymentMethod: domain.PaymentMethod(paymentMethod),
		PlacedAt:      placedAt,
		PaidAt:        paidAt,
		ConfirmedAt:   confirmedAt,
		ShippedAt:     shippedAt,
		DeliveredAt:   deliveredAt,
		CancelledAt:   cancelledAt,
	}
	if cancelReason != nil {
		order.CancelReason = *cancelReason
	}
	order.CreatedAt = createdAt
	order.UpdatedAt = updatedAt
	order.Version = version
	order.ClearDomainEvents()
	return order, nil
}

func (r *OrderRepository) GetByBuyerID(
	ctx context.Context,
	buyerID string,
	params persistence.PaginationParams,
) (*persistence.PagedResult[domain.OrderSummaryDTO], error) {
	var totalCount int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM ordering.orders
		WHERE buyer_id = $1
	`, buyerID).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("count buyer orders: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			id, order_number, status, total_amount, total_currency,
			(SELECT COUNT(1) FROM ordering.order_items oi WHERE oi.order_id = o.id) AS item_count,
			placed_at
		FROM ordering.orders o
		WHERE buyer_id = $1
		ORDER BY placed_at DESC
		LIMIT $2 OFFSET $3
	`, buyerID, params.Limit(), params.Offset())
	if err != nil {
		return nil, fmt.Errorf("query buyer orders: %w", err)
	}
	defer rows.Close()

	items := make([]domain.OrderSummaryDTO, 0)
	for rows.Next() {
		var (
			dto      domain.OrderSummaryDTO
			id       uuid.UUID
			status   int
			total    decimal.Decimal
			placedAt *time.Time
		)
		if err := rows.Scan(
			&id,
			&dto.OrderNumber,
			&status,
			&total,
			&dto.Currency,
			&dto.ItemCount,
			&placedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order summary: %w", err)
		}
		dto.ID = id.String()
		dto.Status = domain.OrderStatus(status).String()
		dto.Total = total.StringFixed(2)
		if placedAt != nil {
			dto.PlacedAt = placedAt.UTC().Format(time.RFC3339)
		}
		items = append(items, dto)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate buyer orders: %w", err)
	}

	result := persistence.NewPagedResult(items, totalCount, params)
	return &result, nil
}

func (r *OrderRepository) loadItems(ctx context.Context, orderID domain.OrderID) ([]domain.OrderItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, order_id, product_id, product_name,
			price_amount, price_currency,
			quantity, line_total_amount, line_total_currency
		FROM ordering.order_items
		WHERE order_id = $1
		ORDER BY id
	`, orderID.Value())
	if err != nil {
		return nil, fmt.Errorf("query order items: %w", err)
	}
	defer rows.Close()

	items := []domain.OrderItem{}
	for rows.Next() {
		var (
			item             domain.OrderItem
			itemID           uuid.UUID
			orderIDRaw       uuid.UUID
			priceAmount      decimal.Decimal
			priceCurrency    string
			lineTotalAmount  decimal.Decimal
			lineTotalCurency string
		)
		if err := rows.Scan(
			&itemID,
			&orderIDRaw,
			&item.ProductID,
			&item.ProductName,
			&priceAmount,
			&priceCurrency,
			&item.Quantity,
			&lineTotalAmount,
			&lineTotalCurency,
		); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}

		typedItemID, err := domain.OrderItemIDFromString(itemID.String())
		if err != nil {
			return nil, fmt.Errorf("parse order item id: %w", err)
		}
		typedOrderID, err := domain.OrderIDFromString(orderIDRaw.String())
		if err != nil {
			return nil, fmt.Errorf("parse order id in item: %w", err)
		}
		item.ID = typedItemID
		item.OrderID = typedOrderID
		item.Price = types.Money{
			Amount:   priceAmount,
			Currency: priceCurrency,
		}
		item.LineTotal = types.Money{
			Amount:   lineTotalAmount,
			Currency: lineTotalCurency,
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}

	return items, nil
}

func (r *OrderRepository) replaceItems(ctx context.Context, tx pgx.Tx, order *domain.Order) error {
	if _, err := tx.Exec(ctx, `DELETE FROM ordering.order_items WHERE order_id = $1`, order.ID.Value()); err != nil {
		return fmt.Errorf("delete existing order items: %w", err)
	}

	for _, item := range order.Items {
		_, err := tx.Exec(ctx, `
			INSERT INTO ordering.order_items (
				id, order_id, product_id, product_name,
				price_amount, price_currency, quantity,
				line_total_amount, line_total_currency
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`,
			item.ID.Value(),
			order.ID.Value(),
			item.ProductID,
			item.ProductName,
			item.Price.Amount,
			item.Price.Currency,
			item.Quantity,
			item.LineTotal.Amount,
			item.LineTotal.Currency,
		)
		if err != nil {
			return fmt.Errorf("insert order item %s: %w", item.ID.String(), err)
		}
	}

	return nil
}

func (r *OrderRepository) insertOutboxMessages(
	ctx context.Context,
	tx pgx.Tx,
	events []bbdomain.DomainEvent,
	correlationID *uuid.UUID,
) error {
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal domain event: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO ordering.outbox_messages (message_id, message_type, payload, correlation_id, created_at, retry_count)
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

func nullableString(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

var _ domain.OrderRepository = (*OrderRepository)(nil)
