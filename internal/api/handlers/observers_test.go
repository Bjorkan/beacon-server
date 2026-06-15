// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestGetObserverTelemetry_InvalidUUID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/telemetry", getObserverTelemetry(stubReader{}))

	req := httptest.NewRequest(http.MethodGet, "/observers/not-a-uuid/telemetry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetObserverTelemetry_InvalidRange(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/telemetry", getObserverTelemetry(stubReader{}))

	req := httptest.NewRequest(http.MethodGet, "/observers/00000000-0000-0000-0000-000000000001/telemetry?range=banana", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetObserverTelemetry_InvalidAfterID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/telemetry", getObserverTelemetry(stubReader{}))

	req := httptest.NewRequest(http.MethodGet, "/observers/00000000-0000-0000-0000-000000000001/telemetry?afterId=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetObserverTelemetry_InvalidInterval(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/telemetry", getObserverTelemetry(stubReader{}))

	req := httptest.NewRequest(http.MethodGet, "/observers/00000000-0000-0000-0000-000000000001/telemetry?interval=2h", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListObservers_OK(t *testing.T) {
	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/observers", listObservers(stubReader{
		listObservers: func(_ context.Context, _ []string, _, _, _, _, _ string, _ int64, _ int32) (api.Page[api.ObserverSummary], error) {
			return api.Page[api.ObserverSummary]{Items: []api.ObserverSummary{{ID: observerID}}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/observers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetObserver_OK(t *testing.T) {
	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/observers/{observerId}", getObserver(stubReader{
		getObserver: func(_ context.Context, id uuid.UUID) (*api.Observer, error) {
			return &api.Observer{ObserverSummary: api.ObserverSummary{ID: id}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/observers/"+observerID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetObserver_InvalidUUID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/observers/{observerId}", getObserver(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/observers/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetObserverTelemetry_OK(t *testing.T) {
	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/telemetry", getObserverTelemetry(stubReader{
		getObserverTelemetry: func(_ context.Context, _ uuid.UUID, _, _ time.Time, _ int64) (*api.ObserverTelemetry, error) {
			return &api.ObserverTelemetry{Points: []api.ObserverTelemetryPoint{}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/observers/"+observerID.String()+"/telemetry", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetObserverTelemetry_Bucketed_OK(t *testing.T) {
	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/telemetry", getObserverTelemetry(stubReader{
		getObserverTelemetryBucketed: func(_ context.Context, _ uuid.UUID, _, _ time.Time, _ int32) ([]api.ObserverTelemetryPoint, error) {
			return []api.ObserverTelemetryPoint{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/observers/"+observerID.String()+"/telemetry?interval=6h", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListObserverAdverts_OK(t *testing.T) {
	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/adverts", listObserverAdverts(stubReader{
		listObserverAdverts: func(_ context.Context, _ uuid.UUID, _ int64, _ int32) (api.Page[api.AdvertObservation], error) {
			return api.Page[api.AdvertObservation]{Items: []api.AdvertObservation{}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/observers/"+observerID.String()+"/adverts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListObserverAdverts_InvalidUUID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/observers/{observerId}/adverts", listObserverAdverts(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/observers/not-a-uuid/adverts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
