package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/messaging"
	"github.com/osmanozen/oo-commerce/services/reviews/internal/domain"
)

// ReviewRatingPublisher publishes rating statistics to Kafka
// so the Catalog service can update its product's cached rating.
//
// Flow: Review Created/Updated/Deleted → Recalculate Stats → Kafka → Catalog
type ReviewRatingPublisher struct {
	producer messaging.EventBus
	logger   *slog.Logger
}

func NewReviewRatingPublisher(producer messaging.EventBus, logger *slog.Logger) *ReviewRatingPublisher {
	return &ReviewRatingPublisher{
		producer: producer,
		logger:   logger,
	}
}

func (p *ReviewRatingPublisher) PublishRatingUpdate(ctx context.Context, stats *domain.RatingStats) error {
	type ratingUpdatePayload struct {
		ProductID     string   `json:"productId"`
		AverageRating *float64 `json:"averageRating"`
		ReviewCount   int      `json:"reviewCount"`
	}

	var avgFloat *float64
	if stats.AverageRating != nil {
		val, _ := stats.AverageRating.Float64()
		avgFloat = &val
	}

	payload, err := json.Marshal(ratingUpdatePayload{
		ProductID:     stats.ProductID.String(),
		AverageRating: avgFloat,
		ReviewCount:   stats.ReviewCount,
	})
	if err != nil {
		return fmt.Errorf("serializing rating update: %w", err)
	}

	if err := p.producer.Publish(ctx, "reviews.review.updated", stats.ProductID.String(), payload); err != nil {
		return fmt.Errorf("publishing rating update: %w", err)
	}

	avgLog := "null"
	if stats.AverageRating != nil {
		avgLog = stats.AverageRating.StringFixed(2)
	}

	p.logger.InfoContext(ctx, "rating update published to catalog",
		slog.String("product_id", stats.ProductID.String()),
		slog.String("avg_rating", avgLog),
		slog.Int("review_count", stats.ReviewCount),
	)

	return nil
}
