package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"

	"github.com/segmentio/kafka-go"
)

// TopicConfig describes a Kafka topic to be provisioned.
type TopicConfig struct {
	Name              string
	NumPartitions     int
	ReplicationFactor int
	RetentionMs       int64 // -1 for infinite, default 7 days = 604800000
}

// KafkaTopics is the registry of all domain event topics.
// Partition count is tuned for parallel consumption: more partitions = more parallelism.
var KafkaTopics = []TopicConfig{
	// Catalog domain events
	{Name: "catalog.product.created", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "catalog.product.updated", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "catalog.product.deleted", NumPartitions: 6, ReplicationFactor: 3},

	// Cart domain events
	{Name: "cart.item.added", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "cart.item.removed", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "cart.cleared", NumPartitions: 3, ReplicationFactor: 3},

	// Ordering domain events
	{Name: "ordering.order.created", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "ordering.order.paid", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "ordering.order.confirmed", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "ordering.order.cancelled", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "ordering.checkout.initiated", NumPartitions: 12, ReplicationFactor: 3},

	// Inventory domain events
	{Name: "inventory.stock.reserved", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "inventory.stock.released", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "inventory.stock.adjusted", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "inventory.stock.low", NumPartitions: 3, ReplicationFactor: 3},
	{Name: "inventory.stock.reservation-failed", NumPartitions: 6, ReplicationFactor: 3},

	// Saga commands (request channels)
	{Name: "saga.reserve-stock", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "saga.release-stock", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "saga.process-payment", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "saga.refund-payment", NumPartitions: 6, ReplicationFactor: 3},

	// Payment domain events
	{Name: "payment.processed", NumPartitions: 12, ReplicationFactor: 3},
	{Name: "payment.failed", NumPartitions: 6, ReplicationFactor: 3},

	// Profiles domain events
	{Name: "profiles.profile.created", NumPartitions: 3, ReplicationFactor: 3},
	{Name: "profiles.profile.updated", NumPartitions: 3, ReplicationFactor: 3},

	// Reviews domain events
	{Name: "reviews.review.created", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "reviews.review.updated", NumPartitions: 6, ReplicationFactor: 3},
	{Name: "reviews.review.deleted", NumPartitions: 6, ReplicationFactor: 3},

	// Coupons domain events
	{Name: "coupons.coupon.applied", NumPartitions: 6, ReplicationFactor: 3},
}

// EnsureTopics creates Kafka topics if they don't already exist.
// Call this at application startup or as a migration step.
func EnsureTopics(ctx context.Context, brokerAddr string, topics []TopicConfig, logger *slog.Logger) error {
	conn, err := kafka.DialContext(ctx, "tcp", brokerAddr)
	if err != nil {
		return fmt.Errorf("connecting to kafka broker %s: %w", brokerAddr, err)
	}
	defer conn.Close()

	// Get controller for topic creation
	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("getting kafka controller: %w", err)
	}

	controllerConn, err := kafka.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("connecting to kafka controller: %w", err)
	}
	defer controllerConn.Close()

	topicConfigs := make([]kafka.TopicConfig, 0, len(topics))
	for _, t := range topics {
		topicConfigs = append(topicConfigs, kafka.TopicConfig{
			Topic:             t.Name,
			NumPartitions:     t.NumPartitions,
			ReplicationFactor: t.ReplicationFactor,
		})
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("creating kafka topics: %w", err)
	}

	logger.Info("kafka topics ensured", slog.Int("count", len(topics)))
	return nil
}
