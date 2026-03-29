package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/catalog/internal/application/commands"
	"github.com/osmanozen/oo-commerce/src/services/catalog/internal/application/queries"
)

// ProductHandler handles HTTP requests for product endpoints.
type ProductHandler struct {
	createProduct     *commands.CreateProductHandler
	updateProduct     *commands.UpdateProductHandler
	deleteProduct     *commands.DeleteProductHandler
	updateReviewStats *commands.UpdateReviewStatsHandler
	getProducts       *queries.GetProductsHandler
	getProductByID    *queries.GetProductByIDHandler
	logger            *slog.Logger
}

// NewProductHandler creates product HTTP handlers with injected dependencies.
func NewProductHandler(
	createProduct *commands.CreateProductHandler,
	updateProduct *commands.UpdateProductHandler,
	deleteProduct *commands.DeleteProductHandler,
	updateReviewStats *commands.UpdateReviewStatsHandler,
	getProducts *queries.GetProductsHandler,
	getProductByID *queries.GetProductByIDHandler,
	logger *slog.Logger,
) *ProductHandler {
	return &ProductHandler{
		createProduct:     createProduct,
		updateProduct:     updateProduct,
		deleteProduct:     deleteProduct,
		updateReviewStats: updateReviewStats,
		getProducts:       getProducts,
		getProductByID:    getProductByID,
		logger:            logger,
	}
}

// RegisterRoutes mounts product endpoints on the router.
func (h *ProductHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/catalog/products", func(r chi.Router) {
		r.Get("/", h.HandleGetProducts)
		r.Post("/", h.HandleCreateProduct)
		r.Get("/{id}", h.HandleGetProductByID)
		r.Put("/{id}", h.HandleUpdateProduct)
		r.Delete("/{id}", h.HandleDeleteProduct)
		r.Patch("/{id}/review-stats", h.HandleUpdateReviewStats)
	})
}

// HandleCreateProduct handles POST /api/catalog/products
func (h *ProductHandler) HandleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var cmd commands.CreateProductCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	result, err := h.createProduct.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// HandleGetProducts handles GET /api/catalog/products
func (h *ProductHandler) HandleGetProducts(w http.ResponseWriter, r *http.Request) {
	query := queries.GetProductsQuery{
		Page:      parseIntParam(r, "page", 1),
		PageSize:  parseIntParam(r, "pageSize", 12),
		SortBy:    r.URL.Query().Get("sortBy"),
		SortOrder: r.URL.Query().Get("sortOrder"),
	}

	if catID := r.URL.Query().Get("categoryId"); catID != "" {
		query.CategoryID = &catID
	}
	if s := r.URL.Query().Get("search"); s != "" {
		query.Search = &s
	}
	if v := r.URL.Query().Get("minPrice"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			query.MinPrice = &f
		}
	}
	if v := r.URL.Query().Get("maxPrice"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			query.MaxPrice = &f
		}
	}

	result, err := h.getProducts.Handle(r.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleGetProductByID handles GET /api/catalog/products/{id}
func (h *ProductHandler) HandleGetProductByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.getProductByID.Handle(r.Context(), queries.GetProductByIDQuery{ID: id})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleUpdateProduct handles PUT /api/catalog/products/{id}
func (h *ProductHandler) HandleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	var cmd commands.UpdateProductCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.ID = chi.URLParam(r, "id")

	_, err := h.updateProduct.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteProduct handles DELETE /api/catalog/products/{id}
func (h *ProductHandler) HandleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := h.deleteProduct.Handle(r.Context(), commands.DeleteProductCommand{ID: id})
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleUpdateReviewStats handles PATCH /api/catalog/products/{id}/review-stats
func (h *ProductHandler) HandleUpdateReviewStats(w http.ResponseWriter, r *http.Request) {
	var cmd commands.UpdateReviewStatsCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.ProductID = chi.URLParam(r, "id")

	_, err := h.updateReviewStats.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	status := bberrors.MapToHTTPStatus(err)
	resp := bberrors.ToErrorResponse(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func parseIntParam(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}
