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
	"github.com/google/uuid"
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
		listPackets: func(_ context.Context, _ api.PacketListParams) (api.Page[api.PacketSummary], error) {
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

func TestListPackets_SearchAndObserversPassedThrough(t *testing.T) {
	observerA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	observerB := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	var got api.PacketListParams
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{
		listPackets: func(_ context.Context, params api.PacketListParams) (api.Page[api.PacketSummary], error) {
			got = params
			return api.Page[api.PacketSummary]{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/packets?observers="+observerA.String()+","+observerB.String()+"&q=AA-BB%20CC&searchField=path", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(got.ObserverIDs) != 2 || got.ObserverIDs[0] != observerA || got.ObserverIDs[1] != observerB {
		t.Fatalf("unexpected observers: %v", got.ObserverIDs)
	}
	if got.SearchField != api.PacketSearchPath || got.Search != "aabbcc" {
		t.Fatalf("unexpected search field/query: %q %q", got.SearchField, got.Search)
	}
}

func TestListPackets_RejectsInvalidSearchOrObserver(t *testing.T) {
	for _, query := range []string{"?searchField=unknown&q=x", "?observer=not-a-uuid"} {
		r := chi.NewRouter()
		r.Get("/packets", listPackets(stubReader{}))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/packets"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", query, w.Code)
		}
	}
}

func TestListPackets_PluralParams_PassedThrough(t *testing.T) {
	var gotPayloadTypes, gotRouteTypes []int16
	var gotScopes []string
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{
		listPackets: func(_ context.Context, params api.PacketListParams) (api.Page[api.PacketSummary], error) {
			gotPayloadTypes = params.PayloadTypes
			gotRouteTypes = params.RouteTypes
			gotScopes = params.Scopes
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
		listPackets: func(_ context.Context, params api.PacketListParams) (api.Page[api.PacketSummary], error) {
			gotPayloadTypes = params.PayloadTypes
			gotRouteTypes = params.RouteTypes
			gotScopes = params.Scopes
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
		listPackets: func(_ context.Context, params api.PacketListParams) (api.Page[api.PacketSummary], error) {
			gotPayloadTypes = params.PayloadTypes
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

func TestListPackets_ResolvedPathIncludePassedThrough(t *testing.T) {
	var got bool
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{
		listPackets: func(_ context.Context, params api.PacketListParams) (api.Page[api.PacketSummary], error) {
			got = params.IncludeResolvedPath
			return api.Page[api.PacketSummary]{}, nil
		},
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/packets?include=resolvedPath", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !got {
		t.Fatal("expected include=resolvedPath to opt in")
	}
}

func TestListPackets_RejectsUnknownInclude(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/packets", listPackets(stubReader{}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/packets?include=everything", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListPacketsBackfill_ResolvedPathIncludePassedThrough(t *testing.T) {
	var got bool
	r := chi.NewRouter()
	r.Get("/packets/backfill", listPacketsBackfill(stubReader{
		listPacketsAfterID: func(_ context.Context, _ int64, _, _ int16, _ []string, _ string, _ int32, includeResolvedPath bool) ([]api.PacketSummary, error) {
			got = includeResolvedPath
			return nil, nil
		},
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/packets/backfill?afterObservationId=1&include=resolvedPath", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !got {
		t.Fatal("expected backfill include=resolvedPath to opt in")
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
		listPacketsAfterID: func(_ context.Context, _ int64, _, _ int16, _ []string, _ string, _ int32, _ bool) ([]api.PacketSummary, error) {
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
