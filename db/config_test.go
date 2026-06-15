// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"context"
	"errors"
	"testing"

	sqlc "github.com/MeshCore-Beacon/beacon-server/db/sqlc"
	mockdb "github.com/MeshCore-Beacon/beacon-server/db/sqlc/mock"
	"go.uber.org/mock/gomock"
)

func TestListIATAs(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	lat := 49.1967
	lng := -123.1815
	name := "Vancouver"

	mock.EXPECT().
		ListIATAs(gomock.Any()).
		Return([]sqlc.IataCode{
			{Iata: "YVR", DisplayName: &name, ApproxLat: &lat, ApproxLng: &lng},
		}, nil)

	store := &Store{q: mock}
	iatas, err := store.ListIATAs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iatas) != 1 {
		t.Fatalf("expected 1 iata, got %d", len(iatas))
	}
	if iatas[0].IATA != "YVR" {
		t.Errorf("expected IATA YVR, got %s", iatas[0].IATA)
	}
	if *iatas[0].Lat != lat {
		t.Errorf("expected Lat %f, got %f", lat, *iatas[0].Lat)
	}
}

func TestGetIATA(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	lat := 49.1967
	lng := -123.1815
	name := "Vancouver"

	mock.EXPECT().
		GetIATA(gomock.Any(), "YVR").
		Return(sqlc.IataCode{Iata: "YVR", DisplayName: &name, ApproxLat: &lat, ApproxLng: &lng}, nil)

	store := &Store{q: mock}
	iata, err := store.GetIATA(context.Background(), "YVR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iata.IATA != "YVR" {
		t.Errorf("expected YVR, got %s", iata.IATA)
	}
	if *iata.Lng != lng {
		t.Errorf("expected Lng %f, got %f", lng, *iata.Lng)
	}
}

func TestGetRegion_WithZoomLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	zoom := int32(10)

	mock.EXPECT().
		GetRegion(gomock.Any(), int32(1)).
		Return(sqlc.GetRegionRow{
			ID:        1,
			Slug:      "bc",
			Name:      "British Columbia",
			ZoomLevel: &zoom,
		}, nil)

	mock.EXPECT().
		GetRegionIATAs(gomock.Any(), int32(1)).
		Return([]string{"YVR", "YYJ"}, nil)

	store := &Store{q: mock}
	region, err := store.GetRegion(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.Slug != "bc" {
		t.Errorf("expected slug bc, got %s", region.Slug)
	}
	if region.ZoomLevel == nil {
		t.Fatal("expected ZoomLevel to be set")
	}
	if *region.ZoomLevel != 10 {
		t.Errorf("expected ZoomLevel 10, got %d", *region.ZoomLevel)
	}
	if len(region.IATAs) != 2 {
		t.Errorf("expected 2 IATAs, got %d", len(region.IATAs))
	}
}

func TestGetRegion_IATAError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	mock.EXPECT().
		GetRegion(gomock.Any(), int32(1)).
		Return(sqlc.GetRegionRow{ID: 1, Slug: "bc", Name: "British Columbia"}, nil)

	mock.EXPECT().
		GetRegionIATAs(gomock.Any(), int32(1)).
		Return(nil, errors.New("db error"))

	store := &Store{q: mock}
	_, err := store.GetRegion(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetRegionBySlug(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	mock.EXPECT().
		GetRegionBySlug(gomock.Any(), "bc").
		Return(sqlc.GetRegionBySlugRow{ID: 1, Slug: "bc", Name: "British Columbia"}, nil)

	mock.EXPECT().
		GetRegionIATAs(gomock.Any(), int32(1)).
		Return([]string{"YVR"}, nil)

	store := &Store{q: mock}
	region, err := store.GetRegionBySlug(context.Background(), "bc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.Slug != "bc" {
		t.Errorf("expected slug bc, got %s", region.Slug)
	}
	if len(region.IATAs) != 1 {
		t.Errorf("expected 1 IATA, got %d", len(region.IATAs))
	}
}
