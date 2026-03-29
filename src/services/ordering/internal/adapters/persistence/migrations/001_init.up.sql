-- Ordering Service Database Schema
-- Schema: ordering

CREATE SCHEMA IF NOT EXISTS ordering;

-- Orders table
CREATE TABLE ordering.orders (
    id uuid PRIMARY KEY,
    order_number varchar(20) NOT NULL UNIQUE,
    buyer_id varchar(100) NOT NULL,
    status int NOT NULL DEFAULT 1,
    payment_method int NOT NULL DEFAULT 0,

    -- Shipping address (embedded value object)
    shipping_first_name varchar(100) NOT NULL,
    shipping_last_name varchar(100) NOT NULL,
    shipping_street varchar(200) NOT NULL,
    shipping_city varchar(100) NOT NULL,
    shipping_state varchar(100),
    shipping_zip_code varchar(20),
    shipping_country varchar(100) NOT NULL,
    shipping_phone varchar(20),

    -- Billing address (embedded value object)
    billing_first_name varchar(100) NOT NULL,
    billing_last_name varchar(100) NOT NULL,
    billing_street varchar(200) NOT NULL,
    billing_city varchar(100) NOT NULL,
    billing_state varchar(100),
    billing_zip_code varchar(20),
    billing_country varchar(100) NOT NULL,
    billing_phone varchar(20),

    -- Money fields
    subtotal_amount numeric(18,2) NOT NULL,
    subtotal_currency varchar(3) NOT NULL,
    tax_amount numeric(18,2) NOT NULL,
    tax_currency varchar(3) NOT NULL,
    total_amount numeric(18,2) NOT NULL,
    total_currency varchar(3) NOT NULL,

    placed_at timestamptz,
    confirmed_at timestamptz,
    cancelled_at timestamptz,
    cancel_reason varchar(500),
    paid_at timestamptz,
    shipped_at timestamptz,
    delivered_at timestamptz;

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0
);

CREATE INDEX idx_orders_buyer_id ON ordering.orders(buyer_id);
CREATE INDEX idx_orders_status ON ordering.orders(status);
CREATE INDEX idx_orders_order_number ON ordering.orders(order_number);
CREATE INDEX idx_orders_placed_at ON ordering.orders(placed_at DESC);

-- Order items table
CREATE TABLE ordering.order_items (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES ordering.orders(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    product_name varchar(200) NOT NULL,
    price_amount numeric(18,2) NOT NULL,
    price_currency varchar(3) NOT NULL,
    quantity int NOT NULL CHECK (quantity > 0),
    line_total_amount numeric(18,2) NOT NULL,
    line_total_currency varchar(3) NOT NULL
);

CREATE INDEX idx_order_items_order_id ON ordering.order_items(order_id);
CREATE INDEX idx_order_items_product_id ON ordering.order_items(product_id);

-- Checkout saga state persistence
CREATE TABLE ordering.checkout_saga_state (
    correlation_id uuid PRIMARY KEY,
    current_state varchar(50) NOT NULL,
    order_id uuid NOT NULL,
    buyer_id varchar(100) NOT NULL,
    saga_data jsonb NOT NULL,
    started_at timestamptz NOT NULL,
    completed_at timestamptz,
    failed_at timestamptz,
    fail_reason varchar(500),
    version int NOT NULL DEFAULT 0
);

CREATE INDEX idx_saga_state_order_id ON ordering.checkout_saga_state(order_id);
CREATE INDEX idx_saga_state_current_state ON ordering.checkout_saga_state(current_state);

-- Outbox table
CREATE TABLE ordering.outbox_messages (
    id bigserial PRIMARY KEY,
    message_id uuid NOT NULL UNIQUE,
    message_type varchar(256) NOT NULL,
    payload text NOT NULL,
    correlation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    retry_count int NOT NULL DEFAULT 0
);

CREATE INDEX idx_ordering_outbox_unsent ON ordering.outbox_messages(created_at) WHERE sent_at IS NULL;
