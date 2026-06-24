-- 006_fix_observation_dedup_key.sql
--
-- The previous unique constraint on (packet_hash, observer_id, heard_at) was
-- too permissive: heard_at is stamped at ingest time (time.Now()), so MQTT
-- redeliveries of the same packet arrive with slightly different timestamps
-- and bypass dedup, producing duplicate observations for the same observer.
--
-- An observer either heard a packet or they didn't. Drop heard_at from the
-- constraint so that (packet_hash, observer_id) is the dedup key.

-- Remove duplicate observations, keeping the earliest (lowest id) per (packet_hash, observer_id).
DELETE FROM packet_observations
WHERE id NOT IN (
    SELECT MIN(id)
    FROM packet_observations
    GROUP BY packet_hash, observer_id
);

ALTER TABLE packet_observations
    DROP CONSTRAINT packet_observations_packet_hash_observer_id_heard_at_key;

ALTER TABLE packet_observations
    ADD CONSTRAINT packet_observations_packet_hash_observer_id_key
    UNIQUE (packet_hash, observer_id);
