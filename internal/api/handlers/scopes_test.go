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

func TestListScopes_NoIATAs_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/scopes", listScopes(stubReader{
		getScopeNames: func(_ context.Context) ([]string, error) {
			return []string{"#bc", "#west"}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/scopes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetScope_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/scopes/{name}", getScope(stubReader{
		getScopeByName: func(_ context.Context, name string) (*api.ScopeDetail, error) {
			return &api.ScopeDetail{Name: name}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/scopes/%23bc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
