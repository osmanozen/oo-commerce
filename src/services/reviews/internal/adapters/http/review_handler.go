package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/reviews/internal/application/commands"
	"github.com/osmanozen/oo-commerce/src/services/reviews/internal/application/queries"
)

type ReviewHandler struct {
	getReviewsByProduct      *queries.GetReviewsByProductHandler
	getUserReviewForProduct  *queries.GetUserReviewForProductHandler
	canReview                *queries.CanReviewHandler
	createReview             *commands.CreateReviewHandler
	updateReview             *commands.UpdateReviewHandler
	deleteReview             *commands.DeleteReviewHandler
}

func NewReviewHandler(
	getReviewsByProduct *queries.GetReviewsByProductHandler,
	getUserReviewForProduct *queries.GetUserReviewForProductHandler,
	canReview *queries.CanReviewHandler,
	createReview *commands.CreateReviewHandler,
	updateReview *commands.UpdateReviewHandler,
	deleteReview *commands.DeleteReviewHandler,
) *ReviewHandler {
	return &ReviewHandler{
		getReviewsByProduct:     getReviewsByProduct,
		getUserReviewForProduct: getUserReviewForProduct,
		canReview:               canReview,
		createReview:            createReview,
		updateReview:            updateReview,
		deleteReview:            deleteReview,
	}
}

func (h *ReviewHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/reviews", func(r chi.Router) {
		r.Route("/products/{productId}", func(r chi.Router) {
			r.Get("/", h.HandleGetByProduct)
			r.Get("/mine", h.HandleGetMine)
			r.Get("/can-review", h.HandleCanReview)
			r.Post("/", h.HandleCreate)
		})

		r.Put("/{reviewId}", h.HandleUpdate)
		r.Delete("/{reviewId}", h.HandleDelete)
	})
}

func (h *ReviewHandler) HandleGetByProduct(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	result, err := h.getReviewsByProduct.Handle(r.Context(), queries.GetReviewsByProductQuery{
		ProductID: chi.URLParam(r, "productId"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *ReviewHandler) HandleGetMine(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	result, err := h.getUserReviewForProduct.Handle(r.Context(), queries.GetUserReviewForProductQuery{
		ProductID: chi.URLParam(r, "productId"),
		UserID:    userID,
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

func (h *ReviewHandler) HandleCanReview(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	result, err := h.canReview.Handle(r.Context(), queries.CanReviewQuery{
		ProductID: chi.URLParam(r, "productId"),
		UserID:    userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *ReviewHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	var body struct {
		Rating int    `json:"rating"`
		Text   string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	result, err := h.createReview.Handle(r.Context(), commands.CreateReviewCommand{
		ProductID: chi.URLParam(r, "productId"),
		UserID:    userID,
		Rating:    body.Rating,
		Text:      body.Text,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *ReviewHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	var body struct {
		Rating int    `json:"rating"`
		Text   string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	_, err := h.updateReview.Handle(r.Context(), commands.UpdateReviewCommand{
		ReviewID: chi.URLParam(r, "reviewId"),
		UserID:   userID,
		Rating:   body.Rating,
		Text:     body.Text,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ReviewHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	_, err := h.deleteReview.Handle(r.Context(), commands.DeleteReviewCommand{
		ReviewID: chi.URLParam(r, "reviewId"),
		UserID:   userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func extractUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
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
