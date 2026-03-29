package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	bbdomain "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/domain"
	"github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/types"
	"github.com/shopspring/decimal"
)

type couponTag struct{}
type couponUsageTag struct{}

type CouponID = types.TypedID[couponTag]
type CouponUsageID = types.TypedID[couponUsageTag]

func NewCouponID() CouponID                          { return types.NewTypedID[couponTag]() }
func CouponIDFromString(s string) (CouponID, error) { return types.TypedIDFromString[couponTag](s) }

func NewCouponUsageID() CouponUsageID                          { return types.NewTypedID[couponUsageTag]() }
func CouponUsageIDFromString(s string) (CouponUsageID, error) { return types.TypedIDFromString[couponUsageTag](s) }

type DiscountType int

const (
	DiscountTypeUnknown DiscountType = iota
	DiscountTypePercentage
	DiscountTypeFixedAmount
)

var discountTypeNames = map[DiscountType]string{
	DiscountTypePercentage:  "Percentage",
	DiscountTypeFixedAmount: "FixedAmount",
}

func (d DiscountType) String() string {
	name, ok := discountTypeNames[d]
	if !ok {
		return "Unknown"
	}
	return name
}

func ParseDiscountType(name string) (DiscountType, error) {
	for discountType, discountTypeName := range discountTypeNames {
		if strings.EqualFold(discountTypeName, name) {
			return discountType, nil
		}
	}
	return DiscountTypeUnknown, errors.New("invalid discount type")
}

func (d DiscountType) Calculate(
	subtotal decimal.Decimal,
	discountValue decimal.Decimal,
	maxDiscountAmount *decimal.Decimal,
) decimal.Decimal {
	if subtotal.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}

	switch d {
	case DiscountTypePercentage:
		discount := subtotal.Mul(discountValue).Div(decimal.NewFromInt(100))
		if maxDiscountAmount != nil {
			discount = decimal.Min(discount, *maxDiscountAmount)
		}
		discount = discount.Round(2)
		return decimal.Min(discount, subtotal)
	case DiscountTypeFixedAmount:
		return decimal.Min(discountValue, subtotal).Round(2)
	default:
		return decimal.Zero
	}
}

type CouponUsage struct {
	ID              CouponUsageID   `json:"id" db:"id"`
	CouponID        CouponID        `json:"couponId" db:"coupon_id"`
	OrderID         uuid.UUID       `json:"orderId" db:"order_id"`
	UserID          string          `json:"userId" db:"user_id"`
	DiscountApplied decimal.Decimal `json:"discountApplied" db:"discount_applied"`
	UsedAt          time.Time       `json:"usedAt" db:"used_at"`
}

func NewCouponUsage(
	couponID CouponID,
	orderID uuid.UUID,
	userID string,
	discountApplied decimal.Decimal,
) *CouponUsage {
	return &CouponUsage{
		ID:              NewCouponUsageID(),
		CouponID:        couponID,
		OrderID:         orderID,
		UserID:          strings.TrimSpace(userID),
		DiscountApplied: discountApplied.Round(2),
		UsedAt:          time.Now().UTC(),
	}
}

type Coupon struct {
	bbdomain.BaseAggregateRoot
	bbdomain.Auditable
	bbdomain.Versionable

	ID                    CouponID         `json:"id" db:"id"`
	Code                  string           `json:"code" db:"code"`
	Description           string           `json:"description" db:"description"`
	DiscountType          DiscountType     `json:"discountType" db:"discount_type"`
	DiscountValue         decimal.Decimal  `json:"discountValue" db:"discount_value"`
	MinOrderAmount        *decimal.Decimal `json:"minOrderAmount,omitempty" db:"min_order_amount"`
	MaxDiscountAmount     *decimal.Decimal `json:"maxDiscountAmount,omitempty" db:"max_discount_amount"`
	UsageLimit            *int             `json:"usageLimit,omitempty" db:"usage_limit"`
	UsagePerUser          *int             `json:"usagePerUser,omitempty" db:"usage_per_user"`
	TimesUsed             int              `json:"timesUsed" db:"times_used"`
	ValidFrom             time.Time        `json:"validFrom" db:"valid_from"`
	ValidUntil            *time.Time       `json:"validUntil,omitempty" db:"valid_until"`
	IsActive              bool             `json:"isActive" db:"is_active"`
	ApplicableProductIDs  []uuid.UUID      `json:"applicableProductIds" db:"applicable_product_ids"`
	ApplicableCategoryIDs []uuid.UUID      `json:"applicableCategoryIds" db:"applicable_category_ids"`
	Usages                []CouponUsage    `json:"usages,omitempty"`
}

func NewCoupon(
	code string,
	description string,
	discountType DiscountType,
	discountValue decimal.Decimal,
	minOrderAmount *decimal.Decimal,
	maxDiscountAmount *decimal.Decimal,
	usageLimit *int,
	usagePerUser *int,
	validFrom time.Time,
	validUntil *time.Time,
	applicableProductIDs []uuid.UUID,
	applicableCategoryIDs []uuid.UUID,
) (*Coupon, error) {
	coupon := &Coupon{
		ID:                    NewCouponID(),
		Code:                  normalizeCouponCode(code),
		Description:           strings.TrimSpace(description),
		DiscountType:          discountType,
		DiscountValue:         discountValue,
		MinOrderAmount:        cloneDecimalPtr(minOrderAmount),
		MaxDiscountAmount:     cloneDecimalPtr(maxDiscountAmount),
		UsageLimit:            cloneIntPtr(usageLimit),
		UsagePerUser:          cloneIntPtr(usagePerUser),
		TimesUsed:             0,
		ValidFrom:             validFrom.UTC(),
		ValidUntil:            cloneTimePtr(validUntil),
		IsActive:              true,
		ApplicableProductIDs:  cloneUUIDSlice(applicableProductIDs),
		ApplicableCategoryIDs: cloneUUIDSlice(applicableCategoryIDs),
		Usages:                []CouponUsage{},
	}

	if err := coupon.validateState(); err != nil {
		return nil, err
	}

	coupon.SetCreated()
	return coupon, nil
}

func (c *Coupon) Update(
	description string,
	discountType DiscountType,
	discountValue decimal.Decimal,
	minOrderAmount *decimal.Decimal,
	maxDiscountAmount *decimal.Decimal,
	usageLimit *int,
	usagePerUser *int,
	validFrom time.Time,
	validUntil *time.Time,
	applicableProductIDs []uuid.UUID,
	applicableCategoryIDs []uuid.UUID,
) error {
	c.Description = strings.TrimSpace(description)
	c.DiscountType = discountType
	c.DiscountValue = discountValue
	c.MinOrderAmount = cloneDecimalPtr(minOrderAmount)
	c.MaxDiscountAmount = cloneDecimalPtr(maxDiscountAmount)
	c.UsageLimit = cloneIntPtr(usageLimit)
	c.UsagePerUser = cloneIntPtr(usagePerUser)
	c.ValidFrom = validFrom.UTC()
	c.ValidUntil = cloneTimePtr(validUntil)
	c.ApplicableProductIDs = cloneUUIDSlice(applicableProductIDs)
	c.ApplicableCategoryIDs = cloneUUIDSlice(applicableCategoryIDs)

	if err := c.validateState(); err != nil {
		return err
	}

	c.SetUpdated()
	c.IncrementVersion()
	return nil
}

func (c *Coupon) SetActiveStatus(isActive bool) {
	if c.IsActive == isActive {
		return
	}
	c.IsActive = isActive
	c.SetUpdated()
	c.IncrementVersion()
}

func (c *Coupon) IncrementUsage() {
	c.TimesUsed++
	c.SetUpdated()
	c.IncrementVersion()
}

func (c *Coupon) Validate(
	subtotal decimal.Decimal,
	now time.Time,
	userUsageCount int,
) (bool, decimal.Decimal, string) {
	now = now.UTC()

	if !c.IsActive {
		return false, decimal.Zero, "Coupon is not active"
	}

	if now.Before(c.ValidFrom) {
		return false, decimal.Zero, "Coupon is not yet valid"
	}

	if c.ValidUntil != nil && now.After(*c.ValidUntil) {
		return false, decimal.Zero, "Coupon has expired"
	}

	if c.UsageLimit != nil && c.TimesUsed >= *c.UsageLimit {
		return false, decimal.Zero, "Coupon usage limit reached"
	}

	if c.UsagePerUser != nil && userUsageCount >= *c.UsagePerUser {
		return false, decimal.Zero, "User usage limit reached"
	}

	if c.MinOrderAmount != nil && subtotal.LessThan(*c.MinOrderAmount) {
		return false, decimal.Zero, fmt.Sprintf("Minimum order amount required: %s", c.MinOrderAmount.StringFixed(2))
	}

	discountAmount := c.DiscountType.Calculate(subtotal, c.DiscountValue, c.MaxDiscountAmount)
	return true, discountAmount, ""
}

func (c *Coupon) Apply(
	subtotal decimal.Decimal,
	now time.Time,
	userID string,
	userUsageCount int,
	orderID uuid.UUID,
) (*CouponUsage, decimal.Decimal, error) {
	isValid, discountAmount, errMessage := c.Validate(subtotal, now, userUsageCount)
	if !isValid {
		return nil, decimal.Zero, errors.New(errMessage)
	}

	usage := NewCouponUsage(c.ID, orderID, userID, discountAmount)
	c.Usages = append(c.Usages, *usage)
	c.IncrementUsage()

	c.AddDomainEvent(&CouponAppliedEvent{
		BaseDomainEvent: bbdomain.NewBaseDomainEvent(),
		CouponID:        c.ID.Value(),
		OrderID:         orderID,
		UserID:          userID,
		DiscountApplied: discountAmount,
	})

	return usage, discountAmount, nil
}

func (c *Coupon) CalculateDiscount(subtotal decimal.Decimal) decimal.Decimal {
	return c.DiscountType.Calculate(subtotal, c.DiscountValue, c.MaxDiscountAmount)
}

func (c *Coupon) validateState() error {
	if err := validateCouponCode(c.Code); err != nil {
		return err
	}

	if strings.TrimSpace(c.Description) == "" {
		return errors.New("coupon description is required")
	}

	if c.DiscountType == DiscountTypeUnknown {
		return errors.New("discount type is required")
	}

	if !c.DiscountValue.IsPositive() {
		return errors.New("discount value must be positive")
	}

	if c.DiscountType == DiscountTypePercentage && c.DiscountValue.GreaterThan(decimal.NewFromInt(100)) {
		return errors.New("percentage discount cannot exceed 100")
	}

	if c.MinOrderAmount != nil && c.MinOrderAmount.IsNegative() {
		return errors.New("minimum order amount cannot be negative")
	}

	if c.MaxDiscountAmount != nil && !c.MaxDiscountAmount.IsPositive() {
		return errors.New("max discount amount must be positive")
	}

	if c.UsageLimit != nil && *c.UsageLimit <= 0 {
		return errors.New("usage limit must be greater than 0")
	}

	if c.UsagePerUser != nil && *c.UsagePerUser <= 0 {
		return errors.New("usage per user must be greater than 0")
	}

	if c.ValidFrom.IsZero() {
		return errors.New("valid from is required")
	}

	if c.ValidUntil != nil && !c.ValidUntil.After(c.ValidFrom) {
		return errors.New("valid until must be after valid from")
	}

	return nil
}

var couponCodeRegex = regexp.MustCompile(`^[A-Za-z0-9\-]+$`)

func normalizeCouponCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func validateCouponCode(code string) error {
	code = normalizeCouponCode(code)
	if len(code) < 2 || len(code) > 50 {
		return errors.New("coupon code must be 2-50 characters")
	}
	if !couponCodeRegex.MatchString(code) {
		return errors.New("coupon code contains invalid characters")
	}
	return nil
}

func cloneDecimalPtr(v *decimal.Decimal) *decimal.Decimal {
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

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

func cloneUUIDSlice(values []uuid.UUID) []uuid.UUID {
	if len(values) == 0 {
		return []uuid.UUID{}
	}
	cloned := make([]uuid.UUID, len(values))
	copy(cloned, values)
	return cloned
}

type CouponAppliedEvent struct {
	bbdomain.BaseDomainEvent
	CouponID        uuid.UUID       `json:"couponId"`
	OrderID         uuid.UUID       `json:"orderId"`
	UserID          string          `json:"userId"`
	DiscountApplied decimal.Decimal `json:"discountApplied"`
}

func (e *CouponAppliedEvent) EventType() string { return "coupons.coupon.applied" }
