package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/osmanozen/oo-commerce/src/services/reviews/internal/domain"
)

type ProfileReader struct {
	pool *pgxpool.Pool
}

func NewProfileReader(pool *pgxpool.Pool) *ProfileReader {
	return &ProfileReader{pool: pool}
}

func (r *ProfileReader) GetDisplayNames(ctx context.Context, userIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT user_id, first_name, last_name
		FROM profiles.user_profiles
		WHERE user_id = ANY($1)
	`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query profiles display names: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			userID    string
			firstName string
			lastName  string
		)
		if err := rows.Scan(&userID, &firstName, &lastName); err != nil {
			return nil, fmt.Errorf("scan display name row: %w", err)
		}
		fullName := strings.TrimSpace(strings.TrimSpace(firstName) + " " + strings.TrimSpace(lastName))
		if fullName == "" {
			fullName = userID
		}
		out[userID] = fullName
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate display name rows: %w", err)
	}

	return out, nil
}

var _ domain.ProfileReader = (*ProfileReader)(nil)
