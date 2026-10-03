-- +goose Up
-- Title key of events (Story 1.10). The core derives title_key with
-- NormalizeKey(title) and finds suspected duplicates by title_key,
-- start_date and location_id; the database only stores and compares it
-- (AD-2, AD-11). It is not unique: a confirmed duplicate may share it.
-- Expand step (AD-17): the default fills existing rows and keeps code
-- without title_key working after a rollback; RecomputeDerived writes the
-- real keys at the next start.
-- Once applied, this file must never change (AD-17).
ALTER TABLE events ADD COLUMN title_key text NOT NULL DEFAULT '';

-- The duplicate check looks events up by all three keys.
CREATE INDEX events_duplicate_key_idx ON events (title_key, start_date, location_id);
