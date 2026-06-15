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
)

func TestGetPacket_InvalidHex(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets/{packetHash}", getPacket(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets/nothex!!", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_InvalidPayloadType(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?payloadType=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_InvalidRouteType(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?routeType=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_InvalidSince(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?since=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_InvalidUntil(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?until=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_InvalidCursor(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?cursor=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_InvalidLimit(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?limit=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPacketsBackfill_MissingAfterID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets/backfill", listPacketsBackfill(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets/backfill", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPacketsBackfill_InvalidAfterID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets/backfill", listPacketsBackfill(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets/backfill?afterObservationId=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPacketsBackfill_InvalidLimit(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets/backfill", listPacketsBackfill(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets/backfill?afterObservationId=1&limit=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{
		listPackets: func(_ context.Context, _, _ int16, _ []string, _ string, _, _ time.Time, _ int64, _ int32) (api.Page[api.PacketSummary], error) {
			return api.Page[api.PacketSummary]{Items: []api.PacketSummary{{PacketHash: "deadbeef"}}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/packets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetPacket_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets/{packetHash}", getPacket(stubReader{
		getPacket: func(_ context.Context, hash []byte) (*api.Packet, error) {
			return &api.Packet{PacketHash: "deadbeef"}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/packets/deadbeef", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListPacketsBackfill_OK(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets/backfill", listPacketsBackfill(stubReader{
		listPacketsAfterID: func(_ context.Context, _ int64, _, _ int16, _ []string, _ string, _ int32) ([]api.PacketSummary, error) {
			return []api.PacketSummary{{PacketHash: "deadbeef"}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/packets/backfill?afterObservationId=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
