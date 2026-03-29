package domain

import "context"

// ProfileRepository defines persistence contract for UserProfile aggregate.
type ProfileRepository interface {
	Create(ctx context.Context, profile *UserProfile) error
	GetByUserID(ctx context.Context, userID string) (*UserProfile, error)
	Update(ctx context.Context, profile *UserProfile) error
}
