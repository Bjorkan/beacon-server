// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/go-chi/chi/v5"
)

func TestSearchKnownRoutes_MissingParams(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/routes/search", searchKnownRoutes(stubReader{}))

	tests := []struct {
		name  string
		query string
	}{
		{"missing all", ""},
		{"missing from and to", "?iata=YVR"},
		{"missing to", "?iata=YVR&from=aa"},
		{"missing iata", "?from=aa&to=bb"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/routes/search"+tt.query, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", tt.name, w.Code)
		}
	}
}

func TestSearchCrossIATARoutes_MissingParams(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/routes/cross", searchCrossIATARoutes(stubReader{}))

	tests := []struct {
		name  string
		query string
	}{
		{"missing all", ""},
		{"missing toHash and toIata", "?fromHash=aa&fromIata=YVR"},
		{"missing fromIata", "?fromHash=aa&toHash=bb&toIata=YYJ"},
		{"missing fromHash", "?fromIata=YVR&toHash=bb&toIata=YYJ"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/routes/cross"+tt.query, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", tt.name, w.Code)
		}
	}
}

func TestListKnownRoutes_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/routes", listKnownRoutes(stubReader{
		listKnownRoutes: func(_ context.Context, _ api.RouteListParams) (api.Page[api.KnownRoute], error) {
			return api.Page[api.KnownRoute]{Items: []api.KnownRoute{{IATA: "YVR"}}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/routes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListKnownRoutes_PassesBackendFilterAndSort(t *testing.T) {
	var got api.RouteListParams
	r := chi.NewRouter()
	r.Get("/routes", listKnownRoutes(stubReader{
		listKnownRoutes: func(_ context.Context, params api.RouteListParams) (api.Page[api.KnownRoute], error) {
			got = params
			return api.Page[api.KnownRoute]{}, nil
		},
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/routes?iatas=yvr,yyj&sort=hops&direction=asc&limit=25", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(got.IATAs) != 2 || got.IATAs[0] != "YVR" || got.IATAs[1] != "YYJ" {
		t.Fatalf("unexpected IATAs: %v", got.IATAs)
	}
	if got.Sort != api.RouteSortHops || got.Direction != api.SortAsc || got.Limit != 25 {
		t.Fatalf("unexpected sort params: %#v", got)
	}
}

func TestListKnownRoutes_RejectsInvalidSort(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/routes", listKnownRoutes(stubReader{}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/routes?sort=unknown", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchKnownRoutes_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/routes/search", searchKnownRoutes(stubReader{
		searchKnownRoutes: func(_ context.Context, _, _, _ string) ([]api.KnownRoute, error) {
			return []api.KnownRoute{{IATA: "YVR"}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/routes/search?iata=YVR&from=aa&to=bb", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSearchCrossIATARoutes_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/routes/cross", searchCrossIATARoutes(stubReader{
		searchCrossIATARoutes: func(_ context.Context, _, _, _, _ string) ([]api.CrossIATARoute, error) {
			return []api.CrossIATARoute{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/routes/cross?fromHash=aa&fromIata=YVR&toHash=bb&toIata=YYJ", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
