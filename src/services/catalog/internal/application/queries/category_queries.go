package queries

import (
	"context"

	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/catalog/internal/domain"
)

// ─── Get Categories Query ────────────────────────────────────────────────────

type GetCategoriesQuery struct{}

func (q GetCategoriesQuery) QueryName() string { return "GetCategoriesQuery" }

type GetCategoriesHandler struct {
	categories domain.CategoryRepository
}

func NewGetCategoriesHandler(categories domain.CategoryRepository) *GetCategoriesHandler {
	return &GetCategoriesHandler{categories: categories}
}

func (h *GetCategoriesHandler) Handle(ctx context.Context, query GetCategoriesQuery) ([]domain.CategoryDTO, error) {
	return h.categories.GetAll(ctx)
}

// ─── Get Category By ID Query ────────────────────────────────────────────────

type GetCategoryByIdQuery struct {
	ID string `validate:"required,uuid"`
}

func (q GetCategoryByIdQuery) QueryName() string { return "GetCategoryByIdQuery" }

type GetCategoryByIdHandler struct {
	categories domain.CategoryRepository
}

func NewGetCategoryByIdHandler(categories domain.CategoryRepository) *GetCategoryByIdHandler {
	return &GetCategoryByIdHandler{categories: categories}
}

func (h *GetCategoryByIdHandler) Handle(ctx context.Context, query GetCategoryByIdQuery) (*domain.Category, error) {
	catID, err := domain.CategoryIDFromString(query.ID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid category id")
	}

	category, err := h.categories.GetByID(ctx, catID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, bberrors.NotFoundError("category", query.ID)
	}

	return category, nil
}

var (
	_ cqrs.QueryHandler[GetCategoriesQuery, []domain.CategoryDTO] = (*GetCategoriesHandler)(nil)
	_ cqrs.QueryHandler[GetCategoryByIdQuery, *domain.Category]   = (*GetCategoryByIdHandler)(nil)
)
