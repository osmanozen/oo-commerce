package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/cart/internal/application/commands"
	"github.com/osmanozen/oo-commerce/src/services/cart/internal/application/queries"
)

// CartHandler handles HTTP requests for cart endpoints.
type CartHandler struct {
	addToCart        *commands.AddToCartHandler
	updateQuantity   *commands.UpdateCartItemQuantityHandler
	removeFromCart   *commands.RemoveFromCartHandler
	clearCart        *commands.ClearCartHandler
	mergeCart        *commands.MergeCartHandler
	getCart          *queries.GetCartHandler
	logger           *slog.Logger
}

// NewCartHandler creates cart HTTP handlers with injected dependencies.
func NewCartHandler(
	addToCart *commands.AddToCartHandler,
	updateQuantity *commands.UpdateCartItemQuantityHandler,
	removeFromCart *commands.RemoveFromCartHandler,
	clearCart *commands.ClearCartHandler,
	mergeCart *commands.MergeCartHandler,
	getCart *queries.GetCartHandler,
	logger *slog.Logger,
) *CartHandler {
	return &CartHandler{
		addToCart:      addToCart,
		updateQuantity: updateQuantity,
		removeFromCart: removeFromCart,
		clearCart:      clearCart,
		mergeCart:      mergeCart,
		getCart:        getCart,
		logger:         logger,
	}
}

// RegisterRoutes mounts cart endpoints on the router.
func (h *CartHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cart", func(r chi.Router) {
		r.Get("/", h.HandleGetCart)
		r.Delete("/", h.HandleClearCart)
		r.Post("/merge", h.HandleMergeCart)
		
		r.Route("/items", func(r chi.Router) {
			r.Post("/", h.HandleAddToCart)
			r.Patch("/{itemId}", h.HandleUpdateQuantity)
			r.Delete("/{itemId}", h.HandleRemoveItem)
		})
	})
}

// HandleGetCart handles GET /api/cart
func (h *CartHandler) HandleGetCart(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	guestID := extractGuestID(r)

	// In a real app, middleware would set a guest cookie if both are empty
	if userID == nil && guestID == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	result, err := h.getCart.Handle(r.Context(), queries.GetCartQuery{
		UserID:  userID,
		GuestID: guestID,
	})

	if err != nil {
		writeError(w, err)
		return
	}

	if result == nil || len(result.Items) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleAddToCart handles POST /api/cart/items
func (h *CartHandler) HandleAddToCart(w http.ResponseWriter, r *http.Request) {
	var cmd commands.AddToCartCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	cmd.UserID = extractUserID(r)
	cmd.GuestID = extractGuestID(r)

	if cmd.UserID == nil && cmd.GuestID == nil {
		// Auto-assign guest id when anonymous client does not send one.
		gid := uuid.Must(uuid.NewV7()).String()
		cmd.GuestID = &gid
		http.SetCookie(w, &http.Cookie{
			Name:     "cart_buyer_id",
			Value:    gid,
			Path:     "/",
			MaxAge:   30 * 24 * 3600,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   r.TLS != nil,
			Expires:  time.Now().Add(30 * 24 * time.Hour).UTC(),
		})
	}

	_, err := h.addToCart.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleUpdateQuantity handles PATCH /api/cart/items/{itemId}
func (h *CartHandler) HandleUpdateQuantity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	// Wait, we need the CartID! By right, we should find CartID from current buyer ID.
	// For simplicity in this demo handler: The handler requires Cart ID. We will resolve it via GetCart.
	userID := extractUserID(r)
	guestID := extractGuestID(r)

	result, err := h.getCart.Handle(r.Context(), queries.GetCartQuery{
		UserID:  userID,
		GuestID: guestID,
	})

	if err != nil || result == nil {
		writeError(w, bberrors.NotFoundError("cart", "current"))
		return
	}

	cmd := commands.UpdateCartItemQuantityCommand{
		CartID:   result.ID,
		ItemID:   chi.URLParam(r, "itemId"),
		Quantity: body.Quantity,
	}

	_, err = h.updateQuantity.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleRemoveItem handles DELETE /api/cart/items/{itemId}
func (h *CartHandler) HandleRemoveItem(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	guestID := extractGuestID(r)

	result, err := h.getCart.Handle(r.Context(), queries.GetCartQuery{
		UserID:  userID,
		GuestID: guestID,
	})

	if err != nil || result == nil {
		writeError(w, bberrors.NotFoundError("cart", "current"))
		return
	}

	cmd := commands.RemoveFromCartCommand{
		CartID: result.ID,
		ItemID: chi.URLParam(r, "itemId"),
	}

	_, err = h.removeFromCart.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleClearCart handles DELETE /api/cart
func (h *CartHandler) HandleClearCart(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	guestID := extractGuestID(r)

	result, err := h.getCart.Handle(r.Context(), queries.GetCartQuery{
		UserID:  userID,
		GuestID: guestID,
	})

	if err != nil || result == nil {
		w.WriteHeader(http.StatusNoContent) // nothing to clear
		return
	}

	cmd := commands.ClearCartCommand{
		CartID: result.ID,
	}

	_, err = h.clearCart.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleMergeCart handles POST /api/cart/merge
func (h *CartHandler) HandleMergeCart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GuestID string `json:"guestBuyerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}

	userID := extractUserID(r)
	if userID == nil || *userID == "" {
		writeError(w, bberrors.ValidationError("must be authenticated to merge cart"))
		return
	}

	cmd := commands.MergeCartCommand{
		UserID:  *userID,
		GuestID: body.GuestID,
	}

	_, err := h.mergeCart.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Request Extractors ──────────────────────────────────────────────────────

func extractUserID(r *http.Request) *string {
	uid := r.Header.Get("X-User-ID")
	if uid != "" {
		return &uid
	}
	return nil
}

func extractGuestID(r *http.Request) *string {
	gid := r.Header.Get("X-Guest-ID")
	if gid != "" {
		return &gid
	}
	if cookie, err := r.Cookie("cart_buyer_id"); err == nil && cookie.Value != "" {
		val := cookie.Value
		return &val
	}
	return nil
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
