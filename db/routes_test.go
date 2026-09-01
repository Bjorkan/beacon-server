// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"
	"time"

	sqlc "github.com/MeshCore-Beacon/beacon-server/db/sqlc"
	mockdb "github.com/MeshCore-Beacon/beacon-server/db/sqlc/mock"
	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/mock/gomock"
)

func TestRoutePathKey_MatchesPostgresDigest(t *testing.T) {
	// Golden vector: Postgres computes
	//   decode(md5(array_to_string(node_ids, ',')), 'hex')
	// over lowercase-hyphenated UUIDs. Migration 024 backfilled with that
	// expression; this pins the Go side to the identical bytes.
	a := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	b := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	got := hex.EncodeToString(routePathKey([]uuid.UUID{a, b}))
	want := "f097439148601d9f3291c474f82fa64c"
	if got != want {
		t.Errorf("routePathKey = %s, want %s", got, want)
	}
}

func TestUpsertKnownRoute_ComputesPathKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock := mockdb.NewMockQuerier(ctrl)
	store := &Store{q: mock}

	a := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	b := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	wantKey, _ := hex.DecodeString("f097439148601d9f3291c474f82fa64c")

	mock.EXPECT().UpsertKnownRoute(gomock.Any(), gomock.Cond(func(p sqlc.UpsertKnownRouteParams) bool {
		return bytes.Equal(p.PathKey, wantKey)
	})).Return(nil)

	if err := store.UpsertKnownRoute(context.Background(), []uuid.UUID{a, b}, [][]byte{{0x37}, {0xd8}}, "PRG", 2); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteOldRoutes_PassesCutoffs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mock := mockdb.NewMockQuerier(ctrl)
	store := &Store{q: mock}

	retention := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	grace := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	mock.EXPECT().DeleteOldRoutes(gomock.Any(), gomock.Cond(func(p sqlc.DeleteOldRoutesParams) bool {
		return p.LastSeen.Time.Equal(retention) && p.ObservationCount == 3 && p.LastSeen_2.Time.Equal(grace)
	})).Return(nil)

	if err := store.DeleteOldRoutes(context.Background(), retention, 3, grace); err != nil {
		t.Fatal(err)
	}
}

func TestExtractFromNode_Found(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	hops := []api.RouteHop{
		{NodeID: a},
		{NodeID: b},
		{NodeID: c},
	}
	result := extractFromNode(hops, b)
	if len(result) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(result))
	}
	if result[0].NodeID != b {
		t.Errorf("expected first hop to be b, got %s", result[0].NodeID)
	}
	if result[1].NodeID != c {
		t.Errorf("expected second hop to be c, got %s", result[1].NodeID)
	}
}

func TestExtractFromNode_FirstNode(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	hops := []api.RouteHop{{NodeID: a}, {NodeID: b}}
	result := extractFromNode(hops, a)
	if len(result) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(result))
	}
}

func TestExtractFromNode_NotFound(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	hops := []api.RouteHop{{NodeID: a}}
	result := extractFromNode(hops, b)
	// not found returns full slice
	if len(result) != 1 {
		t.Fatalf("expected full slice returned, got %d hops", len(result))
	}
}

func TestExtractFromNode_Empty(t *testing.T) {
	result := extractFromNode(nil, uuid.New())
	if len(result) != 0 {
		t.Errorf("expected empty result for nil hops")
	}
}

func TestListKnownRoutes_UsesKeysetSortAndIATAFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)
	first := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	last := first.Add(time.Hour)

	mock.EXPECT().ListKnownRoutes(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, params sqlc.ListKnownRoutesParams) ([]sqlc.ListKnownRoutesRow, error) {
			if len(params.Column1) != 2 || params.Column1[0] != "YVR" || params.Column1[1] != "YYJ" {
				t.Fatalf("unexpected IATA filter: %v", params.Column1)
			}
			if params.Column4 != api.RouteSortHops || params.Column5 != string(api.SortAsc) || params.Limit != 2 {
				t.Fatalf("unexpected sort/page params: %#v", params)
			}
			return []sqlc.ListKnownRoutesRow{
				{ID: 11, Iata: "YVR", HopCount: 1, FirstSeen: pgtype.Timestamptz{Time: first, Valid: true}, LastSeen: pgtype.Timestamptz{Time: last, Valid: true}, PageSortKey: "00000000000000000001"},
				{ID: 12, Iata: "YYJ", HopCount: 2, FirstSeen: pgtype.Timestamptz{Time: first, Valid: true}, LastSeen: pgtype.Timestamptz{Time: last, Valid: true}, PageSortKey: "00000000000000000002"},
			}, nil
		},
	)
	mock.EXPECT().GetNodesByIDs(gomock.Any(), gomock.Cond(func(ids []uuid.UUID) bool { return len(ids) == 0 })).Return([]sqlc.GetNodesByIDsRow{}, nil)

	store := &Store{q: mock}
	page, err := store.ListKnownRoutes(context.Background(), api.RouteListParams{
		IATAs: []string{"YVR", "YYJ"}, Sort: api.RouteSortHops, Direction: api.SortAsc, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.HasMore || page.NextPageToken == nil {
		t.Fatalf("unexpected page: %#v", page)
	}
	token, err := api.DecodePageToken(*page.NextPageToken)
	if err != nil {
		t.Fatal(err)
	}
	if token.Collection != api.PageCollectionRoutes || token.Sort != api.RouteSortHops || token.Direction != api.SortAsc || token.Key != "00000000000000000001" || token.NumericID != 11 {
		t.Fatalf("unexpected token: %#v", token)
	}
}
