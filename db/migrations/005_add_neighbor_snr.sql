-- Adds an optional SNR reading to node_neighbors. Most rows (advert
-- path-hop-derived topology) will leave this null; it's populated only
-- when the neighbor relationship comes with an actual signal reading
-- (e.g. observer-detected via DISCOVER_RESP).
ALTER TABLE node_neighbors ADD COLUMN snr REAL;
