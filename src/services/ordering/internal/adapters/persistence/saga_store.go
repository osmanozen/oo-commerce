package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/ordering/internal/saga"
)

type SagaStore struct {
	pool *pgxpool.Pool
}

func NewSagaStore(pool *pgxpool.Pool) *SagaStore {
	return &SagaStore{pool: pool}
}

func (s *SagaStore) Save(ctx context.Context, data *saga.CheckoutSagaData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal saga data: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO ordering.checkout_saga_state (
			correlation_id, current_state, order_id, buyer_id, saga_data,
			started_at, completed_at, failed_at, fail_reason, version
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,0)
	`,
		data.CorrelationID,
		data.CurrentState,
		data.OrderID,
		data.BuyerID,
		payload,
		data.StartedAt,
		data.CompletedAt,
		data.FailedAt,
		nullableString(data.FailReason),
	)
	if err != nil {
		return fmt.Errorf("insert saga state: %w", err)
	}

	return nil
}

func (s *SagaStore) GetByCorrelationID(ctx context.Context, id uuid.UUID) (*saga.CheckoutSagaData, error) {
	var payload []byte
	if err := s.pool.QueryRow(ctx, `
		SELECT saga_data
		FROM ordering.checkout_saga_state
		WHERE correlation_id = $1
	`, id).Scan(&payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, bberrors.NotFoundError("checkout_saga_state", id.String())
		}
		return nil, fmt.Errorf("select saga state: %w", err)
	}

	var data saga.CheckoutSagaData
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("unmarshal saga state: %w", err)
	}
	return &data, nil
}

func (s *SagaStore) Update(ctx context.Context, data *saga.CheckoutSagaData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal saga data: %w", err)
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE ordering.checkout_saga_state
		SET
			current_state = $2,
			buyer_id = $3,
			saga_data = $4,
			completed_at = $5,
			failed_at = $6,
			fail_reason = $7,
			version = version + 1
		WHERE correlation_id = $1
	`,
		data.CorrelationID,
		data.CurrentState,
		data.BuyerID,
		payload,
		data.CompletedAt,
		data.FailedAt,
		nullableString(data.FailReason),
	)
	if err != nil {
		return fmt.Errorf("update saga state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return bberrors.NotFoundError("checkout_saga_state", data.CorrelationID.String())
	}
	return nil
}

func (s *SagaStore) GetCorrelationIDByOrderID(ctx context.Context, orderID uuid.UUID) (uuid.UUID, error) {
	var correlationID uuid.UUID
	if err := s.pool.QueryRow(ctx, `
		SELECT correlation_id
		FROM ordering.checkout_saga_state
		WHERE order_id = $1
		ORDER BY started_at DESC
		LIMIT 1
	`, orderID).Scan(&correlationID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, bberrors.NotFoundError("checkout_saga_state", orderID.String())
		}
		return uuid.Nil, fmt.Errorf("select correlation by order id: %w", err)
	}
	return correlationID, nil
}

var _ saga.SagaStore = (*SagaStore)(nil)
