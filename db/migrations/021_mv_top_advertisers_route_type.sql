-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Splits mv_top_advertisers_by_iata's advert_count into flood- and direct-routed counts,
-- alongside the existing combined total, so /stats/top-advertisers can report how a node is
-- actually being heard rather than only a single blended number.
--
-- Flood  = route_type 0 (transport_flood) or 1 (flood)  -- broadcast, no prior path needed.
-- Direct = route_type 2 (direct) or 3 (transport_direct) -- routed along a known path.

DROP MATERIALIZED VIEW mv_top_advertisers_by_iata;

CREATE MATERIALIZED VIEW mv_top_advertisers_by_iata AS
SELECT
  po.iata,
  n.id AS node_id,
  n.name,
  n.node_type,
  date_trunc('hour', po.heard_at)::timestamptz AS bucket,
  COUNT(DISTINCT p.packet_hash) AS advert_count,
  COUNT(DISTINCT p.packet_hash) FILTER (WHERE p.route_type IN (0, 1)) AS flood_advert_count,
  COUNT(DISTINCT p.packet_hash) FILTER (WHERE p.route_type IN (2, 3)) AS direct_advert_count,
  MAX(po.heard_at) AS last_heard
FROM packets p
JOIN packet_observations po ON po.packet_hash = p.packet_hash
JOIN nodes n ON n.public_key = p.origin_pubkey
WHERE p.payload_type = 4 -- ADVERT
  AND po.heard_at > NOW() - INTERVAL '30 days'
GROUP BY po.iata, n.id, n.name, n.node_type, date_trunc('hour', po.heard_at);

CREATE UNIQUE INDEX idx_mv_top_advertisers
  ON mv_top_advertisers_by_iata(iata, node_id, bucket);
