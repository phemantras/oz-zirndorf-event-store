-- name: TakeWriteLock :exec
-- Waits for the transaction-level advisory lock with the given key and
-- holds it until the transaction ends, so write transactions run one after
-- the other.
SELECT pg_advisory_xact_lock(@lock_key::bigint);
