package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaProducer implements EventBus using Apache Kafka.
// Uses segmentio/kafka-go for a pure-Go, dependency-free Kafka client.
//
// Design decisions:
//   - Async writes with RequiredAcks=All for durability
//   - Key-based partitioning for ordered event processing per aggregate
//   - Automatic topic creation disabled — topics must be pre-provisioned
type KafkaProducer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

// KafkaProducerConfig holds configuration for the Kafka producer.
type KafkaProducerConfig struct {
	Brokers      []string      // e.g., ["localhost:9092"]
	BatchSize    int           // messages per batch (default: 100)
	BatchTimeout time.Duration // max wait before flushing batch (default: 10ms)
	Async        bool          // fire-and-forget mode (default: false for outbox)
}

// DefaultKafkaProducerConfig returns production-ready defaults.
func DefaultKafkaProducerConfig(brokers []string) KafkaProducerConfig {
	return KafkaProducerConfig{
		Brokers:      brokers,
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		Async:        false, // synchronous for outbox — ensures delivery before marking sent
	}
}

// NewKafkaProducer creates a new Kafka producer.
func NewKafkaProducer(cfg KafkaProducerConfig, logger *slog.Logger) *KafkaProducer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.Hash{}, // consistent hashing by key → same partition
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		Async:        cfg.Async,
		RequiredAcks: kafka.RequireAll, // wait for all ISR replicas
		Compression:  kafka.Snappy,     // compress for throughput
	}

	logger.Info("kafka producer initialized",
		slog.Any("brokers", cfg.Brokers),
		slog.Int("batch_size", cfg.BatchSize),
		slog.Bool("async", cfg.Async),
	)

	return &KafkaProducer{
		writer: w,
		logger: logger,
	}
}

// Publish sends an event to the specified Kafka topic.
// The key determines the partition — events with the same key (e.g., aggregate ID)
// are guaranteed to be ordered within their partition.
func (p *KafkaProducer) Publish(ctx context.Context, topic string, key string, payload []byte) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now().UTC(),
		Headers: []kafka.Header{
			{Key: "content-type", Value: []byte("application/json")},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish to %s: %w", topic, err)
	}

	p.logger.DebugContext(ctx, "event published to kafka",
		slog.String("topic", topic),
		slog.String("key", key),
		slog.Int("payload_size", len(payload)),
	)

	return nil
}

// Close flushes pending messages and closes the producer.
func (p *KafkaProducer) Close() error {
	p.logger.Info("closing kafka producer")
	return p.writer.Close()
}

// Compile-time interface compliance check.
var _ EventBus = (*KafkaProducer)(nil)
