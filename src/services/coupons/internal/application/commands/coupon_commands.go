package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	bberrors "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/src/services/coupons/internal/domain"
	"github.com/shopspring/decimal"
)

type CreateCouponCommand struct {
	Code                  string    `json:"code"`
	Description           string    `json:"description"`
	DiscountType          string    `json:"discountType"`
	DiscountValue         float64   `json:"discountValue"`
	MinOrderAmount        *float64  `json:"minOrderAmount,omitempty"`
	MaxDiscountAmount     *float64  `json:"maxDiscountAmount,omitempty"`
	UsageLimit            *int      `json:"usageLimit,omitempty"`
	UsagePerUser          *int      `json:"usagePerUser,omitempty"`
	ValidFrom             time.Time  `json:"validFrom"`
	ValidUntil            *time.Time `json:"validUntil,omitempty"`
	ApplicableProductIDs  []string  `json:"applicableProductIds"`
	ApplicableCategoryIDs []string  `json:"applicableCategoryIds"`
}

func (c CreateCouponCommand) CommandName() string { return "CreateCouponCommand" }

type CreateCouponResult struct {
	ID string `json:"id"`
}

type CreateCouponHandler struct {
	repo domain.CouponRepository
}

func NewCreateCouponHandler(repo domain.CouponRepository) *CreateCouponHandler {
	return &CreateCouponHandler{repo: repo}
}

func (h *CreateCouponHandler) Handle(ctx context.Context, cmd CreateCouponCommand) (*CreateCouponResult, error) {
	discountType, err := domain.ParseDiscountType(cmd.DiscountType)
	if err != nil {
		return nil, bberrors.ValidationError(err.Error())
	}

	productIDs, err := parseUUIDList(cmd.ApplicableProductIDs)
	if err != nil {
		return nil, bberrors.ValidationError("applicableProductIds contains invalid uuid")
	}
	categoryIDs, err := parseUUIDList(cmd.ApplicableCategoryIDs)
	if err != nil {
		return nil, bberrors.ValidationError("applicableCategoryIds contains invalid uuid")
	}

	existing, err := h.repo.GetByCode(ctx, cmd.Code)
	if err != nil {
		return nil, fmt.Errorf("checking existing coupon code: %w", err)
	}
	if existing != nil {
		return nil, bberrors.ConflictError("coupon", "code", strings.ToUpper(strings.TrimSpace(cmd.Code)))
	}

	coupon, err := domain.NewCoupon(
		cmd.Code,
		cmd.Description,
		discountType,
		decimal.NewFromFloat(cmd.DiscountValue),
		floatPtrToDecimal(cmd.MinOrderAmount),
		floatPtrToDecimal(cmd.MaxDiscountAmount),
		cloneIntPtr(cmd.UsageLimit),
		cloneIntPtr(cmd.UsagePerUser),
		cmd.ValidFrom,
		cloneTimePtr(cmd.ValidUntil),
		productIDs,
		categoryIDs,
	)
	if err != nil {
		return nil, bberrors.ValidationError(err.Error())
	}

	if err := h.repo.Create(ctx, coupon); err != nil {
		return nil, fmt.Errorf("creating coupon: %w", err)
	}

	return &CreateCouponResult{ID: coupon.ID.String()}, nil
}

type UpdateCouponCommand struct {
	ID                    string     `json:"-"`
	Description           string     `json:"description"`
	DiscountType          string     `json:"discountType"`
	DiscountValue         float64    `json:"discountValue"`
	MinOrderAmount        *float64   `json:"minOrderAmount,omitempty"`
	MaxDiscountAmount     *float64   `json:"maxDiscountAmount,omitempty"`
	UsageLimit            *int       `json:"usageLimit,omitempty"`
	UsagePerUser          *int       `json:"usagePerUser,omitempty"`
	ValidFrom             time.Time  `json:"validFrom"`
	ValidUntil            *time.Time `json:"validUntil,omitempty"`
	ApplicableProductIDs  []string   `json:"applicableProductIds"`
	ApplicableCategoryIDs []string   `json:"applicableCategoryIds"`
}

func (c UpdateCouponCommand) CommandName() string { return "UpdateCouponCommand" }

type UpdateCouponHandler struct {
	repo domain.CouponRepository
}

func NewUpdateCouponHandler(repo domain.CouponRepository) *UpdateCouponHandler {
	return &UpdateCouponHandler{repo: repo}
}

func (h *UpdateCouponHandler) Handle(ctx context.Context, cmd UpdateCouponCommand) (struct{}, error) {
	couponID, err := domain.CouponIDFromString(cmd.ID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid coupon id")
	}

	coupon, err := h.repo.GetByID(ctx, couponID)
	if err != nil {
		return struct{}{}, fmt.Errorf("loading coupon: %w", err)
	}
	if coupon == nil {
		return struct{}{}, bberrors.NotFoundError("coupon", cmd.ID)
	}

	discountType, err := domain.ParseDiscountType(cmd.DiscountType)
	if err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	productIDs, err := parseUUIDList(cmd.ApplicableProductIDs)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("applicableProductIds contains invalid uuid")
	}
	categoryIDs, err := parseUUIDList(cmd.ApplicableCategoryIDs)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("applicableCategoryIds contains invalid uuid")
	}

	if err := coupon.Update(
		cmd.Description,
		discountType,
		decimal.NewFromFloat(cmd.DiscountValue),
		floatPtrToDecimal(cmd.MinOrderAmount),
		floatPtrToDecimal(cmd.MaxDiscountAmount),
		cloneIntPtr(cmd.UsageLimit),
		cloneIntPtr(cmd.UsagePerUser),
		cmd.ValidFrom,
		cloneTimePtr(cmd.ValidUntil),
		productIDs,
		categoryIDs,
	); err != nil {
		return struct{}{}, bberrors.ValidationError(err.Error())
	}

	if err := h.repo.Update(ctx, coupon); err != nil {
		return struct{}{}, fmt.Errorf("updating coupon: %w", err)
	}

	return struct{}{}, nil
}

type ToggleCouponStatusCommand struct {
	ID       string `json:"-"`
	IsActive bool   `json:"isActive"`
}

func (c ToggleCouponStatusCommand) CommandName() string { return "ToggleCouponStatusCommand" }

type ToggleCouponStatusHandler struct {
	repo domain.CouponRepository
}

func NewToggleCouponStatusHandler(repo domain.CouponRepository) *ToggleCouponStatusHandler {
	return &ToggleCouponStatusHandler{repo: repo}
}

func (h *ToggleCouponStatusHandler) Handle(ctx context.Context, cmd ToggleCouponStatusCommand) (struct{}, error) {
	couponID, err := domain.CouponIDFromString(cmd.ID)
	if err != nil {
		return struct{}{}, bberrors.ValidationError("invalid coupon id")
	}

	coupon, err := h.repo.GetByID(ctx, couponID)
	if err != nil {
		return struct{}{}, fmt.Errorf("loading coupon: %w", err)
	}
	if coupon == nil {
		return struct{}{}, bberrors.NotFoundError("coupon", cmd.ID)
	}

	coupon.SetActiveStatus(cmd.IsActive)
	if err := h.repo.Update(ctx, coupon); err != nil {
		return struct{}{}, fmt.Errorf("updating coupon status: %w", err)
	}

	return struct{}{}, nil
}

func floatPtrToDecimal(v *float64) *decimal.Decimal {
	if v == nil {
		return nil
	}
	d := decimal.NewFromFloat(*v)
	return &d
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

func cloneTimePtr(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	cloned := v.UTC()
	return &cloned
}

func parseUUIDList(values []string) ([]uuid.UUID, error) {
	if len(values) == 0 {
		return []uuid.UUID{}, nil
	}

	parsed := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, id)
	}
	return parsed, nil
}

