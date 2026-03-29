-- Inventory Service Database Schema
-- Schema: inventory

CREATE SCHEMA IF NOT EXISTS inventory;

-- Stock items table
CREATE TABLE inventory.stock_items (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL UNIQUE,
    sku varchar(100) NOT NULL,
    total_quantity int NOT NULL DEFAULT 0,
    low_stock_level int NOT NULL DEFAULT 10,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0
);

CREATE INDEX idx_stock_items_product_id ON inventory.stock_items(product_id);
CREATE INDEX idx_stock_items_sku ON inventory.stock_items(sku);

-- Stock reservations table (TTL-based)
CREATE TABLE inventory.stock_reservations (
    id uuid PRIMARY KEY,
    stock_item_id uuid NOT NULL REFERENCES inventory.stock_items(id) ON DELETE CASCADE,
    order_id uuid NOT NULL,
    correlation_id uuid NOT NULL,
    quantity int NOT NULL CHECK (quantity > 0),
    reserved_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    is_committed boolean NOT NULL DEFAULT false,
    is_released boolean NOT NULL DEFAULT false
);

CREATE INDEX idx_reservations_stock_item ON inventory.stock_reservations(stock_item_id);
CREATE INDEX idx_reservations_order_id ON inventory.stock_reservations(order_id);
CREATE INDEX idx_reservations_correlation ON inventory.stock_reservations(correlation_id);
CREATE INDEX idx_reservations_active ON inventory.stock_reservations(expires_at)
    WHERE is_committed = false AND is_released = false;

-- Stock adjustments table (audit trail)
CREATE TABLE inventory.stock_adjustments (
    id uuid PRIMARY KEY,
    stock_item_id uuid NOT NULL REFERENCES inventory.stock_items(id) ON DELETE CASCADE,
    adjustment_type int NOT NULL,
    quantity int NOT NULL,
    reason varchar(500),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by varchar(100) NOT NULL
);

CREATE INDEX idx_adjustments_stock_item ON inventory.stock_adjustments(stock_item_id);
CREATE INDEX idx_adjustments_created_at ON inventory.stock_adjustments(created_at DESC);

-- Outbox table
CREATE TABLE inventory.outbox_messages (
    id bigserial PRIMARY KEY,
    message_id uuid NOT NULL UNIQUE,
    message_type varchar(256) NOT NULL,
    payload text NOT NULL,
    correlation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    retry_count int NOT NULL DEFAULT 0
);

CREATE INDEX idx_inventory_outbox_unsent ON inventory.outbox_messages(created_at) WHERE sent_at IS NULL;
