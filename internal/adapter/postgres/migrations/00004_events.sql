-- +goose Up
-- Events (Story 1.7). The core validates every field and derives
-- effective_start and effective_end from the stored times; the database
-- only stores them (AD-2, AD-16). An event refers to its location, which
-- cannot be deleted while events refer to it. An empty time is NULL and
-- means unknown, never 00:00.
-- Once applied, this file must never change (AD-17).
CREATE TABLE events (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    title text NOT NULL,
    type text NOT NULL,
    location_id uuid NOT NULL
        CONSTRAINT events_location_id_fkey REFERENCES locations (id) ON DELETE RESTRICT,
    start_date date NOT NULL,
    start_time time,
    end_date date,
    end_time time,
    all_day boolean NOT NULL,
    source_description text NOT NULL,
    source_url text,
    note text,
    effective_start timestamptz NOT NULL,
    effective_end timestamptz NOT NULL
);

-- The foreign key check on deleting a location looks events up by location.
CREATE INDEX events_location_id_idx ON events (location_id);
