package cqrs

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Mediator dispatches commands and queries to their registered handlers.
// This is Go's equivalent of MediatR — in-process, type-safe routing.
type Mediator struct {
	mu       sync.RWMutex
	handlers map[string]interface{}
}

// NewMediator creates a new Mediator instance.
func NewMediator() *Mediator {
	return &Mediator{
		handlers: make(map[string]interface{}),
	}
}

// RegisterCommandHandler registers a handler for a specific command type.
func RegisterCommandHandler[C Command, R any](m *Mediator, handler CommandHandler[C, R]) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cmd C
	key := reflect.TypeOf(cmd).String()
	m.handlers[key] = handler
}

// RegisterQueryHandler registers a handler for a specific query type.
func RegisterQueryHandler[Q Query, R any](m *Mediator, handler QueryHandler[Q, R]) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var query Q
	key := reflect.TypeOf(query).String()
	m.handlers[key] = handler
}

// SendCommand dispatches a command to its registered handler.
func SendCommand[C Command, R any](ctx context.Context, m *Mediator, cmd C) (R, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := reflect.TypeOf(cmd).String()
	handler, ok := m.handlers[key]
	if !ok {
		var zero R
		return zero, fmt.Errorf("no handler registered for command: %s", key)
	}

	h, ok := handler.(CommandHandler[C, R])
	if !ok {
		var zero R
		return zero, fmt.Errorf("handler type mismatch for command: %s", key)
	}

	return h.Handle(ctx, cmd)
}

// SendQuery dispatches a query to its registered handler.
func SendQuery[Q Query, R any](ctx context.Context, m *Mediator, query Q) (R, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := reflect.TypeOf(query).String()
	handler, ok := m.handlers[key]
	if !ok {
		var zero R
		return zero, fmt.Errorf("no handler registered for query: %s", key)
	}

	h, ok := handler.(QueryHandler[Q, R])
	if !ok {
		var zero R
		return zero, fmt.Errorf("handler type mismatch for query: %s", key)
	}

	return h.Handle(ctx, query)
}
