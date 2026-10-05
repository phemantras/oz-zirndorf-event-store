-- Plain CRUD only: sorting, validation and the effective period live in the
-- core (AD-2).

-- name: ListEvents :many
SELECT id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
       source_description, source_url, note, effective_start, effective_end, title_key
FROM events;

-- name: ListEventsOverlapping :many
-- The one filter predicate of AD-16 on the stored effective period; a NULL
-- lo leaves the period open at the start, a NULL hi open-ended. The core
-- passes lo and hi as parameters.
SELECT id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
       source_description, source_url, note, effective_start, effective_end, title_key
FROM events
WHERE (sqlc.narg(lo)::timestamptz IS NULL OR effective_end > sqlc.narg(lo))
  AND (sqlc.narg(hi)::timestamptz IS NULL OR effective_start < sqlc.narg(hi));

-- name: GetEvent :one
SELECT id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
       source_description, source_url, note, effective_start, effective_end, title_key
FROM events
WHERE id = $1;

-- name: CreateEvent :one
INSERT INTO events (title, title_key, type, location_id, start_date, start_time, end_date, end_time, all_day,
                    source_description, source_url, note, effective_start, effective_end)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
          source_description, source_url, note, effective_start, effective_end, title_key;

-- name: UpdateEvent :one
UPDATE events
SET title = $2, title_key = $3, type = $4, location_id = $5, start_date = $6, start_time = $7,
    end_date = $8, end_time = $9, all_day = $10, source_description = $11,
    source_url = $12, note = $13, effective_start = $14, effective_end = $15,
    archived_at = NULL
WHERE id = $1
RETURNING id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
          source_description, source_url, note, effective_start, effective_end, title_key;

-- name: FindEventsByDuplicateKey :many
SELECT id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
       source_description, source_url, note, effective_start, effective_end, title_key
FROM events
WHERE title_key = $1 AND start_date = $2 AND location_id = $3;

-- name: UpdateEventDerived :execrows
UPDATE events
SET effective_start = $2, effective_end = $3, title_key = $4, archived_at = NULL
WHERE id = $1;

-- name: MarkEventsArchived :execrows
-- One statement, so an event that a concurrent save makes active again is
-- checked against its new effective_end and stays unmarked. The core passes
-- now as a parameter (AD-2).
UPDATE events
SET archived_at = @now
WHERE effective_end <= @now AND archived_at IS NULL;

-- name: ListTimetableEntries :many
SELECT id, event_id, description, date, start_time, end_time
FROM timetable_entries;

-- name: ListTimetableEntriesOfEvent :many
SELECT id, event_id, description, date, start_time, end_time
FROM timetable_entries
WHERE event_id = $1;

-- name: ListTimetableEntriesOfEvents :many
SELECT id, event_id, description, date, start_time, end_time
FROM timetable_entries
WHERE event_id = ANY(@event_ids::uuid[]);

-- name: DeleteTimetableEntriesOfEvent :exec
DELETE FROM timetable_entries
WHERE event_id = $1;

-- name: CreateTimetableEntry :one
INSERT INTO timetable_entries (event_id, description, date, start_time, end_time)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, event_id, description, date, start_time, end_time;

-- name: DeleteEvent :execrows
DELETE FROM events
WHERE id = $1;

-- name: CountEventsByLocation :one
SELECT count(*)
FROM events
WHERE location_id = $1;
