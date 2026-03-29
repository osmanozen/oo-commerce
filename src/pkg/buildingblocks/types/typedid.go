package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// TypedID is a generic strongly-typed ID wrapper around UUID v7.
// It prevents accidental misuse of IDs across different entity types
// (e.g., passing a ProductID where a CategoryID is expected).
type TypedID[T any] struct {
	value uuid.UUID
}

// NewTypedID generates a new strongly-typed UUID v7 identifier.
func NewTypedID[T any]() TypedID[T] {
	return TypedID[T]{value: uuid.Must(uuid.NewV7())}
}

// TypedIDFrom creates a TypedID from an existing UUID.
func TypedIDFrom[T any](id uuid.UUID) (TypedID[T], error) {
	if id == uuid.Nil {
		return TypedID[T]{}, errors.New("id cannot be nil")
	}
	return TypedID[T]{value: id}, nil
}

// TypedIDFromString parses a string into a TypedID.
func TypedIDFromString[T any](s string) (TypedID[T], error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return TypedID[T]{}, fmt.Errorf("invalid id format: %w", err)
	}
	if id == uuid.Nil {
		return TypedID[T]{}, errors.New("id cannot be nil")
	}
	return TypedID[T]{value: id}, nil
}

// Value returns the underlying UUID.
func (t TypedID[T]) Value() uuid.UUID {
	return t.value
}

// String returns the string representation.
func (t TypedID[T]) String() string {
	return t.value.String()
}

// IsZero returns true if the ID is the zero value (nil UUID).
func (t TypedID[T]) IsZero() bool {
	return t.value == uuid.Nil
}

// MarshalJSON implements json.Marshaler.
func (t TypedID[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.value.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *TypedID[T]) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	t.value = id
	return nil
}

// Scan implements sql.Scanner for database reads.
func (t *TypedID[T]) Scan(src interface{}) error {
	switch v := src.(type) {
	case []byte:
		id, err := uuid.ParseBytes(v)
		if err != nil {
			return err
		}
		t.value = id
		return nil
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return err
		}
		t.value = id
		return nil
	default:
		return fmt.Errorf("unsupported scan source: %T", src)
	}
}

// DriverValue implements driver.Valuer for database writes.
func (t TypedID[T]) DriverValue() (driver.Value, error) {
	return t.value.String(), nil
}
