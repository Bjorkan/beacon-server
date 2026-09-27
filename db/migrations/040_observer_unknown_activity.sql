-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Unknown legacy payloads are still recorded packets. Restore their archived
-- counts from the all-payload observer rollup; their discarded radio samples
-- cannot be recovered. Subtract all existing activity (including prior repairs)
-- so migration-journal retries do not double-count or erase newer samples.
INSERT INTO analytics_observer_activity_hourly
(observer_id,payload_type,bucket,observations,airtime_n,snr_n,rssi_n)
SELECT totals.observer_id, -1, totals.bucket, totals.n-COALESCE(activity.n,0),0,0,0
FROM (SELECT observer_id,bucket,SUM(observation_count)::bigint AS n
      FROM analytics_top_observers_by_iata GROUP BY observer_id,bucket) totals
LEFT JOIN (SELECT observer_id,bucket,SUM(observations)::bigint AS n
           FROM analytics_observer_activity_hourly GROUP BY observer_id,bucket) activity
USING (observer_id,bucket)
WHERE totals.n>COALESCE(activity.n,0)
ON CONFLICT (observer_id,payload_type,bucket) DO UPDATE
SET observations=analytics_observer_activity_hourly.observations+EXCLUDED.observations;

DROP MATERIALIZED VIEW mv_observer_activity_hourly;
CREATE OR REPLACE VIEW analytics_live_observer_activity_hourly AS
SELECT
  observer_id,
  COALESCE(payload_type, -1)::smallint AS payload_type,
  date_trunc('hour', heard_at, 'UTC')::timestamptz AS bucket,
  COUNT(*)::bigint AS observations,
  SUM(airtime_ms)::real AS airtime_ms,
  COUNT(airtime_ms)::bigint AS airtime_n,
  SUM(snr)   FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::real   AS snr_sum,
  COUNT(snr) FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::bigint AS snr_n,
  MIN(snr)   FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::real   AS snr_min,
  SUM(rssi)  FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::bigint AS rssi_sum,
  COUNT(rssi) FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::bigint AS rssi_n
FROM packet_observations
WHERE heard_at > NOW() - INTERVAL '30 days'
GROUP BY observer_id, COALESCE(payload_type, -1), date_trunc('hour', heard_at, 'UTC');

CREATE MATERIALIZED VIEW mv_observer_activity_hourly AS
WITH combined AS (
    SELECT observer_id, payload_type, bucket, observations, airtime_ms, airtime_n, snr_sum, snr_n, snr_min, rssi_sum, rssi_n FROM analytics_live_observer_activity_hourly
    UNION ALL
    SELECT observer_id, payload_type, bucket, observations, airtime_ms, airtime_n, snr_sum, snr_n, snr_min, rssi_sum, rssi_n FROM analytics_observer_activity_hourly
    WHERE bucket >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days'
)
SELECT observer_id, payload_type, bucket, SUM(observations)::bigint AS observations, SUM(airtime_ms)::real AS airtime_ms, SUM(airtime_n)::bigint AS airtime_n, SUM(snr_sum)::real AS snr_sum, SUM(snr_n)::bigint AS snr_n, MIN(snr_min)::real AS snr_min, SUM(rssi_sum)::bigint AS rssi_sum, SUM(rssi_n)::bigint AS rssi_n
FROM combined GROUP BY observer_id,payload_type,bucket;

CREATE UNIQUE INDEX idx_analytics_observer_activity_hourly_view ON mv_observer_activity_hourly (observer_id,payload_type,bucket);



-- Preserve the bounded atomic archive, including unknown payload activity.
CREATE OR REPLACE FUNCTION archive_delete_packets(cutoff timestamptz, batch_size integer)
RETURNS bigint LANGUAGE plpgsql VOLATILE AS $$
DECLARE hashes bytea[]; deleted bigint;
BEGIN
    IF batch_size < 1 THEN RAISE EXCEPTION 'batch_size must be positive'; END IF;
    SELECT array_agg(p.packet_hash) INTO hashes FROM (
        SELECT ep.packet_hash FROM packets ep
        WHERE ep.last_heard_at < cutoff
        ORDER BY ep.last_heard_at, ep.packet_hash
        LIMIT batch_size FOR UPDATE OF ep SKIP LOCKED
    ) p;

    WITH expired_observations AS MATERIALIZED (
        SELECT po.* FROM packet_observations po WHERE po.packet_hash = ANY(hashes)
    ),
archived_hourly_iata_stats AS (
    INSERT INTO analytics_hourly_iata_stats (iata, hour, observation_count, unique_packets)
    SELECT iata, hour, observation_count, unique_packets FROM (
SELECT
  iata,
  date_trunc('hour', heard_at, 'UTC')::timestamptz AS hour,
  COUNT(*) AS observation_count,
  COUNT(DISTINCT packet_hash) AS unique_packets
FROM expired_observations
WHERE heard_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days'
GROUP BY iata, date_trunc('hour', heard_at, 'UTC')
    ) batch
    ON CONFLICT (iata,hour) DO UPDATE SET
        observation_count = analytics_hourly_iata_stats.observation_count + EXCLUDED.observation_count,
        unique_packets = analytics_hourly_iata_stats.unique_packets + EXCLUDED.unique_packets
),
archived_payload_breakdown_by_iata AS (
    INSERT INTO analytics_payload_breakdown_by_iata (iata, payload_type, bucket, count)
    SELECT iata, payload_type, bucket, count FROM (
SELECT
  iata,
  payload_type,
  date_trunc('hour', heard_at, 'UTC')::timestamptz AS bucket,
  COUNT(*) AS count
FROM expired_observations
WHERE heard_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days'
  AND payload_type IS NOT NULL
GROUP BY iata, payload_type, date_trunc('hour', heard_at, 'UTC')
    ) batch
    ON CONFLICT (iata,payload_type,bucket) DO UPDATE SET
        count = analytics_payload_breakdown_by_iata.count + EXCLUDED.count
),
archived_top_observers_by_iata AS (
    INSERT INTO analytics_top_observers_by_iata (iata, observer_id, bucket, observation_count, display_name, observer_type)
    SELECT iata, observer_id, bucket, observation_count, display_name, observer_type FROM (
SELECT
  po.iata,
  po.observer_id,
  o.display_name,
  o.observer_type,
  date_trunc('hour', po.heard_at, 'UTC')::timestamptz AS bucket,
  COUNT(*) AS observation_count
FROM expired_observations po
JOIN observers o ON o.id = po.observer_id
WHERE po.heard_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days'
GROUP BY po.iata, po.observer_id, o.display_name, o.observer_type, date_trunc('hour', po.heard_at, 'UTC')
    ) batch
    ON CONFLICT (iata,observer_id,bucket) DO UPDATE SET
        observation_count = analytics_top_observers_by_iata.observation_count + EXCLUDED.observation_count,
        display_name = EXCLUDED.display_name,
        observer_type = EXCLUDED.observer_type
),
archived_top_talkers_by_iata AS (
    INSERT INTO analytics_top_talkers_by_iata (iata, sender_name, bucket, message_count, last_sent)
    SELECT iata, sender_name, bucket, message_count, last_sent FROM (
SELECT
  po.iata,
  cm.sender_name,
  date_trunc('hour', cm.sent_at, 'UTC')::timestamptz AS bucket,
  COUNT(DISTINCT cm.id) AS message_count,
  MAX(cm.sent_at) AS last_sent
FROM channel_messages cm
JOIN expired_observations po ON po.packet_hash = cm.packet_hash
WHERE cm.sender_name IS NOT NULL
  AND cm.sent_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days'
GROUP BY po.iata, cm.sender_name, date_trunc('hour', cm.sent_at, 'UTC')
    ) batch
    ON CONFLICT (iata,sender_name,bucket) DO UPDATE SET
        message_count = analytics_top_talkers_by_iata.message_count + EXCLUDED.message_count,
        last_sent = GREATEST(analytics_top_talkers_by_iata.last_sent, EXCLUDED.last_sent)
),
archived_top_advertisers_by_iata AS (
    INSERT INTO analytics_top_advertisers_by_iata (iata, node_id, bucket, advert_count, flood_advert_count, direct_advert_count, last_heard, name, node_type)
    SELECT iata, node_id, bucket, advert_count, flood_advert_count, direct_advert_count, last_heard, name, node_type FROM (
SELECT
  po.iata,
  n.id AS node_id,
  n.name,
  n.node_type,
  date_trunc('hour', po.heard_at, 'UTC')::timestamptz AS bucket,
  COUNT(DISTINCT p.packet_hash) AS advert_count,
  COUNT(DISTINCT p.packet_hash) FILTER (WHERE p.route_type IN (0, 1)) AS flood_advert_count,
  COUNT(DISTINCT p.packet_hash) FILTER (WHERE p.route_type IN (2, 3)) AS direct_advert_count,
  MAX(po.heard_at) AS last_heard
FROM packets p
JOIN expired_observations po ON po.packet_hash = p.packet_hash
JOIN nodes n ON n.public_key = p.origin_pubkey
WHERE p.payload_type = 4 -- ADVERT
  AND po.heard_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days'
GROUP BY po.iata, n.id, n.name, n.node_type, date_trunc('hour', po.heard_at, 'UTC')
    ) batch
    ON CONFLICT (iata,node_id,bucket) DO UPDATE SET
        advert_count = analytics_top_advertisers_by_iata.advert_count + EXCLUDED.advert_count,
        flood_advert_count = analytics_top_advertisers_by_iata.flood_advert_count + EXCLUDED.flood_advert_count,
        direct_advert_count = analytics_top_advertisers_by_iata.direct_advert_count + EXCLUDED.direct_advert_count,
        last_heard = GREATEST(analytics_top_advertisers_by_iata.last_heard, EXCLUDED.last_heard),
        name = EXCLUDED.name,
        node_type = EXCLUDED.node_type
),
archived_observer_activity_hourly AS (
    INSERT INTO analytics_observer_activity_hourly (observer_id, payload_type, bucket, observations, airtime_ms, airtime_n, snr_sum, snr_n, snr_min, rssi_sum, rssi_n)
    SELECT observer_id, payload_type, bucket, observations, airtime_ms, airtime_n, snr_sum, snr_n, snr_min, rssi_sum, rssi_n FROM (
SELECT
  observer_id,
  COALESCE(payload_type, -1)::smallint AS payload_type,
  date_trunc('hour', heard_at, 'UTC')::timestamptz AS bucket,
  COUNT(*)::bigint AS observations,
  SUM(airtime_ms)::real AS airtime_ms,
  COUNT(airtime_ms)::bigint AS airtime_n,
  SUM(snr)   FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::real   AS snr_sum,
  COUNT(snr) FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::bigint AS snr_n,
  MIN(snr)   FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::real   AS snr_min,
  SUM(rssi)  FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::bigint AS rssi_sum,
  COUNT(rssi) FILTER (WHERE NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0))::bigint AS rssi_n
FROM expired_observations
WHERE heard_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days'
GROUP BY observer_id, COALESCE(payload_type, -1), date_trunc('hour', heard_at, 'UTC')
    ) batch
    ON CONFLICT (observer_id,payload_type,bucket) DO UPDATE SET
        observations = analytics_observer_activity_hourly.observations + EXCLUDED.observations,
        airtime_ms = CASE WHEN analytics_observer_activity_hourly.airtime_ms IS NULL AND EXCLUDED.airtime_ms IS NULL THEN NULL ELSE COALESCE(analytics_observer_activity_hourly.airtime_ms, 0) + COALESCE(EXCLUDED.airtime_ms, 0) END,
        airtime_n = analytics_observer_activity_hourly.airtime_n + EXCLUDED.airtime_n,
        snr_sum = CASE WHEN analytics_observer_activity_hourly.snr_sum IS NULL AND EXCLUDED.snr_sum IS NULL THEN NULL ELSE COALESCE(analytics_observer_activity_hourly.snr_sum, 0) + COALESCE(EXCLUDED.snr_sum, 0) END,
        snr_n = analytics_observer_activity_hourly.snr_n + EXCLUDED.snr_n,
        snr_min = LEAST(analytics_observer_activity_hourly.snr_min, EXCLUDED.snr_min),
        rssi_sum = CASE WHEN analytics_observer_activity_hourly.rssi_sum IS NULL AND EXCLUDED.rssi_sum IS NULL THEN NULL ELSE COALESCE(analytics_observer_activity_hourly.rssi_sum, 0) + COALESCE(EXCLUDED.rssi_sum, 0) END,
        rssi_n = analytics_observer_activity_hourly.rssi_n + EXCLUDED.rssi_n
),
archived_signal_stats_hourly AS (
    INSERT INTO analytics_signal_stats_hourly (iata, hour, kind, snr_bin, rssi_bin, receptions, snr_samples, snr_sum, rssi_samples, rssi_sum)
    SELECT iata, hour, kind, snr_bin, rssi_bin, receptions, snr_samples, snr_sum, rssi_samples, rssi_sum FROM (
WITH samples AS (
    SELECT iata, date_trunc('hour', heard_at, 'UTC') AS hour,
           CASE WHEN NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0)
                     AND snr > '-Infinity'::real AND snr < 'Infinity'::real
                THEN snr::double precision END AS snr,
           CASE WHEN NOT (COALESCE(rssi, 0) = 0 AND COALESCE(snr, 0) = 0)
                THEN rssi::double precision END AS rssi
    FROM expired_observations
    WHERE heard_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '720 hours'

), binned AS (
    SELECT *, width_bucket(snr, -30, 30, 12) AS snr_bin,
              width_bucket(rssi, -140, 0, 14) AS rssi_bin
    FROM samples
)
SELECT iata, hour, grouping(snr_bin, rssi_bin)::integer AS kind,
       COALESCE(snr_bin, -1)::integer AS snr_bin,
       COALESCE(rssi_bin, -1)::integer AS rssi_bin,
       count(*)::bigint AS receptions,
       count(snr)::bigint AS snr_samples,
       COALESCE(sum(snr), 0)::double precision AS snr_sum,
       count(rssi)::bigint AS rssi_samples,
       COALESCE(sum(rssi), 0)::double precision AS rssi_sum
FROM binned
GROUP BY GROUPING SETS ((iata, hour), (iata, hour, snr_bin), (iata, hour, rssi_bin))
    ) batch
    ON CONFLICT (iata,hour,kind,snr_bin,rssi_bin) DO UPDATE SET
        receptions = analytics_signal_stats_hourly.receptions + EXCLUDED.receptions,
        snr_samples = analytics_signal_stats_hourly.snr_samples + EXCLUDED.snr_samples,
        snr_sum = analytics_signal_stats_hourly.snr_sum + EXCLUDED.snr_sum,
        rssi_samples = analytics_signal_stats_hourly.rssi_samples + EXCLUDED.rssi_samples,
        rssi_sum = analytics_signal_stats_hourly.rssi_sum + EXCLUDED.rssi_sum
),
archived_path_stats_hourly AS (
    INSERT INTO analytics_path_stats_hourly (iata, hour, category, hash_bytes, entries, receptions)
    SELECT iata, hour, category, hash_bytes, entries, receptions FROM (
WITH classified AS (
    SELECT iata, heard_at, hash_size, hop_count,
           CASE WHEN payload_type = 9 THEN 2
                -- Match meshcore-go IsValidPathLen (1/2/3-byte hashes, max 64 path bytes).
                WHEN payload_type IS NULL OR payload_type NOT BETWEEN 0 AND 15
                  OR NOT (path_length_byte BETWEEN 0 AND 191
                    AND hash_size BETWEEN 1 AND 3 AND hop_count BETWEEN 0 AND 63
                    AND hash_size = (path_length_byte >> 6) + 1
                    AND hop_count = (path_length_byte & 63)
                    AND hash_size::integer * hop_count::integer <= 64
                    AND COALESCE(octet_length(path_bytes), 0) = hash_size::integer * hop_count::integer)
                THEN 3
                WHEN hop_count = 0 THEN 1
                ELSE 0 END::integer AS category
    FROM expired_observations
    WHERE heard_at >= date_trunc('hour', NOW(), 'UTC') - INTERVAL '720 hours'

), buckets AS (
    SELECT iata, date_trunc('hour', heard_at, 'UTC') AS hour, category,
           CASE WHEN category = 0 THEN hash_size ELSE 0 END::integer AS hash_bytes,
           CASE WHEN category = 0 THEN hop_count ELSE 0 END::integer AS entries
    FROM classified
)
SELECT iata, hour, category, hash_bytes, entries, count(*)::bigint AS receptions
FROM buckets GROUP BY iata, hour, category, hash_bytes, entries
    ) batch
    ON CONFLICT (iata,hour,category,hash_bytes,entries) DO UPDATE SET
        receptions = analytics_path_stats_hourly.receptions + EXCLUDED.receptions
)
    DELETE FROM packets WHERE packet_hash = ANY(hashes);
    GET DIAGNOSTICS deleted = ROW_COUNT;

    -- Once per completed cleanup, including when there were no raw packets to delete.
    IF deleted < batch_size THEN
        DELETE FROM analytics_hourly_iata_stats WHERE hour < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
        DELETE FROM analytics_payload_breakdown_by_iata WHERE bucket < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
        DELETE FROM analytics_top_observers_by_iata WHERE bucket < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
        DELETE FROM analytics_top_talkers_by_iata WHERE bucket < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
        DELETE FROM analytics_top_advertisers_by_iata WHERE bucket < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
        DELETE FROM analytics_observer_activity_hourly WHERE bucket < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
        DELETE FROM analytics_signal_stats_hourly WHERE hour < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
        DELETE FROM analytics_path_stats_hourly WHERE hour < date_trunc('hour', NOW(), 'UTC') - INTERVAL '30 days';
    END IF;
    RETURN deleted;
END $$;
