-- +goose Up
-- Locations (Story 1.4). The core derives name_key with NormalizeKey and
-- validates every field; the database only enforces uniqueness of the key
-- (AD-2, AD-11). Once applied, this file must never change (AD-17).
CREATE TABLE locations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL,
    name_key text NOT NULL CONSTRAINT locations_name_key_unique UNIQUE,
    address text NOT NULL,
    latitude double precision NOT NULL,
    longitude double precision NOT NULL,
    precision text NOT NULL,
    note text
);
