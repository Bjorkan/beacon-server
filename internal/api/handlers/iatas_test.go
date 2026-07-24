// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/go-chi/chi/v5"
)

func TestListIATAs_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/iatas", listIATAs(stubReader{
		listIATAs: func(_ context.Context) ([]api.IATA, error) {
			return []api.IATA{{IATA: "YVR"}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/iatas", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result []api.IATA
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(result) != 1 || result[0].IATA != "YVR" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestGetIATA_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/iatas/{iata}", getIATA(stubReader{
		getIATA: func(_ context.Context, iata string) (*api.IATA, error) {
			return &api.IATA{IATA: iata}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/iatas/YVR", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetIATABorder_OK(t *testing.T) {
	feature := `{"type":"Feature","bbox":[-76.4,45.0,-75.2,45.6],"properties":{},"geometry":{"type":"Polygon","coordinates":[[[-76.4,45.0],[-75.2,45.0],[-75.2,45.6],[-76.4,45.6],[-76.4,45.0]]]}}`
	r := chi.NewRouter()
	r.Get("/iatas/{iata}/border", getIATABorder(stubReader{
		getIATABorder: func(_ context.Context, iata string) (json.RawMessage, error) {
			return json.RawMessage(feature), nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/iatas/YOW/border", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/geo+json" {
		t.Errorf("expected Content-Type application/geo+json, got %q", ct)
	}
	if w.Body.String() != feature {
		t.Errorf("expected body to be the raw feature JSON, got %s", w.Body.String())
	}
}

func TestGetIATABorder_NoContentWhenNil(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/iatas/{iata}/border", getIATABorder(stubReader{
		getIATABorder: func(_ context.Context, _ string) (json.RawMessage, error) {
			return nil, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/iatas/YBG/border", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestGetIATABorder_NoContentOnCacheRoundTrippedNull(t *testing.T) {
	// A cached nil json.RawMessage comes back as the literal 4-byte JSON "null" after a
	// round trip through the cache's own json.Marshal/Unmarshal -- must still read as 204.
	r := chi.NewRouter()
	r.Get("/iatas/{iata}/border", getIATABorder(stubReader{
		getIATABorder: func(_ context.Context, _ string) (json.RawMessage, error) {
			return json.RawMessage("null"), nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/iatas/YBG/border", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestGetIATABorder_NotFound(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/iatas/{iata}/border", getIATABorder(stubReader{
		getIATABorder: func(_ context.Context, _ string) (json.RawMessage, error) {
			return nil, errors.New("not found")
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/iatas/ZZZ/border", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
