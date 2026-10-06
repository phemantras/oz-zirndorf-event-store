-- +goose Up
-- Import key of events (Story 3.2). An import entry may name its event in
-- the import source by importKey; the import preview finds the stored event
-- by this key to classify the entry as update or unchanged. Only the import
-- commit (Story 3.3) sets it; saving an event in the admin keeps it, and the
-- public read form never shows it (AD-6, AD-14).
-- Expand step (AD-17): the column is nullable without default, so existing
-- rows have no key and code without import_key keeps working after a
-- rollback.
-- Once applied, this file must never change (AD-17).
ALTER TABLE events ADD COLUMN import_key text;

-- A set key names exactly one event; any number of events have none.
CREATE UNIQUE INDEX events_import_key_idx ON events (import_key) WHERE import_key IS NOT NULL;
