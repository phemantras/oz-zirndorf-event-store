-- +goose Up
-- Timetable entries (Story 1.9). An entry is a value object of its event
-- and only ever replaced as a whole list together with the event, in one
-- transaction (AD-15). The core validates the entries and checks that they
-- lie within the event; the database only stores them. Deleting an event
-- deletes its entries. An empty time is NULL and means unknown, never
-- 00:00.
-- Once applied, this file must never change (AD-17).
CREATE TABLE timetable_entries (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    event_id uuid NOT NULL
        CONSTRAINT timetable_entries_event_id_fkey REFERENCES events (id) ON DELETE CASCADE,
    description text NOT NULL,
    date date NOT NULL,
    start_time time,
    end_time time
);

-- Reading and replacing the timetable of one event, and the cascade on
-- deleting an event, look entries up by event.
CREATE INDEX timetable_entries_event_id_idx ON timetable_entries (event_id);
