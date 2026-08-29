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
		listObservers: func(_ context.Context, _ api.ObserverListParams) (api.Page[api.ObserverSummary], error) {
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

func TestListObservers_SortAndPageTokenPassedThrough(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000321")
	token := api.EncodePageToken(api.PageToken{Version: api.PageTokenVersion, Collection: api.PageCollectionObservers, Sort: api.ObserverSortStatus, Direction: api.SortDesc, Key: "online", ID: id})
	var got api.ObserverListParams
	r := chi.NewRouter()
	r.Get("/observers", listObservers(stubReader{
		listObservers: func(_ context.Context, params api.ObserverListParams) (api.Page[api.ObserverSummary], error) {
			got = params
			return api.Page[api.ObserverSummary]{}, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/observers?sort=status&direction=desc&pageToken="+token+"&scope=%23bc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got.Sort != api.ObserverSortStatus || got.Direction != api.SortDesc {
		t.Fatalf("got sort=%q direction=%q", got.Sort, got.Direction)
	}
	if got.PageToken == nil || got.PageToken.ID != id || got.PageToken.Key != "online" {
		t.Fatalf("page token not passed through: %#v", got.PageToken)
	}
	if got.Scope != "#bc" {
		t.Fatalf("got scope %q, want #bc", got.Scope)
	}
}

func TestListObservers_RejectsInvalidSortablePagination(t *testing.T) {
	validID := uuid.MustParse("00000000-0000-0000-0000-000000000321")
	mismatched := api.EncodePageToken(api.PageToken{Version: api.PageTokenVersion, Collection: api.PageCollectionObservers, Sort: api.ObserverSortIATA, Direction: api.SortAsc, Key: "YVR", ID: validID})
	wrongCollection := api.EncodePageToken(api.PageToken{Version: api.PageTokenVersion, Collection: api.PageCollectionNodes, Sort: api.ObserverSortStatus, Direction: api.SortDesc, Key: "online", ID: validID})
	tests := []string{
		"/observers?sort=bogus",
		"/observers?direction=sideways",
		"/observers?pageToken=not-a-token",
		"/observers?sort=status&direction=desc&pageToken=" + mismatched,
		"/observers?sort=status&direction=desc&pageToken=" + wrongCollection,
		"/observers?sort=status&direction=desc&cursor=1700000000000",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/observers", listObservers(stubReader{}))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
