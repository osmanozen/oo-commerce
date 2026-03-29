package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/shopspring/decimal"
)

// ─── Strongly-Typed IDs ──────────────────────────────────────────────────────

type orderTag struct{}
type orderItemTag struct{}

type OrderID = types.TypedID[orderTag]
type OrderItemID = types.TypedID[orderItemTag]

func NewOrderID() OrderID         { return types.NewTypedID[orderTag]() }
func NewOrderItemID() OrderItemID { return types.NewTypedID[orderItemTag]() }

func OrderIDFromString(s string) (OrderID, error) { return types.TypedIDFromString[orderTag](s) }
func OrderItemIDFromString(s string) (OrderItemID, error) {
	return types.TypedIDFromString[orderItemTag](s)
}

// ─── Order Status Enum ───────────────────────────────────────────────────────

type OrderStatus int

const (
	OrderStatusUnknown    OrderStatus = iota // 0 = invalid/unset
	OrderStatusPending                       // 1 = awaiting checkout
	OrderStatusProcessing                    // 2 = saga in progress
	OrderStatusPaid                          // 3 = payment received
	OrderStatusConfirmed                     // 4 = fully confirmed
	OrderStatusShipped                       // 5 = shipped
	OrderStatusDelivered                     // 6 = delivered
	OrderStatusCancelled                     // 7 = cancelled
	OrderStatusRefunded                      // 8 = refunded
)

var orderStatusNames = map[OrderStatus]string{
	OrderStatusUnknown:    "Unknown",
	OrderStatusPending:    "Pending",
	OrderStatusProcessing: "Processing",
	OrderStatusPaid:       "Paid",
	OrderStatusConfirmed:  "Confirmed",
	OrderStatusShipped:    "Shipped",
	OrderStatusDelivered:  "Delivered",
	OrderStatusCancelled:  "Cancelled",
	OrderStatusRefunded:   "Refunded",
}

func (s OrderStatus) String() string {
	if name, ok := orderStatusNames[s]; ok {
		return name
	}
	return "Unknown"
}

// ─── Payment Method Enum ─────────────────────────────────────────────────────

type PaymentMethod int

const (
	PaymentMethodUnknown PaymentMethod = iota
	PaymentMethodCreditCard
	PaymentMethodDebitCard
	PaymentMethodBankTransfer
	PaymentMethodPayPal
)

var paymentMethodNames = map[PaymentMethod]string{
	PaymentMethodUnknown:      "Unknown",
	PaymentMethodCreditCard:   "CreditCard",
	PaymentMethodDebitCard:    "DebitCard",
	PaymentMethodBankTransfer: "BankTransfer",
	PaymentMethodPayPal:       "PayPal",
}

func (p PaymentMethod) String() string {
	if name, ok := paymentMethodNames[p]; ok {
		return name
	}
	return "Unknown"
}

func ParsePaymentMethod(name string) (PaymentMethod, error) {
	normalized := strings.TrimSpace(name)
	for method, methodName := range paymentMethodNames {
		if strings.EqualFold(methodName, normalized) {
			return method, nil
		}
	}
	return PaymentMethodUnknown, fmt.Errorf("invalid payment method: %q", name)
}

// ─── Order Address Value Object ──────────────────────────────────────────────

type OrderAddress struct {
	FirstName string `json:"firstName" db:"first_name"`
	LastName  string `json:"lastName" db:"last_name"`
	Street    string `json:"street" db:"street"`
	City      string `json:"city" db:"city"`
	State     string `json:"state" db:"state"`
	ZipCode   string `json:"zipCode" db:"zip_code"`
	Country   string `json:"country" db:"country"`
	Phone     string `json:"phone" db:"phone"`
}

func NewOrderAddress(firstName, lastName, street, city, state, zipCode, country, phone string) (OrderAddress, error) {
	zipCode = strings.TrimSpace(zipCode)
	if !isValidZipCode(zipCode) {
		return OrderAddress{}, errors.New("zip code format is invalid")
	}

	phone = strings.TrimSpace(phone)
	if !isValidPhone(phone) {
		return OrderAddress{}, errors.New("phone must be E.164 format")
	}

	if strings.TrimSpace(firstName) == "" {
		return OrderAddress{}, errors.New("first name is required")
	}
	if strings.TrimSpace(lastName) == "" {
		return OrderAddress{}, errors.New("last name is required")
	}
	if strings.TrimSpace(street) == "" {
		return OrderAddress{}, errors.New("street is required")
	}
	if strings.TrimSpace(city) == "" {
		return OrderAddress{}, errors.New("city is required")
	}
	if strings.TrimSpace(country) == "" {
		return OrderAddress{}, errors.New("country is required")
	}
	return OrderAddress{
		FirstName: strings.TrimSpace(firstName),
		LastName:  strings.TrimSpace(lastName),
		Street:    strings.TrimSpace(street),
		City:      strings.TrimSpace(city),
		State:     strings.TrimSpace(state),
		ZipCode:   zipCode,
		Country:   strings.ToUpper(strings.TrimSpace(country)),
		Phone:     phone,
	}, nil
}

func (a OrderAddress) FullName() string {
	return strings.TrimSpace(a.FirstName + " " + a.LastName)
}

func (a OrderAddress) AddressLine() string {
	return strings.TrimSpace(fmt.Sprintf("%s, %s, %s %s", a.Street, a.City, a.State, a.ZipCode))
}

// ─── Order Item (Owned Entity) ───────────────────────────────────────────────

type OrderItem struct {
	ID          OrderItemID `json:"id" db:"id"`
	OrderID     OrderID     `json:"orderId" db:"order_id"`
	ProductID   uuid.UUID   `json:"productId" db:"product_id"`
	ProductName string      `json:"productName" db:"product_name"`
	Price       types.Money `json:"price"`
	Quantity    int         `json:"quantity" db:"quantity"`
	LineTotal   types.Money `json:"lineTotal"`
}

// ─── Order Aggregate Root ────────────────────────────────────────────────────

type Order struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID              OrderID       `json:"id" db:"id"`
	OrderNumber     string        `json:"orderNumber" db:"order_number"`
	BuyerID         string        `json:"buyerId" db:"buyer_id"`
	Status          OrderStatus   `json:"status" db:"status"`
	ShippingAddress OrderAddress  `json:"shippingAddress"`
	BillingAddress  OrderAddress  `json:"billingAddress"`
	Items           []OrderItem   `json:"items"`
	SubTotal        types.Money   `json:"subTotal"`
	Tax             types.Money   `json:"tax"`
	Total           types.Money   `json:"total"`
	PaymentMethod   PaymentMethod `json:"paymentMethod" db:"payment_method"`
	PlacedAt        *time.Time    `json:"placedAt,omitempty" db:"placed_at"`
	PaidAt          *time.Time    `json:"paidAt,omitempty" db:"paid_at"`
	ConfirmedAt     *time.Time    `json:"confirmedAt,omitempty" db:"confirmed_at"`
	ShippedAt       *time.Time    `json:"shippedAt,omitempty" db:"shipped_at"`
	DeliveredAt     *time.Time    `json:"deliveredAt,omitempty" db:"delivered_at"`
	CancelledAt     *time.Time    `json:"cancelledAt,omitempty" db:"cancelled_at"`
	CancelReason    string        `json:"cancelReason,omitempty" db:"cancel_reason"`
}

// NewOrder creates a new Order aggregate from cart checkout data.
func NewOrder(
	buyerID string,
	shippingAddr, billingAddr OrderAddress,
	items []OrderItem,
	currency string,
	paymentMethod PaymentMethod,
) (*Order, error) {
	if len(items) == 0 {
		return nil, errors.New("order must have at least one item")
	}
	if strings.TrimSpace(buyerID) == "" {
		return nil, errors.New("buyer id is required")
	}
	if paymentMethod == PaymentMethodUnknown {
		return nil, errors.New("payment method is required")
	}

	order := &Order{
		ID:              NewOrderID(),
		OrderNumber:     generateOrderNumber(),
		BuyerID:         buyerID,
		Status:          OrderStatusPending,
		ShippingAddress: shippingAddr,
		BillingAddress:  billingAddr,
		Items:           make([]OrderItem, 0, len(items)),
		PaymentMethod:   paymentMethod,
	}
	order.SetCreated()

	for _, item := range items {
		if item.ID.IsZero() {
			item.ID = NewOrderItemID()
		}
		item.OrderID = order.ID
		order.Items = append(order.Items, item)
	}

	// Calculate totals.
	if err := order.recalculateTotals(currency); err != nil {
		return nil, fmt.Errorf("calculating totals: %w", err)
	}
	if err := order.Validate(); err != nil {
		return nil, err
	}

	// Raise domain event.
	now := time.Now().UTC()
	order.PlacedAt = &now
	order.AddDomainEvent(&OrderCreatedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		OrderID:         order.ID.Value(),
		BuyerID:         buyerID,
		Total:           order.Total.Amount,
		Currency:        order.Total.Currency,
	})

	return order, nil
}

// Confirm transitions the order to Confirmed status.
func (o *Order) Confirm() error {
	if o.Status != OrderStatusPaid && o.Status != OrderStatusProcessing {
		return fmt.Errorf("cannot confirm order in %s status", o.Status)
	}
	o.Status = OrderStatusConfirmed
	now := time.Now().UTC()
	o.ConfirmedAt = &now
	o.SetUpdated()
	o.IncrementVersion()

	o.AddDomainEvent(&OrderConfirmedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		OrderID:         o.ID.Value(),
		BuyerID:         o.BuyerID,
	})

	return nil
}

// Cancel cancels the order with a reason.
func (o *Order) Cancel(reason string) error {
	if !o.CanBeCancelled() {
		return fmt.Errorf("order cannot be cancelled in %s status", o.Status.String())
	}

	o.Status = OrderStatusCancelled
	now := time.Now().UTC()
	o.CancelledAt = &now
	o.CancelReason = strings.TrimSpace(reason)
	o.SetUpdated()
	o.IncrementVersion()

	o.AddDomainEvent(&OrderCancelledEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		OrderID:         o.ID.Value(),
		BuyerID:         o.BuyerID,
		Reason:          reason,
	})

	return nil
}

// MarkPaid transitions the order to Paid status.
func (o *Order) MarkPaid() error {
	if o.Status != OrderStatusProcessing && o.Status != OrderStatusPending {
		return fmt.Errorf("cannot mark as paid in %s status", o.Status)
	}
	o.Status = OrderStatusPaid
	now := time.Now().UTC()
	o.PaidAt = &now
	o.SetUpdated()
	o.IncrementVersion()

	o.AddDomainEvent(&OrderPaidEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		OrderID:         o.ID.Value(),
	})

	return nil
}

func (o *Order) MarkAsShipped() error {
	if o.Status != OrderStatusConfirmed {
		return fmt.Errorf("cannot mark as shipped in %s status", o.Status)
	}

	o.Status = OrderStatusShipped
	now := time.Now().UTC()
	o.ShippedAt = &now
	o.SetUpdated()
	o.IncrementVersion()
	return nil
}

func (o *Order) MarkAsDelivered() error {
	if o.Status != OrderStatusShipped {
		return fmt.Errorf("cannot mark as delivered in %s status", o.Status)
	}

	o.Status = OrderStatusDelivered
	now := time.Now().UTC()
	o.DeliveredAt = &now
	o.SetUpdated()
	o.IncrementVersion()
	return nil
}

func (o *Order) CanBeCancelled() bool {
	return o.Status == OrderStatusPending || o.Status == OrderStatusPaid
}

func (o *Order) GetTotalAmount() types.Money {
	return o.Total
}

func (o *Order) Validate() error {
	if o.ID.IsZero() {
		return errors.New("order id cannot be zero")
	}
	if strings.TrimSpace(o.BuyerID) == "" {
		return errors.New("buyer id is required")
	}
	if len(o.Items) == 0 {
		return errors.New("order must have at least one item")
	}
	if o.PaymentMethod == PaymentMethodUnknown {
		return errors.New("payment method is required")
	}
	for _, item := range o.Items {
		if item.Quantity <= 0 {
			return fmt.Errorf("item %s quantity must be positive", item.ID.String())
		}
	}
	if o.Total.Currency == "" {
		return errors.New("order total currency is required")
	}
	return nil
}

func NewOrderItem(
	orderID OrderID,
	productID uuid.UUID,
	productName string,
	price types.Money,
	quantity int,
) (OrderItem, error) {
	if productID == uuid.Nil {
		return OrderItem{}, errors.New("product id is required")
	}
	if strings.TrimSpace(productName) == "" {
		return OrderItem{}, errors.New("product name is required")
	}
	if quantity <= 0 {
		return OrderItem{}, errors.New("quantity must be positive")
	}

	lineTotal := price.Multiply(quantity)
	return OrderItem{
		ID:          NewOrderItemID(),
		OrderID:     orderID,
		ProductID:   productID,
		ProductName: strings.TrimSpace(productName),
		Price:       price,
		Quantity:    quantity,
		LineTotal:   lineTotal,
	}, nil
}

func (o *Order) recalculateTotals(currency string) error {
	subTotal := types.ZeroMoney(currency)
	for _, item := range o.Items {
		lineTotal, err := subTotal.Add(item.LineTotal)
		if err != nil {
			return err
		}
		subTotal = lineTotal
	}
	o.SubTotal = subTotal
	// Simple tax calculation (18% KDV — Turkish VAT).
	taxRate := decimal.NewFromFloat(0.18)
	taxAmount := subTotal.Amount.Mul(taxRate).Round(2)
	o.Tax = types.Money{Amount: taxAmount, Currency: currency}
	total, err := subTotal.Add(o.Tax)
	if err != nil {
		return err
	}
	o.Total = total
	return nil
}

// ─── Domain Events ───────────────────────────────────────────────────────────

type OrderCreatedEvent struct {
	bbdomain.BaseDomainEvent
	OrderID  uuid.UUID       `json:"orderId"`
	BuyerID  string          `json:"buyerId"`
	Total    decimal.Decimal `json:"total"`
	Currency string          `json:"currency"`
}

func (e *OrderCreatedEvent) EventType() string { return "ordering.order.created" }

type OrderPaidEvent struct {
	bbdomain.BaseDomainEvent
	OrderID uuid.UUID `json:"orderId"`
}

func (e *OrderPaidEvent) EventType() string { return "ordering.order.paid" }

type OrderConfirmedEvent struct {
	bbdomain.BaseDomainEvent
	OrderID uuid.UUID `json:"orderId"`
	BuyerID string    `json:"buyerId"`
}

func (e *OrderConfirmedEvent) EventType() string { return "ordering.order.confirmed" }

type OrderCancelledEvent struct {
	bbdomain.BaseDomainEvent
	OrderID uuid.UUID `json:"orderId"`
	BuyerID string    `json:"buyerId"`
	Reason  string    `json:"reason"`
}

func (e *OrderCancelledEvent) EventType() string { return "ordering.order.cancelled" }

// ─── Helpers ─────────────────────────────────────────────────────────────────

func generateOrderNumber() string {
	// UUID v7 first 8 chars + timestamp suffix for human-readable order numbers.
	id := uuid.Must(uuid.NewV7())
	return fmt.Sprintf("ORD-%s", strings.ToUpper(id.String()[:8]))
}

var (
	zipCodeRegex = regexp.MustCompile(`^[A-Za-z0-9\- ]{3,12}$`)
	phoneRegex   = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)
)

func isValidZipCode(zipCode string) bool {
	return zipCodeRegex.MatchString(strings.TrimSpace(zipCode))
}

func isValidPhone(phone string) bool {
	return phoneRegex.MatchString(strings.TrimSpace(phone))
}
