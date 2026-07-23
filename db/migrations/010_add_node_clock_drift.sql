-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Stores the device-reported clock drift measured from each ADVERT: the signed
-- advert body already carries the device's own wall-clock timestamp
-- (advert.Timestamp, epoch seconds), which the server previously decoded and
-- then discarded before it reached the nodes table. This column stores
-- (device timestamp - server receive time) in seconds, computed at the same
-- moment nodes.last_advert_at is set to NOW(), so last_advert_at doubles as
-- the "when was this drift measured" timestamp with no separate column
-- needed. NULL until a node's first successfully-verified advert is upserted.
--
-- +ve = device clock ahead of the server; -ve = behind. Surfaced on the node
-- API for repeaters/room servers only (nodeType 2/3) -- see internal/api/nodes.go.

ALTER TABLE nodes ADD COLUMN device_clock_drift_seconds INTEGER;
