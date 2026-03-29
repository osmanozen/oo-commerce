-- Catalog Service Database Schema
-- Schema: catalog

CREATE SCHEMA IF NOT EXISTS catalog;

-- Categories table
CREATE TABLE catalog.categories (
    id uuid PRIMARY KEY,
    name_value varchar(100) NOT NULL UNIQUE,
    description varchar(500),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0
);

CREATE INDEX idx_categories_name ON catalog.categories(name_value);

-- Products table
CREATE TABLE catalog.products (
    id uuid PRIMARY KEY,
    name_value varchar(200) NOT NULL,
    description varchar(2000),
    sku_value varchar(100) NOT NULL UNIQUE,
    price_amount numeric(18,2) NOT NULL,
    price_currency varchar(3) NOT NULL,
    category_id uuid NOT NULL REFERENCES catalog.categories(id) ON DELETE CASCADE,
    image_url varchar(500),
    average_rating numeric(3,2),
    review_count int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0
);

CREATE INDEX idx_products_category_id ON catalog.products(category_id);
CREATE INDEX idx_products_sku ON catalog.products(sku_value);
CREATE INDEX idx_products_price ON catalog.products(price_amount);
CREATE INDEX idx_products_rating ON catalog.products(average_rating DESC NULLS LAST);
CREATE INDEX idx_products_created_at ON catalog.products(created_at DESC);

-- Product images table
CREATE TABLE catalog.product_images (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES catalog.products(id) ON DELETE CASCADE,
    url varchar(500) NOT NULL,
    alt_text varchar(200),
    display_order int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_product_images_product_id ON catalog.product_images(product_id);
CREATE INDEX idx_product_images_display_order ON catalog.product_images(product_id, display_order);

-- Outbox messages table (per-service)
CREATE TABLE catalog.outbox_messages (
    id bigserial PRIMARY KEY,
    message_id uuid NOT NULL UNIQUE,
    message_type varchar(256) NOT NULL,
    payload text NOT NULL,
    correlation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    retry_count int NOT NULL DEFAULT 0
);

CREATE INDEX idx_outbox_unsent ON catalog.outbox_messages(created_at) WHERE sent_at IS NULL;
