package messaging

import "context"

// EventBus publishes domain events to the message broker.
// Implementations MUST be safe for concurrent use.
type EventBus interface {
	// Publish sends an event to the specified topic.
	// The key is used for Kafka partitioning — events with the same key
	// are guaranteed to land on the same partition (ordered processing).
	Publish(ctx context.Context, topic string, key string, payload []byte) error

	// Close gracefully shuts down the event bus, flushing pending messages.
	Close() error
}

// EventHandler processes a received event message.
type EventHandler func(ctx context.Context, msg Message) error

// Message represents a consumed event from the message broker.
type Message struct {
	Topic     string
	Key       string
	Value     []byte
	Partition int
	Offset    int64
	Headers   map[string]string
}

// EventSubscriber consumes events from the message broker.
// Implementations use Kafka consumer groups for horizontal scaling.
type EventSubscriber interface {
	// Subscribe registers a handler for a given topic.
	// The consumer group ensures each message is processed by exactly one
	// instance within the group — enabling parallel consumption at scale.
	Subscribe(ctx context.Context, topic string, groupID string, handler EventHandler) error

	// Close gracefully shuts down all consumer connections.
	Close() error
}
