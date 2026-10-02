-- Plain CRUD only: sorting, normalization and validation live in the core.

-- name: ListLocations :many
SELECT id, name, name_key, address, latitude, longitude, precision, note
FROM locations;

-- name: GetLocation :one
SELECT id, name, name_key, address, latitude, longitude, precision, note
FROM locations
WHERE id = $1;

-- name: FindLocationByNameKey :one
SELECT id, name, name_key, address, latitude, longitude, precision, note
FROM locations
WHERE name_key = $1;

-- name: CreateLocation :one
INSERT INTO locations (name, name_key, address, latitude, longitude, precision, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, name_key, address, latitude, longitude, precision, note;

-- name: UpdateLocation :one
UPDATE locations
SET name = $2, name_key = $3, address = $4, latitude = $5, longitude = $6, precision = $7, note = $8
WHERE id = $1
RETURNING id, name, name_key, address, latitude, longitude, precision, note;
