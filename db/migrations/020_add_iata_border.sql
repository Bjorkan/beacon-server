-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Optional GeoJSON border for the region border map feature. Stores a full
-- GeoJSON Feature (type/bbox/properties/geometry), not a bare geometry, so
-- properties can be added later without a schema change. Geometry is always
-- Polygon or MultiPolygon; bbox is computed and stored at write time (see
-- internal/config/border.go) so reads never need to recompute it.
--
-- NULL for the vast majority of IATAs, matching display_name/approx_lat/
-- approx_lng's existing "optional, mostly unset" pattern. Written via the
-- same config-file-driven seeding path as those columns (internal/config/
-- seed.go), not a runtime HTTP write endpoint -- this codebase has none.
--
-- Deliberately excluded from GET /api/v1/iatas (border polygons are large --
-- thousands of vertices, MultiPolygon for archipelago coastlines -- and
-- would bloat every list response for the ~all IATAs that don't have one).
-- Served only via GET /api/v1/iatas/{iata}/border, fetched lazily one at a
-- time on client-side region selection.

ALTER TABLE iata_codes ADD COLUMN border JSONB;
