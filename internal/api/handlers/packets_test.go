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
		listPackets: func(_ context.Context, _, _ []int16, _ []string, _ []string, _, _ time.Time, _ int64, _ int32) (api.Page[api.PacketSummary], error) {
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

func TestListPackets_PluralParams_PassedThrough(t *testing.T) {
	var gotPayloadTypes, gotRouteTypes []int16
	var gotScopes []string
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{
		listPackets: func(_ context.Context, payloadTypes, routeTypes []int16, _ []string, scopes []string, _, _ time.Time, _ int64, _ int32) (api.Page[api.PacketSummary], error) {
			gotPayloadTypes = payloadTypes
			gotRouteTypes = routeTypes
			gotScopes = scopes
			return api.Page[api.PacketSummary]{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/packets?payloadTypes=2,4&routeTypes=0,1&scopes=%23bc,%23west", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(gotPayloadTypes) != 2 || gotPayloadTypes[0] != 2 || gotPayloadTypes[1] != 4 {
		t.Errorf("expected payloadTypes [2 4], got %v", gotPayloadTypes)
	}
	if len(gotRouteTypes) != 2 || gotRouteTypes[0] != 0 || gotRouteTypes[1] != 1 {
		t.Errorf("expected routeTypes [0 1], got %v", gotRouteTypes)
	}
	if len(gotScopes) != 2 || gotScopes[0] != "#bc" || gotScopes[1] != "#west" {
		t.Errorf("expected scopes [#bc #west], got %v", gotScopes)
	}
}

func TestListPackets_SingularParams_StillWork(t *testing.T) {
	var gotPayloadTypes, gotRouteTypes []int16
	var gotScopes []string
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{
		listPackets: func(_ context.Context, payloadTypes, routeTypes []int16, _ []string, scopes []string, _, _ time.Time, _ int64, _ int32) (api.Page[api.PacketSummary], error) {
			gotPayloadTypes = payloadTypes
			gotRouteTypes = routeTypes
			gotScopes = scopes
			return api.Page[api.PacketSummary]{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/packets?payloadType=4&routeType=1&scope=%23bc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(gotPayloadTypes) != 1 || gotPayloadTypes[0] != 4 {
		t.Errorf("expected payloadTypes [4], got %v", gotPayloadTypes)
	}
	if len(gotRouteTypes) != 1 || gotRouteTypes[0] != 1 {
		t.Errorf("expected routeTypes [1], got %v", gotRouteTypes)
	}
	if len(gotScopes) != 1 || gotScopes[0] != "#bc" {
		t.Errorf("expected scopes [#bc], got %v", gotScopes)
	}
}

func TestListPackets_PluralParams_TakePrecedenceOverSingular(t *testing.T) {
	// Mirrors parseIATAs' precedence: when both are present, the plural param wins outright
	// rather than merging with the singular one.
	var gotPayloadTypes []int16
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{
		listPackets: func(_ context.Context, payloadTypes, _ []int16, _ []string, _ []string, _, _ time.Time, _ int64, _ int32) (api.Page[api.PacketSummary], error) {
			gotPayloadTypes = payloadTypes
			return api.Page[api.PacketSummary]{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/packets?payloadType=9&payloadTypes=2,4", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(gotPayloadTypes) != 2 || gotPayloadTypes[0] != 2 || gotPayloadTypes[1] != 4 {
		t.Errorf("expected payloadTypes [2 4] (plural wins), got %v", gotPayloadTypes)
	}
}

func TestListPackets_InvalidPayloadTypes(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?payloadTypes=2,notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListPackets_InvalidRouteTypes(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/packets?routeTypes=0,notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
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
