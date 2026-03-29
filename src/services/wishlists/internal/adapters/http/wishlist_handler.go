package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/wishlists/internal/application/commands"
	"github.com/osmanozen/oo-commerce/src/services/wishlists/internal/application/queries"
)

type WishlistHandler struct {
	getWishlist   *queries.GetUserWishlistHandler
	getCount      *queries.GetWishlistCountHandler
	getProductIDs *queries.GetWishlistProductIDsHandler
	addToWishlist *commands.AddToWishlistHandler
	removeItem    *commands.RemoveFromWishlistHandler
}

func NewWishlistHandler(
	getWishlist *queries.GetUserWishlistHandler,
	getCount *queries.GetWishlistCountHandler,
	getProductIDs *queries.GetWishlistProductIDsHandler,
	addToWishlist *commands.AddToWishlistHandler,
	removeItem *commands.RemoveFromWishlistHandler,
) *WishlistHandler {
	return &WishlistHandler{
		getWishlist:   getWishlist,
		getCount:      getCount,
		getProductIDs: getProductIDs,
		addToWishlist: addToWishlist,
		removeItem:    removeItem,
	}
}

func (h *WishlistHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/wishlist", func(r chi.Router) {
		r.Get("/", h.HandleGetWishlist)
		r.Get("/count", h.HandleGetCount)
		r.Get("/product-ids", h.HandleGetProductIDs)
		r.Post("/{productId}", h.HandleAddToWishlist)
		r.Delete("/{productId}", h.HandleRemoveFromWishlist)
	})
}

func (h *WishlistHandler) HandleGetWishlist(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	result, err := h.getWishlist.Handle(r.Context(), queries.GetUserWishlistQuery{UserID: userID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *WishlistHandler) HandleGetCount(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	count, err := h.getCount.Handle(r.Context(), queries.GetWishlistCountQuery{UserID: userID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, count)
}

func (h *WishlistHandler) HandleGetProductIDs(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	result, err := h.getProductIDs.Handle(r.Context(), queries.GetWishlistProductIDsQuery{UserID: userID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *WishlistHandler) HandleAddToWishlist(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	productID := chi.URLParam(r, "productId")
	result, err := h.addToWishlist.Handle(r.Context(), commands.AddToWishlistCommand{
		UserID:    userID,
		ProductID: productID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *WishlistHandler) HandleRemoveFromWishlist(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	productID := chi.URLParam(r, "productId")
	_, err := h.removeItem.Handle(r.Context(), commands.RemoveFromWishlistCommand{
		UserID:    userID,
		ProductID: productID,
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
