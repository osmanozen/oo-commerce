package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/inventory/internal/application/commands"
	"github.com/osmanozen/oo-commerce/services/inventory/internal/application/queries"
)

// StockHandler handles HTTP requests for inventory endpoints.
type StockHandler struct {
	adjustStock    *commands.AdjustStockHandler
	getStock       *queries.GetStockHandler
	getStockLevels *queries.GetStockLevelsHandler
	logger         *slog.Logger
}

func NewStockHandler(
	adjustStock *commands.AdjustStockHandler,
	getStock *queries.GetStockHandler,
	getStockLevels *queries.GetStockLevelsHandler,
	logger *slog.Logger,
) *StockHandler {
	return &StockHandler{
		adjustStock:    adjustStock,
		getStock:       getStock,
		getStockLevels: getStockLevels,
		logger:         logger,
	}
}

func (h *StockHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/inventory", func(r chi.Router) {
		r.Get("/stock/{productId}", h.HandleGetStock)
		r.Post("/stock/{productId}/adjust", h.HandleAdjustStock)
		r.Post("/stock/levels", h.HandleGetStockLevels)
	})
}

func (h *StockHandler) HandleGetStock(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "productId")
	result, err := h.getStock.Handle(r.Context(), queries.GetStockQuery{ProductID: productID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *StockHandler) HandleAdjustStock(w http.ResponseWriter, r *http.Request) {
	var cmd commands.AdjustStockCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.ProductID = chi.URLParam(r, "productId")
	cmd.CreatedBy = extractCreatedBy(r)

	_, err := h.adjustStock.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func extractCreatedBy(r *http.Request) string {
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		return uid
	}
	return "system"
}

func (h *StockHandler) HandleGetStockLevels(w http.ResponseWriter, r *http.Request) {
	var query queries.GetStockLevelsQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	result, err := h.getStockLevels.Handle(r.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

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
