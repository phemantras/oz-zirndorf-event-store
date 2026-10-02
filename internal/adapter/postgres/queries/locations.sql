-- Plain CRUD only: sorting, normalization and validation live in the core.
-- The column address is left over from before Story 1.12 and is neither read
-- nor written (AD-17), so every query names its columns.

-- name: ListLocations :many
SELECT id, name, name_key, street, postal_code, city, latitude, longitude, precision, note
FROM locations;

-- name: GetLocation :one
SELECT id, name, name_key, street, postal_code, city, latitude, longitude, precision, note
FROM locations
WHERE id = $1;

-- name: FindLocationByNameKey :one
SELECT id, name, name_key, street, postal_code, city, latitude, longitude, precision, note
FROM locations
WHERE name_key = $1;

-- name: CreateLocation :one
INSERT INTO locations (name, name_key, street, postal_code, city, latitude, longitude, precision, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, name, name_key, street, postal_code, city, latitude, longitude, precision, note;

-- name: UpdateLocation :one
UPDATE locations
SET name = $2, name_key = $3, street = $4, postal_code = $5, city = $6,
    latitude = $7, longitude = $8, precision = $9, note = $10
WHERE id = $1
RETURNING id, name, name_key, street, postal_code, city, latitude, longitude, precision, note;
