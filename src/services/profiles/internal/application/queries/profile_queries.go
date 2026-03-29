package queries

import (
	"context"
	"fmt"
	"strings"

	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/profiles/internal/domain"
)

type GetProfileQuery struct {
	UserID string
}

func (q GetProfileQuery) QueryName() string { return "GetProfileQuery" }

type ProfileDTO struct {
	ID          string       `json:"id"`
	UserID      string       `json:"userId"`
	DisplayName string       `json:"displayName"`
	AvatarURL   *string      `json:"avatarUrl"`
	Addresses   []AddressDTO `json:"addresses"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   string       `json:"updatedAt"`
}

type AddressDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Street    string `json:"street"`
	City      string `json:"city"`
	State     string `json:"state"`
	ZipCode   string `json:"zipCode"`
	Country   string `json:"country"`
	IsDefault bool   `json:"isDefault"`
}

type GetProfileHandler struct {
	profiles domain.ProfileRepository
}

func NewGetProfileHandler(profiles domain.ProfileRepository) *GetProfileHandler {
	return &GetProfileHandler{profiles: profiles}
}

func (h *GetProfileHandler) Handle(ctx context.Context, query GetProfileQuery) (*ProfileDTO, error) {
	userID := strings.TrimSpace(query.UserID)
	if userID == "" {
		return nil, bberrors.ValidationError("user id is required")
	}

	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting profile: %w", err)
	}

	// Auto-create profile on first access.
	if profile == nil {
		profile, err = domain.NewUserProfile(userID, fmt.Sprintf("%s@example.local", userID), "User", "")
		if err != nil {
			return nil, bberrors.ValidationError(err.Error())
		}
		if err := h.profiles.Create(ctx, profile); err != nil {
			// Graceful handling for potential concurrent create.
			existing, getErr := h.profiles.GetByUserID(ctx, userID)
			if getErr != nil {
				return nil, fmt.Errorf("creating profile: %w", err)
			}
			if existing == nil {
				return nil, fmt.Errorf("creating profile: %w", err)
			}
			profile = existing
		}
	}

	addresses := make([]AddressDTO, 0, len(profile.Addresses))
	for _, a := range profile.Addresses {
		addresses = append(addresses, AddressDTO{
			ID:        a.ID.String(),
			Name:      a.Label,
			Street:    a.Street,
			City:      a.City,
			State:     a.State,
			ZipCode:   a.ZipCode,
			Country:   a.Country,
			IsDefault: a.IsDefault,
		})
	}

	return &ProfileDTO{
		ID:          profile.ID.String(),
		UserID:      profile.UserID,
		DisplayName: profile.DisplayName(),
		AvatarURL:   profile.AvatarURL,
		Addresses:   addresses,
		CreatedAt:   profile.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   profile.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

var _ cqrs.QueryHandler[GetProfileQuery, *ProfileDTO] = (*GetProfileHandler)(nil)
