package queries

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/persistence"
	"github.com/osmanozen/oo-commerce/src/services/reviews/internal/domain"
)

type GetReviewsByProductQuery struct {
	ProductID string `json:"-"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

func (q GetReviewsByProductQuery) QueryName() string { return "GetReviewsByProductQuery" }

type ReviewListItemDTO struct {
	ID                 string `json:"id"`
	UserID             string `json:"userId"`
	DisplayName        string `json:"displayName"`
	Rating             int    `json:"rating"`
	Text               string `json:"text"`
	CreatedAt          string `json:"createdAt"`
	IsVerifiedPurchase bool   `json:"isVerifiedPurchase"`
}

type GetReviewsByProductHandler struct {
	reviews   domain.ReviewRepository
	purchases domain.PurchaseVerifier
	profiles  domain.ProfileReader
}

func NewGetReviewsByProductHandler(reviews domain.ReviewRepository, purchases domain.PurchaseVerifier, profiles domain.ProfileReader) *GetReviewsByProductHandler {
	return &GetReviewsByProductHandler{reviews: reviews, purchases: purchases, profiles: profiles}
}

func (h *GetReviewsByProductHandler) Handle(ctx context.Context, query GetReviewsByProductQuery) (*persistence.PagedResult[ReviewListItemDTO], error) {
	productID, err := uuid.Parse(query.ProductID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid product id")
	}
	pg := persistence.NewPaginationParams(query.Page, query.PageSize)

	reviews, total, err := h.reviews.GetByProductID(ctx, productID, pg.Offset(), pg.Limit())
	if err != nil {
		return nil, fmt.Errorf("load product reviews: %w", err)
	}

	userIDs := make([]string, 0, len(reviews))
	seen := make(map[string]struct{}, len(reviews))
	for _, r := range reviews {
		uid := r.UserID.String()
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		userIDs = append(userIDs, uid)
	}

	displayNames := map[string]string{}
	if len(userIDs) > 0 {
		names, err := h.profiles.GetDisplayNames(ctx, userIDs)
		if err != nil {
			return nil, fmt.Errorf("load profile display names: %w", err)
		}
		displayNames = names
	}

	verifiedSet := map[string]struct{}{}
	if len(userIDs) > 0 {
		verified, err := h.purchases.GetVerifiedUserIDs(ctx, productID, userIDs)
		if err != nil {
			return nil, fmt.Errorf("load verified user ids: %w", err)
		}
		verifiedSet = verified
	}

	items := make([]ReviewListItemDTO, 0, len(reviews))
	for _, r := range reviews {
		uid := r.UserID.String()
		displayName := displayNames[uid]
		if displayName == "" {
			displayName = uid
		}
		_, isVerified := verifiedSet[uid]

		items = append(items, ReviewListItemDTO{
			ID:                 r.ID.String(),
			UserID:             uid,
			DisplayName:        displayName,
			Rating:             r.Rating.Value(),
			Text:               r.Text.String(),
			CreatedAt:          r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			IsVerifiedPurchase: isVerified,
		})
	}

	result := persistence.NewPagedResult(items, total, pg)
	return &result, nil
}

type GetUserReviewForProductQuery struct {
	ProductID string `json:"-"`
	UserID    string `json:"-"`
}

func (q GetUserReviewForProductQuery) QueryName() string { return "GetUserReviewForProductQuery" }

type MyReviewDTO struct {
	ID        string `json:"id"`
	ProductID string `json:"productId"`
	UserID    string `json:"userId"`
	Rating    int    `json:"rating"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type GetUserReviewForProductHandler struct {
	reviews domain.ReviewRepository
}

func NewGetUserReviewForProductHandler(reviews domain.ReviewRepository) *GetUserReviewForProductHandler {
	return &GetUserReviewForProductHandler{reviews: reviews}
}

func (h *GetUserReviewForProductHandler) Handle(ctx context.Context, query GetUserReviewForProductQuery) (*MyReviewDTO, error) {
	productID, err := uuid.Parse(query.ProductID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid product id")
	}
	userID, err := uuid.Parse(query.UserID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid user id")
	}

	review, err := h.reviews.GetByUserAndProduct(ctx, userID, productID)
	if err != nil {
		return nil, fmt.Errorf("load user review: %w", err)
	}
	if review == nil {
		return nil, nil
	}

	return &MyReviewDTO{
		ID:        review.ID.String(),
		ProductID: review.ProductID.String(),
		UserID:    review.UserID.String(),
		Rating:    review.Rating.Value(),
		Text:      review.Text.String(),
		CreatedAt: review.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: review.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

type CanReviewQuery struct {
	ProductID string `json:"-"`
	UserID    string `json:"-"`
}

func (q CanReviewQuery) QueryName() string { return "CanReviewQuery" }

type CanReviewResult struct {
	HasPurchased bool `json:"hasPurchased"`
	HasReviewed  bool `json:"hasReviewed"`
}

type CanReviewHandler struct {
	reviews   domain.ReviewRepository
	purchases domain.PurchaseVerifier
}

func NewCanReviewHandler(reviews domain.ReviewRepository, purchases domain.PurchaseVerifier) *CanReviewHandler {
	return &CanReviewHandler{reviews: reviews, purchases: purchases}
}

func (h *CanReviewHandler) Handle(ctx context.Context, query CanReviewQuery) (*CanReviewResult, error) {
	productID, err := uuid.Parse(query.ProductID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid product id")
	}
	userID, err := uuid.Parse(query.UserID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid user id")
	}

	hasPurchased, err := h.purchases.HasPurchased(ctx, query.UserID, productID)
	if err != nil {
		return nil, fmt.Errorf("check purchase eligibility: %w", err)
	}
	hasReviewed, err := h.reviews.ExistsByUserAndProduct(ctx, userID, productID)
	if err != nil {
		return nil, fmt.Errorf("check review existence: %w", err)
	}

	return &CanReviewResult{
		HasPurchased: hasPurchased,
		HasReviewed:  hasReviewed,
	}, nil
}
