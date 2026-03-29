package domain

import (
	"time"

	"github.com/google/uuid"
)

// DomainEvent represents a fact that happened in the domain.
// Every domain event is immutable and time-ordered via UUID v7.
type DomainEvent interface {
	EventID() uuid.UUID
	OccurredAt() time.Time
	EventType() string
}

// BaseDomainEvent provides common fields for all domain events.
type BaseDomainEvent struct {
	ID         uuid.UUID `json:"eventId"`
	OccurredOn time.Time `json:"occurredAt"`
}

// NewBaseDomainEvent creates a new base domain event with UUID v7 and current timestamp.
func NewBaseDomainEvent() BaseDomainEvent {
	return BaseDomainEvent{
		ID:         uuid.Must(uuid.NewV7()),
		OccurredOn: time.Now().UTC(),
	}
}

func (e BaseDomainEvent) EventID() uuid.UUID {
	return e.ID
}

func (e BaseDomainEvent) OccurredAt() time.Time {
	return e.OccurredOn
}
