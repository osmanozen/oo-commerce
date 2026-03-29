-- Reviews Service Database Schema
-- Schema: reviews

CREATE SCHEMA IF NOT EXISTS reviews;

-- Reviews table
CREATE TABLE reviews.reviews (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL,
    user_id uuid NOT NULL,
    rating int NOT NULL CHECK (rating BETWEEN 1 AND 5),
    review_text varchar(1000) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0,
    CONSTRAINT uq_reviews_user_product UNIQUE (user_id, product_id)
);

CREATE INDEX idx_reviews_product_id ON reviews.reviews(product_id);
CREATE INDEX idx_reviews_user_id ON reviews.reviews(user_id);
CREATE INDEX idx_reviews_rating ON reviews.reviews(product_id, rating);
CREATE INDEX idx_reviews_created_at ON reviews.reviews(created_at DESC);

-- Materialized view for rating statistics (refreshed on review changes)
CREATE MATERIALIZED VIEW reviews.rating_stats AS
SELECT
    product_id,
    ROUND(AVG(rating)::numeric, 2) AS average_rating,
    COUNT(*) AS review_count,
    COUNT(*) FILTER (WHERE rating = 5) AS five_star,
    COUNT(*) FILTER (WHERE rating = 4) AS four_star,
    COUNT(*) FILTER (WHERE rating = 3) AS three_star,
    COUNT(*) FILTER (WHERE rating = 2) AS two_star,
    COUNT(*) FILTER (WHERE rating = 1) AS one_star
FROM reviews.reviews
GROUP BY product_id;

CREATE UNIQUE INDEX idx_rating_stats_product_id ON reviews.rating_stats(product_id);

-- Outbox table
CREATE TABLE reviews.outbox_messages (
    id bigserial PRIMARY KEY,
    message_id uuid NOT NULL UNIQUE,
    message_type varchar(256) NOT NULL,
    payload text NOT NULL,
    correlation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    retry_count int NOT NULL DEFAULT 0
);

CREATE INDEX idx_reviews_outbox_unsent ON reviews.outbox_messages(created_at) WHERE sent_at IS NULL;
