package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaConsumer implements EventSubscriber using Kafka consumer groups.
//
// Design decisions:
//   - Consumer groups for horizontal scaling (each service instance joins the same group)
//   - Auto-commit disabled — manual commit after successful processing (at-least-once)
//   - Heartbeat and session timeouts tuned for containerized environments
//   - Each Subscribe() call spawns a goroutine that reads in a loop until ctx is cancelled
type KafkaConsumer struct {
	brokers []string
	readers []*kafka.Reader
	logger  *slog.Logger
}

// KafkaConsumerConfig holds configuration for the Kafka consumer.
type KafkaConsumerConfig struct {
	Brokers        []string
	MinBytes       int           // minimum batch size (default: 1 byte)
	MaxBytes       int           // maximum batch size (default: 10MB)
	MaxWait        time.Duration // max wait for new data (default: 500ms)
	CommitInterval time.Duration // auto-commit interval (default: 0 = manual)
	StartOffset    int64         // kafka.FirstOffset or kafka.LastOffset
}

// DefaultKafkaConsumerConfig returns production-ready consumer defaults.
func DefaultKafkaConsumerConfig(brokers []string) KafkaConsumerConfig {
	return KafkaConsumerConfig{
		Brokers:        brokers,
		MinBytes:       1,
		MaxBytes:       10 * 1024 * 1024, // 10MB
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 0, // manual commit for at-least-once semantics
		StartOffset:    kafka.LastOffset,
	}
}

// NewKafkaConsumer creates a new Kafka consumer.
func NewKafkaConsumer(brokers []string, logger *slog.Logger) *KafkaConsumer {
	return &KafkaConsumer{
		brokers: brokers,
		logger:  logger,
	}
}

// Subscribe starts consuming messages from the given topic using the specified consumer group.
// The handler is called for each message. Messages are committed only after successful processing.
//
// The groupID ensures that within a consumer group, each partition is assigned to exactly one consumer.
// This enables horizontal scaling: adding more service instances automatically rebalances partitions.
func (c *KafkaConsumer) Subscribe(ctx context.Context, topic string, groupID string, handler EventHandler) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        c.brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10 * 1024 * 1024,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 0, // manual commit
		StartOffset:    kafka.LastOffset,
		// Heartbeat/session tuning for Kubernetes liveness probes
		HeartbeatInterval: 3 * time.Second,
		SessionTimeout:    30 * time.Second,
		RebalanceTimeout:  60 * time.Second,
	})

	c.readers = append(c.readers, reader)

	c.logger.Info("kafka consumer subscribed",
		slog.String("topic", topic),
		slog.String("group_id", groupID),
		slog.Any("brokers", c.brokers),
	)

	// Start consuming in a goroutine.
	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("kafka consumer stopping",
					slog.String("topic", topic),
					slog.String("group_id", groupID),
				)
				return
			default:
				kafkaMsg, err := reader.FetchMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return // context cancelled — graceful shutdown
					}
					c.logger.ErrorContext(ctx, "kafka fetch error",
						slog.String("topic", topic),
						slog.String("error", err.Error()),
					)
					time.Sleep(1 * time.Second) // backoff on transient errors
					continue
				}

				// Convert kafka.Message to our Message type.
				msg := Message{
					Topic:     kafkaMsg.Topic,
					Key:       string(kafkaMsg.Key),
					Value:     kafkaMsg.Value,
					Partition: kafkaMsg.Partition,
					Offset:    kafkaMsg.Offset,
					Headers:   extractHeaders(kafkaMsg.Headers),
				}

				// Process the message.
				if err := handler(ctx, msg); err != nil {
					c.logger.ErrorContext(ctx, "event handler failed",
						slog.String("topic", topic),
						slog.String("key", msg.Key),
						slog.Int64("offset", msg.Offset),
						slog.Int("partition", msg.Partition),
						slog.String("error", err.Error()),
					)
					// Do NOT commit — message will be redelivered (at-least-once).
					// In production, add dead-letter queue after N retries.
					continue
				}

				// Commit only after successful processing.
				if err := reader.CommitMessages(ctx, kafkaMsg); err != nil {
					c.logger.ErrorContext(ctx, "kafka commit failed",
						slog.String("topic", topic),
						slog.Int64("offset", msg.Offset),
						slog.String("error", err.Error()),
					)
				}
			}
		}
	}()

	return nil
}

// Close gracefully shuts down all consumer readers.
func (c *KafkaConsumer) Close() error {
	c.logger.Info("closing kafka consumers", slog.Int("count", len(c.readers)))
	var errs []error
	for _, reader := range c.readers {
		if err := reader.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("closing kafka consumers: %v", errs)
	}
	return nil
}

func extractHeaders(headers []kafka.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for _, h := range headers {
		result[h.Key] = string(h.Value)
	}
	return result
}

// Compile-time interface compliance check.
var _ EventSubscriber = (*KafkaConsumer)(nil)
