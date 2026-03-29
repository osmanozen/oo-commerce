package commands

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/reviews/internal/domain"
)

type CreateReviewCommand struct {
	ProductID string `json:"-"`
	UserID    string `json:"-"`
	Rating    int    `json:"rating"`
	Text      string `json:"text"`
}

type CreateReviewResult struct {
	ID string `json:"id"`
}

func (c CreateReviewCommand) CommandName() string { return "CreateReviewCommand" }

type CreateReviewHandler struct {
	reviews    domain.ReviewRepository
	purchases  domain.PurchaseVerifier
	publisher  domain.ReviewEventPublisher
}

func NewCreateReviewHandler(reviews domain.ReviewRepository, purchases domain.PurchaseVerifier, publisher domain.ReviewEventPublisher) *CreateReviewHandler {
	return &CreateReviewHandler{reviews: reviews, purchases: purchases, publisher: publisher}
}

func (h *CreateReviewHandler) Handle(ctx context.Context, cmd CreateReviewCommand) (*CreateReviewResult, error) {
	productID, err := uuid.Parse(cmd.ProductID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid product id")
	}
	userID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid user id")
	}

	hasPurchased, err := h.purchases.HasPurchased(ctx, cmd.UserID, productID)
	if err != nil {
		return nil, fmt.Errorf("verify purchase: %w", err)
	}
	if !hasPurchased {
		return nil, bberrors.ValidationError("you must purchase this product before reviewing")
	}

	review, err := domain.NewReview(productID, userID, cmd.Rating, cmd.Text)
	if err != nil {
		return nil, bberrors.ValidationError(err.Error())
	}

	if err := h.reviews.Create(ctx, review); err != nil {
		return nil, err
	}

	stats, err := h.reviews.GetRatingStats(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("load rating stats: %w", err)
	}
	if err := h.publisher.PublishRatingUpdate(ctx, stats); err != nil {
		return nil, fmt.Errorf("publish rating stats: %w", err)
	}

	return &CreateReviewResult{ID: review.ID.String()}, nil
}

type UpdateReviewCommand struct {
	ReviewID string `json:"-"`
	UserID   string `json:"-"`
	Rating   int    `json:"rating"`
	Text     string `json:"text"`
}

func (c UpdateReviewCommand) CommandName() string { return "UpdateReviewCommand" }

type UpdateReviewHandler struct {
	reviews   domain.ReviewRepository
	publisher domain.ReviewEventPublisher
}

func NewUpdateReviewHandler(reviews domain.ReviewRepository, publisher domain.ReviewEventPublisher) *UpdateReviewHandler {
	return &UpdateReviewHandler{reviews: reviews, publisher: publisher}
}

func (h *UpdateReviewHandler) Handle(ctx context.Context, cmd UpdateReviewCommand) (struct{}, error) {
	reviewID, err := domain.ReviewIDFromString(cmd.ReviewID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid review id")
	}
	userID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid user id")
	}

	review, err := h.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return struct{}{}, fmt.Errorf("load review: %w", err)
	}
	if review == nil {
		return struct{}{}, bberrors.NotFoundError("review", cmd.ReviewID)
	}
	if review.UserID != userID {
		return struct{}{}, bberrors.NewDomainError(bberrors.ErrForbidden, "review does not belong to user")
	}

	if err := review.Update(cmd.Rating, cmd.Text); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}
	if err := h.reviews.Update(ctx, review); err != nil {
		return struct{}{}, err
	}

	stats, err := h.reviews.GetRatingStats(ctx, review.ProductID)
	if err != nil {
		return struct{}{}, fmt.Errorf("load rating stats: %w", err)
	}
	if err := h.publisher.PublishRatingUpdate(ctx, stats); err != nil {
		return struct{}{}, fmt.Errorf("publish rating stats: %w", err)
	}

	return struct{}{}, nil
}

type DeleteReviewCommand struct {
	ReviewID string `json:"-"`
	UserID   string `json:"-"`
}

func (c DeleteReviewCommand) CommandName() string { return "DeleteReviewCommand" }

type DeleteReviewHandler struct {
	reviews   domain.ReviewRepository
	publisher domain.ReviewEventPublisher
}

func NewDeleteReviewHandler(reviews domain.ReviewRepository, publisher domain.ReviewEventPublisher) *DeleteReviewHandler {
	return &DeleteReviewHandler{reviews: reviews, publisher: publisher}
}

func (h *DeleteReviewHandler) Handle(ctx context.Context, cmd DeleteReviewCommand) (struct{}, error) {
	reviewID, err := domain.ReviewIDFromString(cmd.ReviewID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid review id")
	}
	userID, err := uuid.Parse(cmd.UserID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid user id")
	}

	review, err := h.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return struct{}{}, fmt.Errorf("load review: %w", err)
	}
	if review == nil {
		return struct{}{}, bberrors.NotFoundError("review", cmd.ReviewID)
	}
	if review.UserID != userID {
		return struct{}{}, bberrors.NewDomainError(bberrors.ErrForbidden, "review does not belong to user")
	}

	review.MarkDeleted()
	if err := h.reviews.Delete(ctx, review); err != nil {
		return struct{}{}, err
	}

	stats, err := h.reviews.GetRatingStats(ctx, review.ProductID)
	if err != nil {
		return struct{}{}, fmt.Errorf("load rating stats: %w", err)
	}
	if err := h.publisher.PublishRatingUpdate(ctx, stats); err != nil {
		return struct{}{}, fmt.Errorf("publish rating stats: %w", err)
	}
	return struct{}{}, nil
}
