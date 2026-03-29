package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/catalog/internal/application/commands"
	"github.com/osmanozen/oo-commerce/services/catalog/internal/application/queries"
)

// CategoryHandler handles HTTP requests for category endpoints.
type CategoryHandler struct {
	createCategory   *commands.CreateCategoryHandler
	updateCategory   *commands.UpdateCategoryHandler
	deleteCategory   *commands.DeleteCategoryHandler
	getCategories    *queries.GetCategoriesHandler
	getCategoryByID  *queries.GetCategoryByIdHandler
	logger           *slog.Logger
}

// NewCategoryHandler creates category HTTP handlers with injected dependencies.
func NewCategoryHandler(
	createCategory *commands.CreateCategoryHandler,
	updateCategory *commands.UpdateCategoryHandler,
	deleteCategory *commands.DeleteCategoryHandler,
	getCategories *queries.GetCategoriesHandler,
	getCategoryByID *queries.GetCategoryByIdHandler,
	logger *slog.Logger,
) *CategoryHandler {
	return &CategoryHandler{
		createCategory:  createCategory,
		updateCategory:  updateCategory,
		deleteCategory:  deleteCategory,
		getCategories:   getCategories,
		getCategoryByID: getCategoryByID,
		logger:          logger,
	}
}

// RegisterRoutes mounts category endpoints on the router.
func (h *CategoryHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/catalog/categories", func(r chi.Router) {
		r.Get("/", h.HandleGetCategories)
		r.Post("/", h.HandleCreateCategory)
		r.Get("/{id}", h.HandleGetCategoryByID)
		r.Put("/{id}", h.HandleUpdateCategory)
		r.Delete("/{id}", h.HandleDeleteCategory)
	})
}

// HandleCreateCategory handles POST /api/catalog/categories
func (h *CategoryHandler) HandleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var cmd commands.CreateCategoryCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	result, err := h.createCategory.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// HandleGetCategories handles GET /api/catalog/categories
func (h *CategoryHandler) HandleGetCategories(w http.ResponseWriter, r *http.Request) {
	result, err := h.getCategories.Handle(r.Context(), queries.GetCategoriesQuery{})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleGetCategoryByID handles GET /api/catalog/categories/{id}
func (h *CategoryHandler) HandleGetCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.getCategoryByID.Handle(r.Context(), queries.GetCategoryByIdQuery{ID: id})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleUpdateCategory handles PUT /api/catalog/categories/{id}
func (h *CategoryHandler) HandleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	var cmd commands.UpdateCategoryCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.ID = chi.URLParam(r, "id")

	_, err := h.updateCategory.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteCategory handles DELETE /api/catalog/categories/{id}
func (h *CategoryHandler) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := h.deleteCategory.Handle(r.Context(), commands.DeleteCategoryCommand{ID: id})
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

