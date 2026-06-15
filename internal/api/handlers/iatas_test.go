// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/json"
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
