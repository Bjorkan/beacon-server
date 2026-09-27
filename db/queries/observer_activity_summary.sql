-- name: GetObserverActivityLiveSummary :one
-- Two indexed ranges, bounded to one observer; no legacy presence counters.
WITH latest AS (
 SELECT heard_at FROM packet_observations
 WHERE observer_id = @observer_id::uuid AND heard_at <= @generated_at::timestamptz
 ORDER BY heard_at DESC LIMIT 1
), hourly AS (
 SELECT COUNT(*)::bigint AS n FROM packet_observations
 WHERE observer_id = @observer_id::uuid
 AND heard_at >= @hour_start::timestamptz AND heard_at < @hour_end::timestamptz
)
SELECT (SELECT heard_at FROM latest)::timestamptz AS latest_recorded_at,
 hourly.n AS last_complete_hour FROM hourly;
