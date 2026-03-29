package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/ordering/internal/application/commands"
	"github.com/osmanozen/oo-commerce/src/services/ordering/internal/application/queries"
)

type OrderingHandler struct {
	checkout        *commands.CheckoutHandler
	cancelOrder     *commands.CancelOrderHandler
	simulatePayment *commands.SimulatePaymentHandler
	getOrderByID    *queries.GetOrderByIDHandler
	getUserOrders   *queries.GetUserOrdersHandler
	logger          *slog.Logger
}

func NewOrderingHandler(
	checkout *commands.CheckoutHandler,
	cancelOrder *commands.CancelOrderHandler,
	simulatePayment *commands.SimulatePaymentHandler,
	getOrderByID *queries.GetOrderByIDHandler,
	getUserOrders *queries.GetUserOrdersHandler,
	logger *slog.Logger,
) *OrderingHandler {
	return &OrderingHandler{
		checkout:        checkout,
		cancelOrder:     cancelOrder,
		simulatePayment: simulatePayment,
		getOrderByID:    getOrderByID,
		getUserOrders:   getUserOrders,
		logger:          logger,
	}
}

func (h *OrderingHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/ordering", func(r chi.Router) {
		r.Post("/checkout", h.HandleCheckout)

		r.Route("/orders", func(r chi.Router) {
			r.Get("/", h.HandleGetUserOrders)
			r.Get("/{orderId}", h.HandleGetOrderByID)
			r.Post("/{orderId}/cancel", h.HandleCancelOrder)
			r.Post("/{orderId}/pay", h.HandleSimulatePayment)
		})
	})
}

func (h *OrderingHandler) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.NewDomainError(bberrors.ErrUnauthorized, "authentication required"))
		return
	}

	var cmd commands.CheckoutCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.UserID = userID

	result, err := h.checkout.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *OrderingHandler) HandleGetOrderByID(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.NewDomainError(bberrors.ErrUnauthorized, "authentication required"))
		return
	}

	query := queries.GetOrderByIDQuery{
		UserID:  userID,
		OrderID: chi.URLParam(r, "orderId"),
	}
	result, err := h.getOrderByID.Handle(r.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *OrderingHandler) HandleGetUserOrders(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.NewDomainError(bberrors.ErrUnauthorized, "authentication required"))
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	result, err := h.getUserOrders.Handle(r.Context(), queries.GetUserOrdersQuery{
		UserID:   userID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *OrderingHandler) HandleCancelOrder(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.NewDomainError(bberrors.ErrUnauthorized, "authentication required"))
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, bberrors.ValidationError("invalid request body"))
			return
		}
	}

	_, err := h.cancelOrder.Handle(r.Context(), commands.CancelOrderCommand{
		UserID:  userID,
		OrderID: chi.URLParam(r, "orderId"),
		Reason:  body.Reason,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OrderingHandler) HandleSimulatePayment(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("APP_ENV") == "production" {
		writeError(w, bberrors.NewDomainError(bberrors.ErrForbidden, "payment simulation endpoint disabled in production"))
		return
	}

	var cmd commands.SimulatePaymentCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.OrderID = chi.URLParam(r, "orderId")

	_, err := h.simulatePayment.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
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
