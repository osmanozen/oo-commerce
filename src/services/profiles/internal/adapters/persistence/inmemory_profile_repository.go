package persistence

import (
	"context"
	"sync"

	"github.com/osmanozen/oo-commerce/src/services/profiles/internal/domain"
)

type InMemoryProfileRepository struct {
	mu     sync.RWMutex
	byUser map[string]*domain.UserProfile
}

func NewInMemoryProfileRepository() *InMemoryProfileRepository {
	return &InMemoryProfileRepository{
		byUser: make(map[string]*domain.UserProfile),
	}
}

func (r *InMemoryProfileRepository) Create(_ context.Context, profile *domain.UserProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byUser[profile.UserID] = cloneProfile(profile)
	return nil
}

func (r *InMemoryProfileRepository) GetByUserID(_ context.Context, userID string) (*domain.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byUser[userID]
	if !ok {
		return nil, nil
	}
	return cloneProfile(p), nil
}

func (r *InMemoryProfileRepository) Update(_ context.Context, profile *domain.UserProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byUser[profile.UserID] = cloneProfile(profile)
	return nil
}

func cloneProfile(src *domain.UserProfile) *domain.UserProfile {
	if src == nil {
		return nil
	}
	cp := *src
	cp.Addresses = append([]domain.Address(nil), src.Addresses...)
	if src.AvatarURL != nil {
		u := *src.AvatarURL
		cp.AvatarURL = &u
	}
	return &cp
}

var _ domain.ProfileRepository = (*InMemoryProfileRepository)(nil)
