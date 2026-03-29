package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/profiles/internal/domain"
)

type UpdateProfileCommand struct {
	UserID      string `json:"-"`
	DisplayName string `json:"displayName"`
}

func (c UpdateProfileCommand) CommandName() string { return "UpdateProfileCommand" }

type UpdateProfileHandler struct {
	profiles domain.ProfileRepository
}

func NewUpdateProfileHandler(profiles domain.ProfileRepository) *UpdateProfileHandler {
	return &UpdateProfileHandler{profiles: profiles}
}

func (h *UpdateProfileHandler) Handle(ctx context.Context, cmd UpdateProfileCommand) (struct{}, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return struct{}{}, bberrors.ValidationError("user id is required")
	}
	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return struct{}{}, fmt.Errorf("getting profile: %w", err)
	}
	if profile == nil {
		return struct{}{}, bberrors.NotFoundError("profile", userID)
	}
	if err := profile.UpdateDisplayName(cmd.DisplayName); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}
	if err := h.profiles.Update(ctx, profile); err != nil {
		return struct{}{}, fmt.Errorf("updating profile: %w", err)
	}
	return struct{}{}, nil
}

type AddAddressCommand struct {
	UserID       string `json:"-"`
	Name         string `json:"name"`
	Street       string `json:"street"`
	City         string `json:"city"`
	State        string `json:"state"`
	ZipCode      string `json:"zipCode"`
	Country      string `json:"country"`
	SetAsDefault bool   `json:"setAsDefault"`
}

func (c AddAddressCommand) CommandName() string { return "AddAddressCommand" }

type AddAddressHandler struct {
	profiles domain.ProfileRepository
}

func NewAddAddressHandler(profiles domain.ProfileRepository) *AddAddressHandler {
	return &AddAddressHandler{profiles: profiles}
}

func (h *AddAddressHandler) Handle(ctx context.Context, cmd AddAddressCommand) (string, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return "", bberrors.ValidationError("user id is required")
	}
	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("getting profile: %w", err)
	}
	if profile == nil {
		return "", bberrors.NotFoundError("profile", userID)
	}
	addr, err := profile.AddAddress(cmd.Name, cmd.Street, cmd.City, cmd.State, cmd.ZipCode, cmd.Country, cmd.SetAsDefault)
	if err != nil {
		return "", bberrors.ValidationError(err.Error())
	}
	if err := h.profiles.Update(ctx, profile); err != nil {
		return "", fmt.Errorf("adding address: %w", err)
	}
	return addr.ID.String(), nil
}

type UpdateAddressCommand struct {
	UserID    string `json:"-"`
	AddressID string `json:"-"`
	Name      string `json:"name"`
	Street    string `json:"street"`
	City      string `json:"city"`
	State     string `json:"state"`
	ZipCode   string `json:"zipCode"`
	Country   string `json:"country"`
}

func (c UpdateAddressCommand) CommandName() string { return "UpdateAddressCommand" }

type UpdateAddressHandler struct {
	profiles domain.ProfileRepository
}

func NewUpdateAddressHandler(profiles domain.ProfileRepository) *UpdateAddressHandler {
	return &UpdateAddressHandler{profiles: profiles}
}

func (h *UpdateAddressHandler) Handle(ctx context.Context, cmd UpdateAddressCommand) (struct{}, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return struct{}{}, bberrors.ValidationError("user id is required")
	}
	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return struct{}{}, fmt.Errorf("getting profile: %w", err)
	}
	if profile == nil {
		return struct{}{}, bberrors.NotFoundError("profile", userID)
	}
	addressID, err := domain.AddressIDFromString(cmd.AddressID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid address id")
	}
	if err := profile.UpdateAddress(addressID, cmd.Name, cmd.Street, cmd.City, cmd.State, cmd.ZipCode, cmd.Country); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}
	if err := h.profiles.Update(ctx, profile); err != nil {
		return struct{}{}, fmt.Errorf("updating address: %w", err)
	}
	return struct{}{}, nil
}

type DeleteAddressCommand struct {
	UserID    string `json:"-"`
	AddressID string `json:"-"`
}

func (c DeleteAddressCommand) CommandName() string { return "DeleteAddressCommand" }

type DeleteAddressHandler struct {
	profiles domain.ProfileRepository
}

func NewDeleteAddressHandler(profiles domain.ProfileRepository) *DeleteAddressHandler {
	return &DeleteAddressHandler{profiles: profiles}
}

func (h *DeleteAddressHandler) Handle(ctx context.Context, cmd DeleteAddressCommand) (struct{}, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return struct{}{}, bberrors.ValidationError("user id is required")
	}
	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return struct{}{}, fmt.Errorf("getting profile: %w", err)
	}
	if profile == nil {
		return struct{}{}, bberrors.NotFoundError("profile", userID)
	}
	addressID, err := domain.AddressIDFromString(cmd.AddressID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid address id")
	}
	if err := profile.DeleteAddress(addressID); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}
	if err := h.profiles.Update(ctx, profile); err != nil {
		return struct{}{}, fmt.Errorf("deleting address: %w", err)
	}
	return struct{}{}, nil
}

type SetDefaultAddressCommand struct {
	UserID    string `json:"-"`
	AddressID string `json:"-"`
}

func (c SetDefaultAddressCommand) CommandName() string { return "SetDefaultAddressCommand" }

type SetDefaultAddressHandler struct {
	profiles domain.ProfileRepository
}

func NewSetDefaultAddressHandler(profiles domain.ProfileRepository) *SetDefaultAddressHandler {
	return &SetDefaultAddressHandler{profiles: profiles}
}

func (h *SetDefaultAddressHandler) Handle(ctx context.Context, cmd SetDefaultAddressCommand) (struct{}, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return struct{}{}, bberrors.ValidationError("user id is required")
	}
	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return struct{}{}, fmt.Errorf("getting profile: %w", err)
	}
	if profile == nil {
		return struct{}{}, bberrors.NotFoundError("profile", userID)
	}
	addressID, err := domain.AddressIDFromString(cmd.AddressID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid address id")
	}
	if err := profile.SetDefaultAddress(addressID); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}
	if err := h.profiles.Update(ctx, profile); err != nil {
		return struct{}{}, fmt.Errorf("setting default address: %w", err)
	}
	return struct{}{}, nil
}

type UploadAvatarCommand struct {
	UserID      string `json:"-"`
	FileName    string `json:"-"`
	ContentType string `json:"-"`
	SizeBytes   int64  `json:"-"`
}

func (c UploadAvatarCommand) CommandName() string { return "UploadAvatarCommand" }

type UploadAvatarResult struct {
	AvatarURL string `json:"avatarUrl"`
}

type UploadAvatarHandler struct {
	profiles domain.ProfileRepository
}

func NewUploadAvatarHandler(profiles domain.ProfileRepository) *UploadAvatarHandler {
	return &UploadAvatarHandler{profiles: profiles}
}

func (h *UploadAvatarHandler) Handle(ctx context.Context, cmd UploadAvatarCommand) (UploadAvatarResult, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return UploadAvatarResult{}, bberrors.ValidationError("user id is required")
	}
	if cmd.SizeBytes <= 0 || cmd.SizeBytes > 5*1024*1024 {
		return UploadAvatarResult{}, bberrors.ValidationError("avatar file size must be <= 5MB")
	}
	ext := strings.ToLower(filepath.Ext(cmd.FileName))
	switch ext {
	case ".jpg", ".jpeg", ".png":
	default:
		return UploadAvatarResult{}, bberrors.ValidationError("avatar format must be jpg/jpeg/png")
	}
	if cmd.ContentType != "" && !strings.HasPrefix(strings.ToLower(cmd.ContentType), "image/") {
		return UploadAvatarResult{}, bberrors.ValidationError("avatar content type must be image/*")
	}

	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return UploadAvatarResult{}, fmt.Errorf("getting profile: %w", err)
	}
	if profile == nil {
		return UploadAvatarResult{}, bberrors.NotFoundError("profile", userID)
	}

	avatarURL := fmt.Sprintf("https://profiles.local/avatars/%s-%d.jpg", userID, time.Now().UTC().Unix())
	if err := profile.SetAvatar(avatarURL); err != nil {
		return UploadAvatarResult{}, bberrors.ValidationError(err.Error())
	}
	if err := h.profiles.Update(ctx, profile); err != nil {
		return UploadAvatarResult{}, fmt.Errorf("updating avatar: %w", err)
	}
	return UploadAvatarResult{AvatarURL: avatarURL}, nil
}

type RemoveAvatarCommand struct {
	UserID string `json:"-"`
}

func (c RemoveAvatarCommand) CommandName() string { return "RemoveAvatarCommand" }

type RemoveAvatarHandler struct {
	profiles domain.ProfileRepository
}

func NewRemoveAvatarHandler(profiles domain.ProfileRepository) *RemoveAvatarHandler {
	return &RemoveAvatarHandler{profiles: profiles}
}

func (h *RemoveAvatarHandler) Handle(ctx context.Context, cmd RemoveAvatarCommand) (struct{}, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return struct{}{}, bberrors.ValidationError("user id is required")
	}
	profile, err := h.profiles.GetByUserID(ctx, userID)
	if err != nil {
		return struct{}{}, fmt.Errorf("getting profile: %w", err)
	}
	if profile == nil {
		return struct{}{}, bberrors.NotFoundError("profile", userID)
	}
	profile.RemoveAvatar()
	if err := h.profiles.Update(ctx, profile); err != nil {
		return struct{}{}, fmt.Errorf("removing avatar: %w", err)
	}
	return struct{}{}, nil
}

var (
	_ cqrs.CommandHandler[UpdateProfileCommand, struct{}]          = (*UpdateProfileHandler)(nil)
	_ cqrs.CommandHandler[AddAddressCommand, string]               = (*AddAddressHandler)(nil)
	_ cqrs.CommandHandler[UpdateAddressCommand, struct{}]          = (*UpdateAddressHandler)(nil)
	_ cqrs.CommandHandler[DeleteAddressCommand, struct{}]          = (*DeleteAddressHandler)(nil)
	_ cqrs.CommandHandler[SetDefaultAddressCommand, struct{}]      = (*SetDefaultAddressHandler)(nil)
	_ cqrs.CommandHandler[UploadAvatarCommand, UploadAvatarResult] = (*UploadAvatarHandler)(nil)
	_ cqrs.CommandHandler[RemoveAvatarCommand, struct{}]           = (*RemoveAvatarHandler)(nil)
)
