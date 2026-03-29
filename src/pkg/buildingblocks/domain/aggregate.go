package domain

import "time"

// AggregateRoot is the interface that all aggregate roots must implement.
// Aggregate roots are consistency boundaries — all mutations go through the root.
type AggregateRoot interface {
	GetDomainEvents() []DomainEvent
	ClearDomainEvents()
}

// BaseAggregateRoot provides domain event collection capabilities for aggregate roots.
type BaseAggregateRoot struct {
	domainEvents []DomainEvent
}

// AddDomainEvent appends a domain event to be published after save.
func (a *BaseAggregateRoot) AddDomainEvent(event DomainEvent) {
	a.domainEvents = append(a.domainEvents, event)
}

// GetDomainEvents returns collected domain events.
func (a *BaseAggregateRoot) GetDomainEvents() []DomainEvent {
	return a.domainEvents
}

// ClearDomainEvents drains the event collection after publishing.
func (a *BaseAggregateRoot) ClearDomainEvents() {
	a.domainEvents = nil
}

// Auditable provides automatic timestamp fields for created/updated tracking.
type Auditable struct {
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

// SetCreated stamps both CreatedAt and UpdatedAt for new entities.
func (a *Auditable) SetCreated() {
	now := time.Now().UTC()
	a.CreatedAt = now
	a.UpdatedAt = now
}

// SetUpdated stamps UpdatedAt on modification.
func (a *Auditable) SetUpdated() {
	a.UpdatedAt = time.Now().UTC()
}

// Versionable provides optimistic concurrency control.
type Versionable struct {
	Version int `json:"version" db:"version"`
}

// IncrementVersion bumps the concurrency token for optimistic locking.
func (v *Versionable) IncrementVersion() {
	v.Version++
}
