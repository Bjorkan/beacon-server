// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MeshCore-Beacon/beacon-server/internal/ingest"
	"github.com/go-chi/chi/v5"
)

func TestListBrokers_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/brokers", listBrokers([]*ingest.Worker{}))
	req := httptest.NewRequest(http.MethodGet, "/brokers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result []BrokerStatus
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
}
