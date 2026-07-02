-- Copyright 2026 Beacon Contributors
-- SPDX-License-Identifier: AGPL-3.0-or-later

-- Cleans up the ghost "unknown key" channel rows created by the
-- side_effects.go bug where UpsertChannelHashOnly was called
-- unconditionally on every GRP_TXT packet, even when the key was
-- known and the message decrypted successfully.
--
-- Safe because hash-only rows (key_fingerprint IS NULL) never have
-- channel_messages attached to them -- InsertChannelMessage is only
-- ever called against a keyed channel row. We only delete a hash-only
-- row when a keyed row already exists for the same channel_hash;
-- hashes with no known key at all (genuinely unknown) are left as-is.

DELETE FROM channels c
WHERE c.key_fingerprint IS NULL
  AND EXISTS (
    SELECT 1 FROM channels k
    WHERE k.channel_hash = c.channel_hash
      AND k.key_fingerprint IS NOT NULL
  );
