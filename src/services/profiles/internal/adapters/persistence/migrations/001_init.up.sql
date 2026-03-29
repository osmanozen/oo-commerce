-- Profiles Service Database Schema
-- Schema: profiles

CREATE SCHEMA IF NOT EXISTS profiles;

-- User profiles table
CREATE TABLE profiles.user_profiles (
    id uuid PRIMARY KEY,
    user_id varchar(100) NOT NULL UNIQUE,
    email varchar(200) NOT NULL,
    first_name varchar(100) NOT NULL,
    last_name varchar(100) NOT NULL,
    phone varchar(20),
    avatar_url varchar(500),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 0
);

CREATE INDEX idx_profiles_user_id ON profiles.user_profiles(user_id);
CREATE INDEX idx_profiles_email ON profiles.user_profiles(email);

-- Addresses table
CREATE TABLE profiles.addresses (
    id uuid PRIMARY KEY,
    profile_id uuid NOT NULL REFERENCES profiles.user_profiles(id) ON DELETE CASCADE,
    label varchar(50),
    first_name varchar(100) NOT NULL,
    last_name varchar(100) NOT NULL,
    street varchar(200) NOT NULL,
    city varchar(100) NOT NULL,
    state varchar(100),
    zip_code varchar(20),
    country varchar(100) NOT NULL,
    phone varchar(20),
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_addresses_profile_id ON profiles.addresses(profile_id);

-- Outbox table
CREATE TABLE profiles.outbox_messages (
    id bigserial PRIMARY KEY,
    message_id uuid NOT NULL UNIQUE,
    message_type varchar(256) NOT NULL,
    payload text NOT NULL,
    correlation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    retry_count int NOT NULL DEFAULT 0
);

CREATE INDEX idx_profiles_outbox_unsent ON profiles.outbox_messages(created_at) WHERE sent_at IS NULL;
