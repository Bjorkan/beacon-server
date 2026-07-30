-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Supports the observer /neighbors MQTT topic: a periodic report listing the
-- OTA-configured region scope of the observer itself ("self") and of each
-- zero-hop neighbor the observer was able to query.
--
-- NOTE: these are plaintext OTA "region scope" strings (comma-separated,
-- e.g. "*" or "US,CA") read from device config over the air. They are
-- unrelated to the transport_scopes / observer_scopes / default_scope_id
-- machinery elsewhere in this schema, which matches encrypted transport
-- packets to named routing scopes by key fingerprint. Deliberately named
-- differently (region_scope, not scope) to avoid confusion between the two.

ALTER TABLE observers ADD COLUMN region_scope TEXT;

-- Per-neighbor-edge, not per-node: the OTA scope query is answered by
-- whichever node reports it, in the context of a specific observer's
-- zero-hop neighbor list, so it lives alongside snr on node_neighbors.
ALTER TABLE node_neighbors ADD COLUMN region_scope TEXT;
