// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestGetNode_InvalidUUID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}", getNode(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodeObservations_InvalidUUID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}/observations", listNodeObservations(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/bad/observations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodeObservations_InvalidCursor(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}/observations", listNodeObservations(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/00000000-0000-0000-0000-000000000001/observations?cursor=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodeObservations_InvalidLimit(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}/observations", listNodeObservations(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/00000000-0000-0000-0000-000000000001/observations?limit=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_InvalidType(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?type=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_InvalidLimit(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?limit=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_InvalidCursor(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?cursor=notanint", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_InvalidPubkey(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?pubkey=nothex!!", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_InvalidSupportsMultibytePaths(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?supportsMultibytePaths=notabool", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_InvalidSupportsMultibyteTraces(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?supportsMultibyteTraces=notabool", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_OK(t *testing.T) {
	nodeID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{
		listNodes: func(_ context.Context, _ api.NodeListParams) (api.Page[api.NodeSummary], error) {
			return api.Page[api.NodeSummary]{Items: []api.NodeSummary{{ID: nodeID}}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListNodes_NeighborsParam_PassedThrough(t *testing.T) {
	nodeID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	neighborID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	var gotIncludeNeighbors bool
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{
		listNodes: func(_ context.Context, params api.NodeListParams) (api.Page[api.NodeSummary], error) {
			gotIncludeNeighbors = params.IncludeNeighbors
			return api.Page[api.NodeSummary]{Items: []api.NodeSummary{{ID: nodeID, NeighborIDs: []uuid.UUID{neighborID}}}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?neighbors=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !gotIncludeNeighbors {
		t.Error("expected neighbors=true query param to be passed through as includeNeighbors=true")
	}
	if !strings.Contains(w.Body.String(), "neighborIds") {
		t.Errorf("expected response body to include neighborIds, got %s", w.Body.String())
	}
}

func TestListNodes_NeighborsParam_InvalidValue(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?neighbors=notabool", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_NeighborsParam_BareFlagMeansTrue(t *testing.T) {
	var gotIncludeNeighbors bool
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{
		listNodes: func(_ context.Context, params api.NodeListParams) (api.Page[api.NodeSummary], error) {
			gotIncludeNeighbors = params.IncludeNeighbors
			return api.Page[api.NodeSummary]{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?neighbors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !gotIncludeNeighbors {
		t.Error("expected bare ?neighbors (no value) to be treated as true")
	}
}

func TestListNodes_PubkeyPrefixParam_PassedThrough(t *testing.T) {
	var gotPubkeyPrefix string
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{
		listNodes: func(_ context.Context, params api.NodeListParams) (api.Page[api.NodeSummary], error) {
			gotPubkeyPrefix = params.PubkeyPrefix
			return api.Page[api.NodeSummary]{}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?pubkeyPrefix=AB12", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if gotPubkeyPrefix != "ab12" {
		t.Errorf("expected pubkeyPrefix to be lowercased and passed through, got %q", gotPubkeyPrefix)
	}
}

func TestListNodes_PubkeyPrefixParam_InvalidHex(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?pubkeyPrefix=zz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodes_PubkeyPrefixParam_RejectsWildcards(t *testing.T) {
	// isHexString should reject ILIKE wildcard characters, since pubkeyPrefix is matched as
	// raw text rather than decoded to bytes -- % or _ getting through would let a caller
	// widen the match pattern beyond a literal prefix.
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes?pubkeyPrefix=ab%25", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetNode_OK(t *testing.T) {
	nodeID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}", getNode(stubReader{
		getNode: func(_ context.Context, id uuid.UUID) (*api.Node, error) {
			return &api.Node{NodeSummary: api.NodeSummary{ID: id}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/"+nodeID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListNodeNeighbors_OK(t *testing.T) {
	nodeID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}/neighbors", listNodeNeighbors(stubReader{
		getNodeNeighbors: func(_ context.Context, _ uuid.UUID) ([]api.NodeNeighbor, error) {
			return []api.NodeNeighbor{{ID: nodeID}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/"+nodeID.String()+"/neighbors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListNodeNeighbors_InvalidUUID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}/neighbors", listNodeNeighbors(stubReader{}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/not-a-uuid/neighbors", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListNodeObservations_OK(t *testing.T) {
	nodeID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	r := chi.NewRouter()
	r.Get("/nodes/{nodeId}/observations", listNodeObservations(stubReader{
		listNodeObservations: func(_ context.Context, _ uuid.UUID, _ int64, _ int32) (api.Page[api.PacketObservationSummary], error) {
			return api.Page[api.PacketObservationSummary]{Items: []api.PacketObservationSummary{{ID: 1}}}, nil
		},
	}))
	req := httptest.NewRequest(http.MethodGet, "/nodes/"+nodeID.String()+"/observations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListNodes_SortAndPageTokenPassedThrough(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000123")
	token := api.EncodePageToken(api.PageToken{Version: api.PageTokenVersion, Collection: api.PageCollectionNodes, Sort: api.NodeSortName, Direction: api.SortAsc, Key: "alpha", ID: id})
	var got api.NodeListParams
	r := chi.NewRouter()
	r.Get("/nodes", listNodes(stubReader{
		listNodes: func(_ context.Context, params api.NodeListParams) (api.Page[api.NodeSummary], error) {
			got = params
			return api.Page[api.NodeSummary]{}, nil
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/nodes?sort=name&direction=asc&pageToken="+token+"&scope=%23bc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got.Sort != api.NodeSortName || got.Direction != api.SortAsc {
		t.Fatalf("got sort=%q direction=%q", got.Sort, got.Direction)
	}
	if got.PageToken == nil || got.PageToken.ID != id || got.PageToken.Key != "alpha" {
		t.Fatalf("page token not passed through: %#v", got.PageToken)
	}
	if got.Scope != "#bc" {
		t.Fatalf("got scope %q, want #bc", got.Scope)
	}
}

func TestListNodes_RejectsInvalidSortablePagination(t *testing.T) {
	validID := uuid.MustParse("00000000-0000-0000-0000-000000000123")
	mismatched := api.EncodePageToken(api.PageToken{Version: api.PageTokenVersion, Collection: api.PageCollectionNodes, Sort: api.NodeSortType, Direction: api.SortAsc, Key: "repeater", ID: validID})
	wrongCollection := api.EncodePageToken(api.PageToken{Version: api.PageTokenVersion, Collection: api.PageCollectionObservers, Sort: api.NodeSortName, Direction: api.SortAsc, Key: "alpha", ID: validID})
	tests := []string{
		"/nodes?sort=bogus",
		"/nodes?direction=sideways",
		"/nodes?pageToken=not-a-token",
		"/nodes?sort=name&direction=asc&pageToken=" + mismatched,
		"/nodes?sort=name&direction=asc&pageToken=" + wrongCollection,
		"/nodes?sort=name&direction=asc&cursor=1700000000000",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/nodes", listNodes(stubReader{}))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
