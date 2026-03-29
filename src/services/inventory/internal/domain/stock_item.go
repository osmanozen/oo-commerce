package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
)

// ─── Strongly-Typed IDs ──────────────────────────────────────────────────────

type stockItemTag struct{}
type reservationTag struct{}
type adjustmentTag struct{}

type StockItemID = types.TypedID[stockItemTag]
type ReservationID = types.TypedID[reservationTag]
type AdjustmentID = types.TypedID[adjustmentTag]

func NewStockItemID() StockItemID     { return types.NewTypedID[stockItemTag]() }
func NewReservationID() ReservationID { return types.NewTypedID[reservationTag]() }
func NewAdjustmentID() AdjustmentID   { return types.NewTypedID[adjustmentTag]() }

func StockItemIDFromString(s string) (StockItemID, error) {
	return types.TypedIDFromString[stockItemTag](s)
}

func AdjustmentIDFromString(s string) (AdjustmentID, error) {
	return types.TypedIDFromString[adjustmentTag](s)
}

// ─── Stock Reservation (Owned Entity) ────────────────────────────────────────

// StockReservation represents a temporary hold on stock with TTL.
// Reservations expire after TTL to prevent deadlocks from abandoned checkouts.
type StockReservation struct {
	ID            ReservationID `json:"id" db:"id"`
	StockItemID   StockItemID   `json:"stockItemId" db:"stock_item_id"`
	OrderID       uuid.UUID     `json:"orderId" db:"order_id"`
	CorrelationID uuid.UUID     `json:"correlationId" db:"correlation_id"`
	Quantity      int           `json:"quantity" db:"quantity"`
	ReservedAt    time.Time     `json:"reservedAt" db:"reserved_at"`
	ExpiresAt     time.Time     `json:"expiresAt" db:"expires_at"`
	IsCommitted   bool          `json:"isCommitted" db:"is_committed"`
	IsReleased    bool          `json:"isReleased" db:"is_released"`
}

// IsActive returns true if the reservation is still holding stock.
func (r *StockReservation) IsActive() bool {
	return !r.IsCommitted && !r.IsReleased && time.Now().UTC().Before(r.ExpiresAt)
}

// ─── Stock Adjustment (Owned Entity) ─────────────────────────────────────────

type AdjustmentType int

const (
	AdjustmentTypeUnknown AdjustmentType = iota
	AdjustmentTypeReceived
	AdjustmentTypeSold
	AdjustmentTypeReturned
	AdjustmentTypeDamaged
	AdjustmentTypeAdjustment
)

type StockAdjustment struct {
	ID          AdjustmentID   `json:"id" db:"id"`
	StockItemID StockItemID    `json:"stockItemId" db:"stock_item_id"`
	Type        AdjustmentType `json:"type" db:"adjustment_type"`
	Quantity    int            `json:"quantity" db:"quantity"`
	Reason      string         `json:"reason" db:"reason"`
	CreatedAt   time.Time      `json:"createdAt" db:"created_at"`
	CreatedBy   string         `json:"createdBy" db:"created_by"`
}

// ─── StockItem Aggregate Root ────────────────────────────────────────────────

const defaultReservationTTL = 15 * time.Minute

// StockItem is the aggregate root for inventory management.
// It owns StockReservation and StockAdjustment entities.
//
// Key invariant: AvailableQuantity = TotalQuantity - ActiveReservations
type StockItem struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID            StockItemID        `json:"id" db:"id"`
	ProductID     uuid.UUID          `json:"productId" db:"product_id"`
	SKU           string             `json:"sku" db:"sku"`
	TotalQuantity int                `json:"totalQuantity" db:"total_quantity"`
	LowStockLevel int                `json:"lowStockLevel" db:"low_stock_level"`
	Reservations  []StockReservation `json:"reservations,omitempty"`
	Adjustments   []StockAdjustment  `json:"adjustments,omitempty"`
}

// NewStockItem creates a StockItem from a ProductCreatedEvent.
func NewStockItem(productID uuid.UUID, sku string) *StockItem {
	s := &StockItem{
		ID:            NewStockItemID(),
		ProductID:     productID,
		SKU:           sku,
		TotalQuantity: 0,
		LowStockLevel: 10, // default threshold
	}
	s.SetCreated()
	return s
}

// AvailableQuantity calculates real-time available stock (total - active reservations).
func (s *StockItem) AvailableQuantity() int {
	reserved := 0
	for _, r := range s.Reservations {
		if r.IsActive() {
			reserved += r.Quantity
		}
	}
	return s.TotalQuantity - reserved
}

// Reserve creates a TTL-based reservation for the checkout saga.
func (s *StockItem) Reserve(orderID, correlationID uuid.UUID, quantity int) (*StockReservation, error) {
	if quantity <= 0 {
		return nil, errors.New("reservation quantity must be positive")
	}

	available := s.AvailableQuantity()
	if available < quantity {
		// Raise failure event for saga compensation.
		s.AddDomainEvent(&StockReservationFailedEvent{
			BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
			ProductID:       s.ProductID,
			CorrelationID:   correlationID,
			Requested:       quantity,
			Available:       available,
		})
		return nil, fmt.Errorf("insufficient stock: requested %d, available %d", quantity, available)
	}

	now := time.Now().UTC()
	reservation := StockReservation{
		ID:            NewReservationID(),
		StockItemID:   s.ID,
		OrderID:       orderID,
		CorrelationID: correlationID,
		Quantity:      quantity,
		ReservedAt:    now,
		ExpiresAt:     now.Add(defaultReservationTTL),
	}

	s.Reservations = append(s.Reservations, reservation)
	s.SetUpdated()
	s.IncrementVersion()

	// Raise success event for saga.
	s.AddDomainEvent(&StockReservedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ProductID:       s.ProductID,
		CorrelationID:   correlationID,
		ReservationID:   reservation.ID.Value(),
		Quantity:        quantity,
	})

	// Check low stock threshold.
	if s.AvailableQuantity() <= s.LowStockLevel {
		s.AddDomainEvent(&StockLowEvent{
			BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
			ProductID:       s.ProductID,
			SKU:             s.SKU,
			Available:       s.AvailableQuantity(),
			Threshold:       s.LowStockLevel,
		})
	}

	return &reservation, nil
}

// CommitReservation converts a reservation into a permanent stock deduction.
func (s *StockItem) CommitReservation(reservationID ReservationID) error {
	for i, r := range s.Reservations {
		if r.ID == reservationID {
			if r.IsCommitted {
				return errors.New("reservation already committed")
			}
			if r.IsReleased {
				return errors.New("reservation already released")
			}
			s.Reservations[i].IsCommitted = true
			s.TotalQuantity -= r.Quantity
			s.SetUpdated()
			s.IncrementVersion()
			return nil
		}
	}
	return errors.New("reservation not found")
}

// ReleaseReservation cancels a reservation and returns stock to available pool.
func (s *StockItem) ReleaseReservation(reservationID ReservationID) error {
	for i, r := range s.Reservations {
		if r.ID == reservationID {
			if r.IsReleased {
				return errors.New("reservation already released")
			}
			if r.IsCommitted {
				return errors.New("reservation already committed")
			}
			s.Reservations[i].IsReleased = true
			s.SetUpdated()
			s.IncrementVersion()

			s.AddDomainEvent(&StockReleasedEvent{
				BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
				ProductID:       s.ProductID,
				CorrelationID:   r.CorrelationID,
				ReservationID:   reservationID.Value(),
				Quantity:        r.Quantity,
			})
			return nil
		}
	}
	return errors.New("reservation not found")
}

// AdjustStock records a stock adjustment and updates the total.
func (s *StockItem) AdjustStock(adjustType AdjustmentType, quantity int, reason, createdBy string) error {
	if quantity == 0 {
		return errors.New("adjustment quantity cannot be zero")
	}

	adj := StockAdjustment{
		ID:          NewAdjustmentID(),
		StockItemID: s.ID,
		Type:        adjustType,
		Quantity:    quantity,
		Reason:      reason,
		CreatedAt:   time.Now().UTC(),
		CreatedBy:   createdBy,
	}
	s.Adjustments = append(s.Adjustments, adj)
	s.TotalQuantity += quantity
	s.SetUpdated()
	s.IncrementVersion()

	s.AddDomainEvent(&StockAdjustedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ProductID:       s.ProductID,
		SKU:             s.SKU,
		Quantity:        quantity,
		NewTotal:        s.TotalQuantity,
	})

	return nil
}

// ─── Domain Events ───────────────────────────────────────────────────────────

type StockReservedEvent struct {
	bbdomain.BaseDomainEvent
	ProductID     uuid.UUID `json:"productId"`
	CorrelationID uuid.UUID `json:"correlationId"`
	ReservationID uuid.UUID `json:"reservationId"`
	Quantity      int       `json:"quantity"`
}

func (e *StockReservedEvent) EventType() string { return "inventory.stock.reserved" }

type StockReservationFailedEvent struct {
	bbdomain.BaseDomainEvent
	ProductID     uuid.UUID `json:"productId"`
	CorrelationID uuid.UUID `json:"correlationId"`
	Requested     int       `json:"requested"`
	Available     int       `json:"available"`
}

func (e *StockReservationFailedEvent) EventType() string { return "inventory.stock.reservation-failed" }

type StockReleasedEvent struct {
	bbdomain.BaseDomainEvent
	ProductID     uuid.UUID `json:"productId"`
	CorrelationID uuid.UUID `json:"correlationId"`
	ReservationID uuid.UUID `json:"reservationId"`
	Quantity      int       `json:"quantity"`
}

func (e *StockReleasedEvent) EventType() string { return "inventory.stock.released" }

type StockAdjustedEvent struct {
	bbdomain.BaseDomainEvent
	ProductID uuid.UUID `json:"productId"`
	SKU       string    `json:"sku"`
	Quantity  int       `json:"quantity"`
	NewTotal  int       `json:"newTotal"`
}

func (e *StockAdjustedEvent) EventType() string { return "inventory.stock.adjusted" }

type StockLowEvent struct {
	bbdomain.BaseDomainEvent
	ProductID uuid.UUID `json:"productId"`
	SKU       string    `json:"sku"`
	Available int       `json:"available"`
	Threshold int       `json:"threshold"`
}

func (e *StockLowEvent) EventType() string { return "inventory.stock.low" }
