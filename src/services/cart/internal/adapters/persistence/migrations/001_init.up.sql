-- Cart Service Database Schema
-- Schema: cart

CREATE SCHEMA IF NOT EXISTS cart;

-- Carts table
CREATE TABLE cart.carts (
    id uuid PRIMARY KEY,
    user_id varchar(100),
    guest_id varchar(100),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_buyer CHECK (user_id IS NOT NULL OR guest_id IS NOT NULL)
);

CREATE UNIQUE INDEX idx_carts_user_id ON cart.carts(user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX idx_carts_guest_id ON cart.carts(guest_id) WHERE guest_id IS NOT NULL;
CREATE INDEX idx_carts_updated_at ON cart.carts(updated_at);

-- Cart items table (denormalized product data)
CREATE TABLE cart.cart_items (
    id uuid PRIMARY KEY,
    cart_id uuid NOT NULL REFERENCES cart.carts(id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    product_name varchar(200) NOT NULL,
    image_url varchar(500),
    unit_price numeric(18,2) NOT NULL,
    currency varchar(3) NOT NULL,
    quantity int NOT NULL CHECK (quantity > 0),
    added_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_cart_product UNIQUE (cart_id, product_id)
);

CREATE INDEX idx_cart_items_cart_id ON cart.cart_items(cart_id);

-- Outbox table
CREATE TABLE cart.outbox_messages (
    id bigserial PRIMARY KEY,
    message_id uuid NOT NULL UNIQUE,
    message_type varchar(256) NOT NULL,
    payload text NOT NULL,
    correlation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    retry_count int NOT NULL DEFAULT 0
);

CREATE INDEX idx_cart_outbox_unsent ON cart.outbox_messages(created_at) WHERE sent_at IS NULL;
