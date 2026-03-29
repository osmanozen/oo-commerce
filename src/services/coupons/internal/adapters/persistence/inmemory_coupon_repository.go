package persistence

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/osmanozen/oo-commerce/services/coupons/internal/domain"
)

type InMemoryCouponRepository struct {
	mu   sync.RWMutex
	data map[string]*domain.Coupon
}

func NewInMemoryCouponRepository() *InMemoryCouponRepository {
	return &InMemoryCouponRepository{
		data: map[string]*domain.Coupon{},
	}
}

func (r *InMemoryCouponRepository) Create(_ context.Context, coupon *domain.Coupon) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data[coupon.ID.String()] = cloneCoupon(coupon)
	return nil
}

func (r *InMemoryCouponRepository) Update(_ context.Context, coupon *domain.Coupon) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data[coupon.ID.String()] = cloneCoupon(coupon)
	return nil
}

func (r *InMemoryCouponRepository) GetByID(_ context.Context, id domain.CouponID) (*domain.Coupon, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	coupon, ok := r.data[id.String()]
	if !ok {
		return nil, nil
	}
	return cloneCoupon(coupon), nil
}

func (r *InMemoryCouponRepository) GetByCode(_ context.Context, code string) (*domain.Coupon, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	for _, coupon := range r.data {
		if strings.EqualFold(coupon.Code, normalizedCode) {
			return cloneCoupon(coupon), nil
		}
	}

	return nil, nil
}

func (r *InMemoryCouponRepository) List(
	_ context.Context,
	filter domain.CouponListFilter,
	offset int,
	limit int,
) ([]*domain.Coupon, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	search := strings.ToUpper(strings.TrimSpace(filter.Search))
	filtered := make([]*domain.Coupon, 0, len(r.data))

	for _, coupon := range r.data {
		if filter.IsActive != nil && coupon.IsActive != *filter.IsActive {
			continue
		}
		if search != "" {
			inCode := strings.Contains(strings.ToUpper(coupon.Code), search)
			inDescription := strings.Contains(strings.ToUpper(coupon.Description), search)
			if !inCode && !inDescription {
				continue
			}
		}
		filtered = append(filtered, cloneCoupon(coupon))
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	if offset >= total {
		return []*domain.Coupon{}, total, nil
	}

	if limit <= 0 {
		limit = 20
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return filtered[offset:end], total, nil
}

func (r *InMemoryCouponRepository) CountUserUsage(
	_ context.Context,
	couponID domain.CouponID,
	userID string,
) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	coupon, ok := r.data[couponID.String()]
	if !ok {
		return 0, nil
	}

	count := 0
	for _, usage := range coupon.Usages {
		if usage.UserID == userID {
			count++
		}
	}
	return count, nil
}

func cloneCoupon(coupon *domain.Coupon) *domain.Coupon {
	if coupon == nil {
		return nil
	}

	cloned := *coupon
	cloned.ApplicableProductIDs = append([]uuid.UUID{}, coupon.ApplicableProductIDs...)
	cloned.ApplicableCategoryIDs = append([]uuid.UUID{}, coupon.ApplicableCategoryIDs...)
	cloned.Usages = append([]domain.CouponUsage{}, coupon.Usages...)

	if coupon.MinOrderAmount != nil {
		v := *coupon.MinOrderAmount
		cloned.MinOrderAmount = &v
	}
	if coupon.MaxDiscountAmount != nil {
		v := *coupon.MaxDiscountAmount
		cloned.MaxDiscountAmount = &v
	}
	if coupon.UsageLimit != nil {
		v := *coupon.UsageLimit
		cloned.UsageLimit = &v
	}
	if coupon.UsagePerUser != nil {
		v := *coupon.UsagePerUser
		cloned.UsagePerUser = &v
	}
	if coupon.ValidUntil != nil {
		v := *coupon.ValidUntil
		cloned.ValidUntil = &v
	}

	return &cloned
}

var _ domain.CouponRepository = (*InMemoryCouponRepository)(nil)
