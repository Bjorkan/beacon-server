-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- name: GetScopeCatalogue :one
SELECT * FROM meshmapper_scope_catalogues WHERE iata = $1 AND url = $2;

-- name: SaveScopeCatalogue :exec
-- One statement commits the validated snapshot and its lookup identities together.
-- Empty arrays insert nothing. NULL payload/checked_at retain last-known-good data
-- after an error or 304. Imported names never replace existing manual metadata.
WITH inserted AS (
    INSERT INTO transport_scopes (name, transport_key, key_fingerprint, imported_only)
    SELECT entry.name, entry.key, entry.fingerprint, TRUE
    FROM (SELECT unnest(@names::text[]) AS name, unnest(@keys::bytea[]) AS key,
                 unnest(@fingerprints::bytea[]) AS fingerprint) AS entry
    ON CONFLICT (name) DO NOTHING
)
INSERT INTO meshmapper_scope_catalogues (iata, url, payload, etag, checked_at, attempted_at, next_attempt, last_error)
VALUES (@iata, @url, sqlc.narg(payload)::jsonb, sqlc.narg(etag)::text,
    sqlc.narg(checked_at)::timestamptz, @attempted_at, @next_attempt, @last_error)
ON CONFLICT (iata, url) DO UPDATE SET
    payload = COALESCE(EXCLUDED.payload, meshmapper_scope_catalogues.payload),
    etag = COALESCE(EXCLUDED.etag, meshmapper_scope_catalogues.etag),
    checked_at = COALESCE(EXCLUDED.checked_at, meshmapper_scope_catalogues.checked_at),
    attempted_at = EXCLUDED.attempted_at,
    next_attempt = EXCLUDED.next_attempt,
    last_error = EXCLUDED.last_error;
