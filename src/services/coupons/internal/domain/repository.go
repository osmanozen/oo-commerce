package domain

import "context"

type CouponListFilter struct {
	IsActive *bool
	Search   string
}

type CouponRepository interface {
	Create(ctx context.Context, coupon *Coupon) error
	Update(ctx context.Context, coupon *Coupon) error
	GetByID(ctx context.Context, id CouponID) (*Coupon, error)
	GetByCode(ctx context.Context, code string) (*Coupon, error)
	List(ctx context.Context, filter CouponListFilter, offset int, limit int) ([]*Coupon, int, error)
	CountUserUsage(ctx context.Context, couponID CouponID, userID string) (int, error)
}
