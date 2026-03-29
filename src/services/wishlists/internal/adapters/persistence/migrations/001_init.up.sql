-- Wishlists Service Database Schema
-- Schema: wishlists

CREATE SCHEMA IF NOT EXISTS wishlists;

-- Wishlist items table
CREATE TABLE wishlists.wishlist_items (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    product_id uuid NOT NULL,
    added_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0,
    CONSTRAINT uq_wishlist_user_product UNIQUE (user_id, product_id)
);

CREATE INDEX idx_wishlist_user_id ON wishlists.wishlist_items(user_id);
CREATE INDEX idx_wishlist_product_id ON wishlists.wishlist_items(product_id);
CREATE INDEX idx_wishlist_added_at ON wishlists.wishlist_items(user_id, added_at DESC);
