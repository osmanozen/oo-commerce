-- Coupons Service Database Schema
-- Schema: coupons

CREATE SCHEMA IF NOT EXISTS coupons;

-- Coupons table
CREATE TABLE coupons.coupons (
    id uuid PRIMARY KEY,
    code varchar(50) NOT NULL UNIQUE,
    description varchar(500),
    discount_type int NOT NULL,
    discount_value numeric(18,2) NOT NULL CHECK (discount_value > 0),
    min_order_amount numeric(18,2) NOT NULL DEFAULT 0,
    max_discount numeric(18,2),
    start_date timestamptz NOT NULL,
    end_date timestamptz NOT NULL,
    max_usages int NOT NULL DEFAULT 0,
    max_usages_per_user int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0,
    CONSTRAINT chk_dates CHECK (end_date > start_date)
);

CREATE INDEX idx_coupons_code ON coupons.coupons(code);
CREATE INDEX idx_coupons_active ON coupons.coupons(is_active, start_date, end_date);

-- Coupon-Category mapping (many-to-many)
CREATE TABLE coupons.coupon_categories (
    coupon_id uuid NOT NULL REFERENCES coupons.coupons(id) ON DELETE CASCADE,
    category_id uuid NOT NULL,
    PRIMARY KEY (coupon_id, category_id)
);

-- Coupon usage tracking
CREATE TABLE coupons.coupon_usages (
    id uuid PRIMARY KEY,
    coupon_id uuid NOT NULL REFERENCES coupons.coupons(id) ON DELETE CASCADE,
    user_id varchar(100) NOT NULL,
    order_id uuid NOT NULL,
    used_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_usages_coupon_id ON coupons.coupon_usages(coupon_id);
CREATE INDEX idx_usages_user_id ON coupons.coupon_usages(coupon_id, user_id);
CREATE INDEX idx_usages_order_id ON coupons.coupon_usages(order_id);

-- Outbox table
CREATE TABLE coupons.outbox_messages (
    id bigserial PRIMARY KEY,
    message_id uuid NOT NULL UNIQUE,
    message_type varchar(256) NOT NULL,
    payload text NOT NULL,
    correlation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    retry_count int NOT NULL DEFAULT 0
);

CREATE INDEX idx_coupons_outbox_unsent ON coupons.outbox_messages(created_at) WHERE sent_at IS NULL;
