package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/cqrs"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/adapters/persistence"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/domain"
	"github.com/osmanozen/oo-commerce/services/ordering/internal/saga"
	"github.com/shopspring/decimal"
)

type CheckoutAddressInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Street    string `json:"street"`
	City      string `json:"city"`
	State     string `json:"state"`
	ZipCode   string `json:"zipCode"`
	Country   string `json:"country"`
	Phone     string `json:"phone"`
}

type CheckoutCommand struct {
	UserID          string               `json:"-"`
	CartID          string               `json:"cartId"`
	ShippingAddress CheckoutAddressInput `json:"shippingAddress"`
	BillingAddress  CheckoutAddressInput `json:"billingAddress"`
	PaymentMethod   string               `json:"paymentMethod"`
}

func (c CheckoutCommand) CommandName() string { return "CheckoutCommand" }

type CheckoutResult struct {
	OrderID     string `json:"orderId"`
	OrderNumber string `json:"orderNumber"`
	Status      string `json:"status"`
	Total       string `json:"total"`
}

type CheckoutHandler struct {
	carts  *persistence.CartReader
	orders domain.OrderRepository
	saga   *saga.CheckoutSaga
}

func NewCheckoutHandler(
	carts *persistence.CartReader,
	orders domain.OrderRepository,
	saga *saga.CheckoutSaga,
) *CheckoutHandler {
	return &CheckoutHandler{
		carts:  carts,
		orders: orders,
		saga:   saga,
	}
}

func (h *CheckoutHandler) Handle(ctx context.Context, cmd CheckoutCommand) (*CheckoutResult, error) {
	if strings.TrimSpace(cmd.UserID) == "" {
		return nil, bberrors.ValidationError("user id is required")
	}

	cartID, err := uuid.Parse(strings.TrimSpace(cmd.CartID))
	if err != nil {
		return nil, bberrors.ValidationError("invalid cart id")
	}

	cart, err := h.carts.GetByID(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("load cart: %w", err)
	}
	if cart == nil {
		return nil, bberrors.NotFoundError("cart", cmd.CartID)
	}
	if cart.UserID == nil || *cart.UserID != cmd.UserID {
		return nil, bberrors.NewDomainError(bberrors.ErrForbidden, "cart does not belong to user")
	}
	if len(cart.Items) == 0 {
		return nil, bberrors.ValidationError("cart is empty")
	}

	shipping, err := domain.NewOrderAddress(
		cmd.ShippingAddress.FirstName,
		cmd.ShippingAddress.LastName,
		cmd.ShippingAddress.Street,
		cmd.ShippingAddress.City,
		cmd.ShippingAddress.State,
		cmd.ShippingAddress.ZipCode,
		cmd.ShippingAddress.Country,
		cmd.ShippingAddress.Phone,
	)
	if err != nil {
		return nil, bberrors.ValidationError(err.Error())
	}
	billing, err := domain.NewOrderAddress(
		cmd.BillingAddress.FirstName,
		cmd.BillingAddress.LastName,
		cmd.BillingAddress.Street,
		cmd.BillingAddress.City,
		cmd.BillingAddress.State,
		cmd.BillingAddress.ZipCode,
		cmd.BillingAddress.Country,
		cmd.BillingAddress.Phone,
	)
	if err != nil {
		return nil, bberrors.ValidationError(err.Error())
	}
	paymentMethod, err := domain.ParsePaymentMethod(cmd.PaymentMethod)
	if err != nil || paymentMethod == domain.PaymentMethodUnknown {
		return nil, bberrors.ValidationError("invalid payment method")
	}

	items := make([]domain.OrderItem, 0, len(cart.Items))
	for _, cartItem := range cart.Items {
		price, moneyErr := domainMoney(cartItem.UnitPrice, cartItem.Currency)
		if moneyErr != nil {
			return nil, bberrors.ValidationError(moneyErr.Error())
		}
		item, newItemErr := domain.NewOrderItem(
			domain.NewOrderID(),
			cartItem.ProductID,
			cartItem.ProductName,
			price,
			cartItem.Quantity,
		)
		if newItemErr != nil {
			return nil, bberrors.ValidationError(newItemErr.Error())
		}
		items = append(items, item)
	}

	order, err := domain.NewOrder(
		cmd.UserID,
		shipping,
		billing,
		items,
		items[0].Price.Currency,
		paymentMethod,
	)
	if err != nil {
		return nil, bberrors.ValidationError(err.Error())
	}

	if err := h.orders.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	sagaData := &saga.CheckoutSagaData{
		CorrelationID: uuid.Must(uuid.NewV7()),
		OrderID:       order.ID.Value(),
		BuyerID:       order.BuyerID,
		Items:         make([]saga.CheckoutItemData, 0, len(order.Items)),
		TotalAmount:   order.Total.Amount,
		Currency:      order.Total.Currency,
	}
	for _, item := range order.Items {
		sagaData.Items = append(sagaData.Items, saga.CheckoutItemData{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price.Amount,
		})
	}
	if err := h.saga.Start(ctx, sagaData); err != nil {
		return nil, fmt.Errorf("start checkout saga: %w", err)
	}

	_ = h.carts.Clear(ctx, cart.ID)

	return &CheckoutResult{
		OrderID:     order.ID.String(),
		OrderNumber: order.OrderNumber,
		Status:      order.Status.String(),
		Total:       order.Total.Amount.StringFixed(2),
	}, nil
}

var _ cqrs.CommandHandler[CheckoutCommand, *CheckoutResult] = (*CheckoutHandler)(nil)

func domainMoney(amount decimal.Decimal, currency string) (types.Money, error) {
	return types.NewMoney(amount, currency)
}
