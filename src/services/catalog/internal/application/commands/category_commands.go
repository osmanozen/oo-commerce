package commands

import (
	"context"

	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/catalog/internal/domain"
)

// ─── Create Category Command ─────────────────────────────────────────────────

type CreateCategoryCommand struct {
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

func (c CreateCategoryCommand) CommandName() string { return "CreateCategoryCommand" }

type CreateCategoryHandler struct {
	categories domain.CategoryRepository
}

func NewCreateCategoryHandler(categories domain.CategoryRepository) *CreateCategoryHandler {
	return &CreateCategoryHandler{categories: categories}
}

func (h *CreateCategoryHandler) Handle(ctx context.Context, cmd CreateCategoryCommand) (string, error) {
	category, err := domain.NewCategory(cmd.Name, cmd.Description)
	if err != nil {
		return "", bberrors.ValidationError(err.Error())
	}

	if err := h.categories.Create(ctx, category); err != nil {
		return "", err
	}

	return category.ID.String(), nil
}

// ─── Update Category Command ─────────────────────────────────────────────────

type UpdateCategoryCommand struct {
	ID          string `json:"-"`
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Description string `json:"description" validate:"max=500"`
}

func (c UpdateCategoryCommand) CommandName() string { return "UpdateCategoryCommand" }

type UpdateCategoryHandler struct {
	categories domain.CategoryRepository
}

func NewUpdateCategoryHandler(categories domain.CategoryRepository) *UpdateCategoryHandler {
	return &UpdateCategoryHandler{categories: categories}
}

func (h *UpdateCategoryHandler) Handle(ctx context.Context, cmd UpdateCategoryCommand) (struct{}, error) {
	catID, err := domain.CategoryIDFromString(cmd.ID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid category id")
	}

	category, err := h.categories.GetByID(ctx, catID)
	if err != nil || category == nil {
		return struct{}{}, bberrors.NotFoundError("category", cmd.ID)
	}

	if err := category.Update(cmd.Name, cmd.Description); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	if err := h.categories.Update(ctx, category); err != nil {
		return struct{}{}, err
	}

	return struct{}{}, nil
}

// ─── Delete Category Command ─────────────────────────────────────────────────

type DeleteCategoryCommand struct {
	ID string `json:"-"`
}

func (c DeleteCategoryCommand) CommandName() string { return "DeleteCategoryCommand" }

type DeleteCategoryHandler struct {
	categories domain.CategoryRepository
}

func NewDeleteCategoryHandler(categories domain.CategoryRepository) *DeleteCategoryHandler {
	return &DeleteCategoryHandler{categories: categories}
}

func (h *DeleteCategoryHandler) Handle(ctx context.Context, cmd DeleteCategoryCommand) (struct{}, error) {
	catID, err := domain.CategoryIDFromString(cmd.ID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid category id")
	}

	exists, err := h.categories.Exists(ctx, catID)
	if err != nil {
		return struct{}{}, err
	}
	if !exists {
		return struct{}{}, bberrors.NotFoundError("category", cmd.ID)
	}

	if err := h.categories.Delete(ctx, catID); err != nil {
		return struct{}{}, err
	}

	return struct{}{}, nil
}

var (
	_ cqrs.CommandHandler[CreateCategoryCommand, string]   = (*CreateCategoryHandler)(nil)
	_ cqrs.CommandHandler[UpdateCategoryCommand, struct{}] = (*UpdateCategoryHandler)(nil)
	_ cqrs.CommandHandler[DeleteCategoryCommand, struct{}] = (*DeleteCategoryHandler)(nil)
)
