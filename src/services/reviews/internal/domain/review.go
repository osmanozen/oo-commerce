package domain

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/pkg/buildingblocks/types"
	"github.com/shopspring/decimal"
)

// ─── Strongly-Typed IDs ──────────────────────────────────────────────────────

type reviewTag struct{}

type ReviewID = types.TypedID[reviewTag]

func NewReviewID() ReviewID                         { return types.NewTypedID[reviewTag]() }
func ReviewIDFromString(s string) (ReviewID, error) { return types.TypedIDFromString[reviewTag](s) }

// ─── Value Objects ───────────────────────────────────────────────────────────

type Rating struct {
	value int
}

func NewRating(value int) (Rating, error) {
	if value < 1 || value > 5 {
		return Rating{}, errors.New("rating must be between 1 and 5")
	}
	return Rating{value: value}, nil
}

func (r Rating) Value() int { return r.value }

type ReviewText struct {
	value string
}

func NewReviewText(text string) (ReviewText, error) {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < 10 {
		return ReviewText{}, errors.New("review text must be at least 10 characters")
	}
	if len(trimmed) > 1000 {
		return ReviewText{}, errors.New("review text must be at most 1000 characters")
	}
	return ReviewText{value: trimmed}, nil
}

func (r ReviewText) String() string { return r.value }

// ─── Review Aggregate Root ───────────────────────────────────────────────────

type Review struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID        ReviewID   `json:"id" db:"id"`
	ProductID uuid.UUID  `json:"productId" db:"product_id"`
	UserID    uuid.UUID  `json:"userId" db:"user_id"`
	Rating    Rating     `json:"rating"`
	Text      ReviewText `json:"text"`
}

func NewReview(productID, userID uuid.UUID, rating int, text string) (*Review, error) {
	ratingVO, err := NewRating(rating)
	if err != nil {
		return nil, err
	}
	textVO, err := NewReviewText(text)
	if err != nil {
		return nil, err
	}

	r := &Review{
		ID:        NewReviewID(),
		ProductID: productID,
		UserID:    userID,
		Rating:    ratingVO,
		Text:      textVO,
	}
	r.SetCreated()

	r.AddDomainEvent(&ReviewCreatedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ReviewID:        r.ID.Value(),
		ProductID:       productID,
		Rating:          rating,
	})

	return r, nil
}

func (r *Review) Update(rating int, text string) error {
	ratingVO, err := NewRating(rating)
	if err != nil {
		return err
	}
	textVO, err := NewReviewText(text)
	if err != nil {
		return err
	}

	r.Rating = ratingVO
	r.Text = textVO
	r.SetUpdated()
	r.IncrementVersion()

	r.AddDomainEvent(&ReviewUpdatedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ReviewID:        r.ID.Value(),
		ProductID:       r.ProductID,
		Rating:          rating,
	})

	return nil
}

func (r *Review) MarkDeleted() {
	r.AddDomainEvent(&ReviewDeletedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		ReviewID:        r.ID.Value(),
		ProductID:       r.ProductID,
		Rating:          r.Rating.Value(),
	})
}

// ─── Rating Statistics ───────────────────────────────────────────────────────

type RatingStats struct {
	ProductID     uuid.UUID        `json:"productId" db:"product_id"`
	AverageRating *decimal.Decimal `json:"averageRating" db:"average_rating"`
	ReviewCount   int              `json:"reviewCount" db:"review_count"`
}

// ─── Domain Events ───────────────────────────────────────────────────────────

type ReviewCreatedEvent struct {
	bbdomain.BaseDomainEvent
	ReviewID  uuid.UUID `json:"reviewId"`
	ProductID uuid.UUID `json:"productId"`
	Rating    int       `json:"rating"`
}

func (e *ReviewCreatedEvent) EventType() string { return "reviews.review.created" }

type ReviewUpdatedEvent struct {
	bbdomain.BaseDomainEvent
	ReviewID  uuid.UUID `json:"reviewId"`
	ProductID uuid.UUID `json:"productId"`
	Rating    int       `json:"rating"`
}

func (e *ReviewUpdatedEvent) EventType() string { return "reviews.review.updated" }

type ReviewDeletedEvent struct {
	bbdomain.BaseDomainEvent
	ReviewID  uuid.UUID `json:"reviewId"`
	ProductID uuid.UUID `json:"productId"`
	Rating    int       `json:"rating"`
}

func (e *ReviewDeletedEvent) EventType() string { return "reviews.review.deleted" }

// ─── Verified Purchase Checker ───────────────────────────────────────────────

type PurchaseVerifier interface {
	HasPurchased(ctx context.Context, userID string, productID uuid.UUID) (bool, error)
	GetVerifiedUserIDs(ctx context.Context, productID uuid.UUID, userIDs []string) (map[string]struct{}, error)
}

type ProfileReader interface {
	GetDisplayNames(ctx context.Context, userIDs []string) (map[string]string, error)
}

// ─── Review Repository ──────────────────────────────────────────────────────

type ReviewRepository interface {
	Create(ctx context.Context, review *Review) error
	GetByID(ctx context.Context, id ReviewID) (*Review, error)
	GetByProductID(ctx context.Context, productID uuid.UUID, offset, limit int) ([]Review, int, error)
	GetByUserAndProduct(ctx context.Context, userID, productID uuid.UUID) (*Review, error)
	Update(ctx context.Context, review *Review) error
	Delete(ctx context.Context, review *Review) error
	GetRatingStats(ctx context.Context, productID uuid.UUID) (*RatingStats, error)
	ExistsByUserAndProduct(ctx context.Context, userID, productID uuid.UUID) (bool, error)
}

type ReviewEventPublisher interface {
	PublishRatingUpdate(ctx context.Context, stats *RatingStats) error
}
