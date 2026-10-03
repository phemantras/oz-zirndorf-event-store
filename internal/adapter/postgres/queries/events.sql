-- Plain CRUD only: sorting, validation and the effective period live in the
-- core (AD-2).

-- name: ListEvents :many
SELECT id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
       source_description, source_url, note, effective_start, effective_end
FROM events;

-- name: GetEvent :one
SELECT id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
       source_description, source_url, note, effective_start, effective_end
FROM events
WHERE id = $1;

-- name: CreateEvent :one
INSERT INTO events (title, type, location_id, start_date, start_time, end_date, end_time, all_day,
                    source_description, source_url, note, effective_start, effective_end)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
          source_description, source_url, note, effective_start, effective_end;

-- name: UpdateEvent :one
UPDATE events
SET title = $2, type = $3, location_id = $4, start_date = $5, start_time = $6,
    end_date = $7, end_time = $8, all_day = $9, source_description = $10,
    source_url = $11, note = $12, effective_start = $13, effective_end = $14
WHERE id = $1
RETURNING id, title, type, location_id, start_date, start_time, end_date, end_time, all_day,
          source_description, source_url, note, effective_start, effective_end;

-- name: UpdateEventPeriod :execrows
UPDATE events
SET effective_start = $2, effective_end = $3
WHERE id = $1;

-- name: ListTimetableEntries :many
SELECT id, event_id, description, date, start_time, end_time
FROM timetable_entries;

-- name: ListTimetableEntriesOfEvent :many
SELECT id, event_id, description, date, start_time, end_time
FROM timetable_entries
WHERE event_id = $1;

-- name: DeleteTimetableEntriesOfEvent :exec
DELETE FROM timetable_entries
WHERE event_id = $1;

-- name: CreateTimetableEntry :one
INSERT INTO timetable_entries (event_id, description, date, start_time, end_time)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, event_id, description, date, start_time, end_time;
