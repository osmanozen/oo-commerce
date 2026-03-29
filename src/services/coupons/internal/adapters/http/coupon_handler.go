package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/coupons/internal/application/commands"
	"github.com/osmanozen/oo-commerce/src/services/coupons/internal/application/queries"
)

type CouponHandler struct {
	createCoupon       *commands.CreateCouponHandler
	updateCoupon       *commands.UpdateCouponHandler
	toggleCouponStatus *commands.ToggleCouponStatusHandler
	getCoupons         *queries.GetCouponsHandler
	getCouponByID      *queries.GetCouponByIDHandler
	validateCoupon     *queries.ValidateCouponHandler
}

func NewCouponHandler(
	createCoupon *commands.CreateCouponHandler,
	updateCoupon *commands.UpdateCouponHandler,
	toggleCouponStatus *commands.ToggleCouponStatusHandler,
	getCoupons *queries.GetCouponsHandler,
	getCouponByID *queries.GetCouponByIDHandler,
	validateCoupon *queries.ValidateCouponHandler,
) *CouponHandler {
	return &CouponHandler{
		createCoupon:       createCoupon,
		updateCoupon:       updateCoupon,
		toggleCouponStatus: toggleCouponStatus,
		getCoupons:         getCoupons,
		getCouponByID:      getCouponByID,
		validateCoupon:     validateCoupon,
	}
}

func (h *CouponHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/coupons", func(r chi.Router) {
		r.Post("/", h.HandleCreate)
		r.Get("/", h.HandleList)
		r.Post("/validate", h.HandleValidate)
		r.Get("/{id}", h.HandleGetByID)
		r.Put("/{id}", h.HandleUpdate)
		r.Patch("/{id}/status", h.HandleToggleStatus)
	})
}

func (h *CouponHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var cmd commands.CreateCouponCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	result, err := h.createCoupon.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *CouponHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	var isActive *bool
	if raw := r.URL.Query().Get("isActive"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, bberrors.ValidationError("isActive must be a boolean"))
			return
		}
		isActive = &parsed
	}

	result, err := h.getCoupons.Handle(r.Context(), queries.GetCouponsQuery{
		Page:     page,
		PageSize: pageSize,
		IsActive: isActive,
		Search:   r.URL.Query().Get("search"),
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *CouponHandler) HandleGetByID(w http.ResponseWriter, r *http.Request) {
	result, err := h.getCouponByID.Handle(r.Context(), queries.GetCouponByIDQuery{
		ID: chi.URLParam(r, "id"),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if result == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *CouponHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	var cmd commands.UpdateCouponCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.ID = chi.URLParam(r, "id")

	if _, err := h.updateCoupon.Handle(r.Context(), cmd); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CouponHandler) HandleToggleStatus(w http.ResponseWriter, r *http.Request) {
	var cmd commands.ToggleCouponStatusCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.ID = chi.URLParam(r, "id")

	if _, err := h.toggleCouponStatus.Handle(r.Context(), cmd); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CouponHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	var query queries.ValidateCouponQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	result, err := h.validateCoupon.Handle(r.Context(), query)
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
