package queries

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/persistence"
	"github.com/osmanozen/oo-commerce/src/services/coupons/internal/domain"
	"github.com/shopspring/decimal"
)

type GetCouponsQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	IsActive *bool  `json:"isActive,omitempty"`
	Search   string `json:"search,omitempty"`
}

func (q GetCouponsQuery) QueryName() string { return "GetCouponsQuery" }

type CouponListItemDTO struct {
	ID                    string   `json:"id"`
	Code                  string   `json:"code"`
	Description           string   `json:"description"`
	DiscountType          string   `json:"discountType"`
	DiscountValue         float64  `json:"discountValue"`
	MinOrderAmount        *float64 `json:"minOrderAmount,omitempty"`
	MaxDiscountAmount     *float64 `json:"maxDiscountAmount,omitempty"`
	UsageLimit            *int     `json:"usageLimit,omitempty"`
	UsagePerUser          *int     `json:"usagePerUser,omitempty"`
	TimesUsed             int      `json:"timesUsed"`
	ValidFrom             string   `json:"validFrom"`
	ValidUntil            *string  `json:"validUntil,omitempty"`
	IsActive              bool     `json:"isActive"`
	ApplicableProductIDs  []string `json:"applicableProductIds"`
	ApplicableCategoryIDs []string `json:"applicableCategoryIds"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
}

type GetCouponsHandler struct {
	repo domain.CouponRepository
}

func NewGetCouponsHandler(repo domain.CouponRepository) *GetCouponsHandler {
	return &GetCouponsHandler{repo: repo}
}

func (h *GetCouponsHandler) Handle(ctx context.Context, query GetCouponsQuery) (*persistence.PagedResult[CouponListItemDTO], error) {
	pg := persistence.NewPaginationParams(query.Page, query.PageSize)
	filter := domain.CouponListFilter{
		IsActive: query.IsActive,
		Search:   strings.TrimSpace(query.Search),
	}

	coupons, totalCount, err := h.repo.List(ctx, filter, pg.Offset(), pg.Limit())
	if err != nil {
		return nil, fmt.Errorf("listing coupons: %w", err)
	}

	items := make([]CouponListItemDTO, 0, len(coupons))
	for _, coupon := range coupons {
		items = append(items, mapCouponToListItem(coupon))
	}

	result := persistence.NewPagedResult(items, totalCount, pg)
	return &result, nil
}

type GetCouponByIDQuery struct {
	ID string `json:"-"`
}

func (q GetCouponByIDQuery) QueryName() string { return "GetCouponByIdQuery" }

type GetCouponByIDHandler struct {
	repo domain.CouponRepository
}

func NewGetCouponByIDHandler(repo domain.CouponRepository) *GetCouponByIDHandler {
	return &GetCouponByIDHandler{repo: repo}
}

func (h *GetCouponByIDHandler) Handle(ctx context.Context, query GetCouponByIDQuery) (*CouponListItemDTO, error) {
	couponID, err := domain.CouponIDFromString(query.ID)
	if err != nil {
		return nil, bberrors.ValidationError("invalid coupon id")
	}

	coupon, err := h.repo.GetByID(ctx, couponID)
	if err != nil {
		return nil, fmt.Errorf("loading coupon: %w", err)
	}
	if coupon == nil {
		return nil, nil
	}

	result := mapCouponToListItem(coupon)
	return &result, nil
}

type ValidateCouponQuery struct {
	Code     string  `json:"code"`
	Subtotal float64 `json:"subtotal"`
	UserID   string  `json:"userId"`
}

func (q ValidateCouponQuery) QueryName() string { return "ValidateCouponQuery" }

type ValidateCouponResult struct {
	IsValid        bool     `json:"isValid"`
	DiscountAmount *float64 `json:"discountAmount"`
	ErrorMessage   *string  `json:"errorMessage"`
}

type ValidateCouponHandler struct {
	repo domain.CouponRepository
}

func NewValidateCouponHandler(repo domain.CouponRepository) *ValidateCouponHandler {
	return &ValidateCouponHandler{repo: repo}
}

func (h *ValidateCouponHandler) Handle(ctx context.Context, query ValidateCouponQuery) (*ValidateCouponResult, error) {
	coupon, err := h.repo.GetByCode(ctx, query.Code)
	if err != nil {
		return nil, fmt.Errorf("loading coupon by code: %w", err)
	}

	if coupon == nil {
		msg := "Coupon not found"
		return &ValidateCouponResult{
			IsValid:      false,
			ErrorMessage: &msg,
		}, nil
	}

	userUsageCount := 0
	userID := strings.TrimSpace(query.UserID)
	if userID != "" {
		userUsageCount, err = h.repo.CountUserUsage(ctx, coupon.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("counting user coupon usage: %w", err)
		}
	}

	isValid, discountAmount, errMessage := coupon.Validate(
		decimal.NewFromFloat(query.Subtotal),
		time.Now().UTC(),
		userUsageCount,
	)
	if !isValid {
		msg := errMessage
		return &ValidateCouponResult{
			IsValid:      false,
			ErrorMessage: &msg,
		}, nil
	}

	amount := discountAmount.Round(2).InexactFloat64()
	return &ValidateCouponResult{
		IsValid:        true,
		DiscountAmount: &amount,
	}, nil
}

func mapCouponToListItem(coupon *domain.Coupon) CouponListItemDTO {
	var minOrderAmount *float64
	if coupon.MinOrderAmount != nil {
		value := coupon.MinOrderAmount.Round(2).InexactFloat64()
		minOrderAmount = &value
	}

	var maxDiscountAmount *float64
	if coupon.MaxDiscountAmount != nil {
		value := coupon.MaxDiscountAmount.Round(2).InexactFloat64()
		maxDiscountAmount = &value
	}

	var validUntil *string
	if coupon.ValidUntil != nil {
		v := coupon.ValidUntil.UTC().Format(time.RFC3339)
		validUntil = &v
	}

	return CouponListItemDTO{
		ID:                    coupon.ID.String(),
		Code:                  coupon.Code,
		Description:           coupon.Description,
		DiscountType:          coupon.DiscountType.String(),
		DiscountValue:         coupon.DiscountValue.Round(2).InexactFloat64(),
		MinOrderAmount:        minOrderAmount,
		MaxDiscountAmount:     maxDiscountAmount,
		UsageLimit:            cloneIntPtr(coupon.UsageLimit),
		UsagePerUser:          cloneIntPtr(coupon.UsagePerUser),
		TimesUsed:             coupon.TimesUsed,
		ValidFrom:             coupon.ValidFrom.UTC().Format(time.RFC3339),
		ValidUntil:            validUntil,
		IsActive:              coupon.IsActive,
		ApplicableProductIDs:  uuidListToStrings(coupon.ApplicableProductIDs),
		ApplicableCategoryIDs: uuidListToStrings(coupon.ApplicableCategoryIDs),
		CreatedAt:             coupon.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:             coupon.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

func uuidListToStrings(ids []uuid.UUID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.String())
	}
	return values
}
