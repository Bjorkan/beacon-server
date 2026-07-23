-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Cleans up observer_telemetry rows written from /status messages whose
-- "stats" object was missing or renamed. status.go's json.Unmarshal
-- silently tolerated the missing object, leaving the stats struct at Go
-- zero-values, and the row was inserted unconditionally from there --
-- uptime_secs=0, battery_mv=0, noise_floor=0, tx/rx_air_secs=0. With the
-- hourly ON CONFLICT (observer_id, reported_at) DO NOTHING dedup,
-- whichever message landed first per hour won, so these zero rows
-- regularly became the hour's telemetry and corrupted the 24h/7d/30d
-- aggregates (AVG(noise_floor_db), AVG(battery_voltage_mv), MAX-MIN
-- airtime).
--
-- Safe because a running observer never reports uptime_seconds == 0;
-- this is the same signal now used by status.go to skip the insert going
-- forward (see internal/ingest/status.go handleStatus).

DELETE FROM observer_telemetry WHERE uptime_seconds = 0;
