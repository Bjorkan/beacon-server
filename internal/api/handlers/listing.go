// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
)

func parseSortablePage(r *http.Request, collection, defaultSort string, defaultDirection api.SortDirection, validSort func(string) bool) (string, api.SortDirection, *api.PageToken, error) {
	sort := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if sort == "" {
		sort = defaultSort
	}
	if !validSort(sort) {
		return "", "", nil, fmt.Errorf("unsupported sort %q", sort)
	}

	direction := api.SortDirection(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("direction"))))
	if direction == "" {
		direction = defaultDirection
	}
	if direction != api.SortAsc && direction != api.SortDesc {
		return "", "", nil, fmt.Errorf("direction must be asc or desc")
	}

	token, err := api.DecodePageToken(r.URL.Query().Get("pageToken"))
	if err != nil {
		return "", "", nil, fmt.Errorf("pageToken is invalid")
	}
	if token != nil && token.Collection != collection {
		return "", "", nil, fmt.Errorf("pageToken does not belong to this endpoint")
	}
	if token != nil && (token.Sort != sort || token.Direction != direction) {
		return "", "", nil, fmt.Errorf("pageToken does not match sort and direction")
	}
	return sort, direction, token, nil
}
