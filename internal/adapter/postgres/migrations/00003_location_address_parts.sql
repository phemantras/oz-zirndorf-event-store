-- +goose Up
-- Address parts (Story 1.12), expand step of AD-17: street, postal code and
-- city replace the free-text address. Existing rows keep their address and
-- get empty parts; nothing is split in SQL, the core validates the parts on
-- the next save. The defaults keep the code before this story running after
-- a rollback: new rows get address '' instead of NULL, which the old code
-- could not scan. The contract step (Story 1.7) drops address and the
-- defaults.
-- Once applied, this file must never change (AD-17).
ALTER TABLE locations
    ADD COLUMN street text NOT NULL DEFAULT '',
    ADD COLUMN postal_code text NOT NULL DEFAULT '',
    ADD COLUMN city text NOT NULL DEFAULT '',
    ALTER COLUMN address DROP NOT NULL,
    ALTER COLUMN address SET DEFAULT '';
