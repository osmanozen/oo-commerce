package cqrs

import "context"

// Command represents a write operation that mutates state.
// Commands are handled by exactly one CommandHandler.
type Command interface {
	CommandName() string
}

// CommandHandler handles a specific command type.
type CommandHandler[C Command, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// Query represents a read-only operation.
// Queries are handled by exactly one QueryHandler.
type Query interface {
	QueryName() string
}

// QueryHandler handles a specific query type.
type QueryHandler[Q Query, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
}
