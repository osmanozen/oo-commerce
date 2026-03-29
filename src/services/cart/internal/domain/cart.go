package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/shopspring/decimal"
)

// ─── Strongly-Typed IDs ──────────────────────────────────────────────────────

type cartTag struct{}
type cartItemTag struct{}

type CartID = types.TypedID[cartTag]
type CartItemID = types.TypedID[cartItemTag]

func NewCartID() CartID                         { return types.NewTypedID[cartTag]() }
func NewCartItemID() CartItemID                 { return types.NewTypedID[cartItemTag]() }
func CartIDFromString(s string) (CartID, error) { return types.TypedIDFromString[cartTag](s) }
func CartItemIDFromString(s string) (CartItemID, error) {
	return types.TypedIDFromString[cartItemTag](s)
}

// ─── Buyer Identity (Value Object) ──────────────────────────────────────────

// BuyerIdentity identifies a cart owner — either an authenticated user or a guest via cookie.
type BuyerIdentity struct {
	UserID  *string `json:"userId,omitempty" db:"user_id"`
	GuestID *string `json:"guestId,omitempty" db:"guest_id"`
}

// IsAuthenticated returns true if the buyer is a logged-in user.
func (b BuyerIdentity) IsAuthenticated() bool {
	return b.UserID != nil && *b.UserID != ""
}

// IsGuest returns true if the buyer is identified by a guest cookie.
func (b BuyerIdentity) IsGuest() bool {
	return !b.IsAuthenticated() && b.GuestID != nil && *b.GuestID != ""
}

// ─── Cart Item (Owned Entity) ───────────────────────────────────────────────

// CartItem represents a denormalized product in the cart.
// Stores product name and price snapshots to avoid cross-service queries on read.
type CartItem struct {
	ID          CartItemID      `json:"id" db:"id"`
	CartID      CartID          `json:"cartId" db:"cart_id"`
	ProductID   uuid.UUID       `json:"productId" db:"product_id"`
	ProductName string          `json:"productName" db:"product_name"`
	ImageURL    *string         `json:"imageUrl,omitempty" db:"image_url"`
	UnitPrice   decimal.Decimal `json:"unitPrice" db:"unit_price"`
	Currency    string          `json:"currency" db:"currency"`
	Quantity    int             `json:"quantity" db:"quantity"`
	AddedAt     time.Time       `json:"addedAt" db:"added_at"`
}

// LineTotal returns the total for this line item.
func (i CartItem) LineTotal() decimal.Decimal {
	return i.UnitPrice.Mul(decimal.NewFromInt(int64(i.Quantity))).Round(2)
}

// ─── Cart Aggregate Root ─────────────────────────────────────────────────────

// Cart is the aggregate root for shopping cart management.
// Supports both guest (cookie-based) and authenticated user carts.
type Cart struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable

	ID    CartID        `json:"id" db:"id"`
	Buyer BuyerIdentity `json:"buyer"`
	Items []CartItem    `json:"items"`
}

// NewCart creates a new empty cart for the given buyer identity.
func NewCart(buyer BuyerIdentity) (*Cart, error) {
	if !buyer.IsAuthenticated() && !buyer.IsGuest() {
		return nil, errors.New("buyer must be either authenticated or guest")
	}

	cart := &Cart{
		ID:    NewCartID(),
		Buyer: buyer,
		Items: []CartItem{},
	}
	cart.SetCreated()
	return cart, nil
}

// AddItem adds a product to the cart or increments its quantity if already present.
func (c *Cart) AddItem(productID uuid.UUID, productName string, imageURL *string, unitPrice decimal.Decimal, currency string, quantity int) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}

	// Check if product already exists in cart → increment quantity.
	for i, item := range c.Items {
		if item.ProductID == productID {
			c.Items[i].Quantity += quantity
			c.SetUpdated()
			return nil
		}
	}

	// Add new item with denormalized product data.
	c.Items = append(c.Items, CartItem{
		ID:          NewCartItemID(),
		CartID:      c.ID,
		ProductID:   productID,
		ProductName: productName,
		ImageURL:    imageURL,
		UnitPrice:   unitPrice,
		Currency:    currency,
		Quantity:    quantity,
		AddedAt:     time.Now().UTC(),
	})

	c.SetUpdated()
	c.AddDomainEvent(&CartItemAddedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		CartID:          c.ID.Value(),
		ProductID:       productID,
		Quantity:        quantity,
	})
	return nil
}

// UpdateQuantity sets the quantity of a specific item.
func (c *Cart) UpdateQuantity(itemID CartItemID, quantity int) error {
	if quantity <= 0 {
		return c.RemoveItem(itemID)
	}

	for i, item := range c.Items {
		if item.ID == itemID {
			c.Items[i].Quantity = quantity
			c.SetUpdated()
			return nil
		}
	}
	return errors.New("cart item not found")
}

// RemoveItem removes an item from the cart.
func (c *Cart) RemoveItem(itemID CartItemID) error {
	for i, item := range c.Items {
		if item.ID == itemID {
			c.Items = append(c.Items[:i], c.Items[i+1:]...)
			c.SetUpdated()
			return nil
		}
	}
	return errors.New("cart item not found")
}

// Clear removes all items from the cart.
func (c *Cart) Clear() {
	c.Items = []CartItem{}
	c.SetUpdated()
}

// MergeFrom merges items from a guest cart into an authenticated user's cart.
// Used when a guest logs in: their guest cart items transfer to the user cart.
func (c *Cart) MergeFrom(guestCart *Cart) {
	for _, guestItem := range guestCart.Items {
		found := false
		for i, existingItem := range c.Items {
			if existingItem.ProductID == guestItem.ProductID {
				// Keep the higher quantity.
				if guestItem.Quantity > existingItem.Quantity {
					c.Items[i].Quantity = guestItem.Quantity
				}
				found = true
				break
			}
		}
		if !found {
			guestItem.CartID = c.ID
			guestItem.ID = NewCartItemID() // new ID for the merged item
			c.Items = append(c.Items, guestItem)
		}
	}
	c.SetUpdated()
}

// ItemCount returns the total number of items.
func (c *Cart) ItemCount() int {
	total := 0
	for _, item := range c.Items {
		total += item.Quantity
	}
	return total
}

// SubTotal calculates the cart subtotal.
func (c *Cart) SubTotal() decimal.Decimal {
	total := decimal.Zero
	for _, item := range c.Items {
		total = total.Add(item.LineTotal())
	}
	return total.Round(2)
}

// ─── Domain Events ───────────────────────────────────────────────────────────

type CartItemAddedEvent struct {
	bbdomain.BaseDomainEvent
	CartID    uuid.UUID `json:"cartId"`
	ProductID uuid.UUID `json:"productId"`
	Quantity  int       `json:"quantity"`
}

func (e *CartItemAddedEvent) EventType() string { return "cart.item.added" }
