// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/google/uuid"
)

func listCursorValues(token *api.PageToken) (valid, empty bool, key string, id uuid.UUID) {
	if token == nil {
		return false, false, "", uuid.Nil
	}
	return true, token.Empty, token.Key, token.ID
}

func nextPageToken(collection, sort string, direction api.SortDirection, empty *bool, key string, id uuid.UUID) string {
	isEmpty := empty != nil && *empty
	return api.EncodePageToken(api.PageToken{
		Version:    api.PageTokenVersion,
		Collection: collection,
		Sort:       sort,
		Direction:  direction,
		Empty:      isEmpty,
		Key:        key,
		ID:         id,
	})
}
