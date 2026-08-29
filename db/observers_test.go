// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlc "github.com/MeshCore-Beacon/beacon-server/db/sqlc"
	mockdb "github.com/MeshCore-Beacon/beacon-server/db/sqlc/mock"
	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/mock/gomock"
)

func TestUpsertObserver_NilDisplayName(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	pubkey := []byte{0x01, 0x02}

	mock.EXPECT().
		UpsertObserver(gomock.Any(), pubkey).
		Return(sqlc.Observer{ID: observerID, DisplayName: nil}, nil)

	store := &Store{q: mock}
	id, displayName, err := store.UpsertObserver(context.Background(), pubkey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != observerID {
		t.Errorf("expected ID %s, got %s", observerID, id)
	}
	if displayName != "" {
		t.Errorf("expected empty displayName, got %s", displayName)
	}
}

func TestUpsertObserver_WithDisplayName(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	pubkey := []byte{0x01, 0x02}
	name := "test-observer"

	mock.EXPECT().
		UpsertObserver(gomock.Any(), pubkey).
		Return(sqlc.Observer{ID: observerID, DisplayName: &name}, nil)

	store := &Store{q: mock}
	_, displayName, err := store.UpsertObserver(context.Background(), pubkey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if displayName != "test-observer" {
		t.Errorf("expected displayName test-observer, got %s", displayName)
	}
}

func TestListObservers_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	lastStatusAt := pgtype.Timestamptz{Time: time.UnixMilli(1700000000000), Valid: true}
	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	rows := make([]sqlc.ListObserversRow, 3)
	for i := range rows {
		rows[i] = sqlc.ListObserversRow{
			ID:           observerID,
			LastSeen:     lastStatusAt,
			LastStatusAt: lastStatusAt,
			PageSortKey:  "00000001700000000000",
		}
	}

	mock.EXPECT().
		ListObservers(gomock.Any(), gomock.Any()).
		Return(rows, nil)

	store := &Store{q: mock}
	page, err := store.ListObservers(context.Background(), api.ObserverListParams{IATAs: []string{"YVR"}, Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(page.Items))
	}
	if !page.HasMore {
		t.Error("expected HasMore true")
	}
	if page.NextCursor == nil {
		t.Error("expected NextCursor to be set")
	}
	if page.NextPageToken == nil {
		t.Fatal("expected NextPageToken to be set")
	}
	token, err := api.DecodePageToken(*page.NextPageToken)
	if err != nil {
		t.Fatalf("invalid NextPageToken: %v", err)
	}
	if token.Collection != api.PageCollectionObservers || token.Sort != api.ObserverSortLastSeen || token.Direction != api.SortDesc {
		t.Fatalf("unexpected token ordering: %#v", token)
	}
}

func TestListObservers_RadioStringFormatting(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	freq := float32(915.0)
	sf := int16(7)
	bw := float32(125.0)

	mock.EXPECT().
		ListObservers(gomock.Any(), gomock.Any()).
		Return([]sqlc.ListObserversRow{
			{
				ID:           observerID,
				RadioFreqMhz: &freq,
				RadioSf:      &sf,
				RadioBwKhz:   &bw,
			},
		}, nil)

	store := &Store{q: mock}
	page, err := store.ListObservers(context.Background(), api.ObserverListParams{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page.Items))
	}
	if page.Items[0].Radio == nil {
		t.Fatal("expected Radio to be set")
	}
	if *page.Items[0].Radio != "915,125,7" {
		t.Errorf("expected Radio 915,125,7, got %s", *page.Items[0].Radio)
	}
}

func TestListObservers_NilRadioFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	mock.EXPECT().
		ListObservers(gomock.Any(), gomock.Any()).
		Return([]sqlc.ListObserversRow{
			{ID: observerID, RadioFreqMhz: nil, RadioSf: nil, RadioBwKhz: nil},
		}, nil)

	store := &Store{q: mock}
	page, err := store.ListObservers(context.Background(), api.ObserverListParams{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Items[0].Radio != nil {
		t.Errorf("expected nil Radio, got %s", *page.Items[0].Radio)
	}
}

func TestListObservers_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	mock.EXPECT().
		ListObservers(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("db error"))

	store := &Store{q: mock}
	_, err := store.ListObservers(context.Background(), api.ObserverListParams{Limit: 10})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetObserver_OnlineStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	obsCount := int64(10)

	mock.EXPECT().
		GetObserverByID(gomock.Any(), observerID).
		Return(sqlc.Observer{
			ID:               observerID,
			PublicKey:        []byte{0x01},
			ObservationCount: &obsCount,
			FirstSeen:        pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			LastSeen:         pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
			LastStatusAt:     pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
		}, nil)

	mock.EXPECT().
		GetObserverBrokers(gomock.Any(), observerID).
		Return([]sqlc.GetObserverBrokersRow{}, nil)

	mock.EXPECT().
		GetObserverScopes(gomock.Any(), observerID).
		Return([]string{"default"}, nil)

	mock.EXPECT().
		GetObserverLastIATA(gomock.Any(), observerID).
		Return("YVR", nil)

	store := &Store{q: mock}
	observer, err := store.GetObserver(context.Background(), observerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observer.Status != "online" {
		t.Errorf("expected status online, got %s", observer.Status)
	}
	if observer.IATA != "YVR" {
		t.Errorf("expected IATA YVR, got %s", observer.IATA)
	}
}

func TestGetObserver_OfflineStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	obsCount := int64(10)

	mock.EXPECT().
		GetObserverByID(gomock.Any(), observerID).
		Return(sqlc.Observer{
			ID:               observerID,
			PublicKey:        []byte{0x01},
			ObservationCount: &obsCount,
			FirstSeen:        pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			LastSeen:         pgtype.Timestamptz{Time: time.Now().Add(-10 * time.Minute), Valid: true},
			LastStatusAt:     pgtype.Timestamptz{Time: time.Now().Add(-10 * time.Minute), Valid: true},
		}, nil)

	mock.EXPECT().
		GetObserverBrokers(gomock.Any(), observerID).
		Return([]sqlc.GetObserverBrokersRow{}, nil)

	mock.EXPECT().
		GetObserverScopes(gomock.Any(), observerID).
		Return([]string{}, nil)

	mock.EXPECT().
		GetObserverLastIATA(gomock.Any(), observerID).
		Return("YVR", nil)

	store := &Store{q: mock}
	observer, err := store.GetObserver(context.Background(), observerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observer.Status != "offline" {
		t.Errorf("expected status offline, got %s", observer.Status)
	}
}

func TestGetObserver_BrokerLastPacketAtNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	obsCount := int64(10)

	mock.EXPECT().
		GetObserverByID(gomock.Any(), observerID).
		Return(sqlc.Observer{
			ID:               observerID,
			PublicKey:        []byte{0x01},
			ObservationCount: &obsCount,
			FirstSeen:        pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			LastSeen:         pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}, nil)

	mock.EXPECT().
		GetObserverBrokers(gomock.Any(), observerID).
		Return([]sqlc.GetObserverBrokersRow{
			{
				BrokerName:   "mqtt://test",
				LastPacketAt: pgtype.Timestamptz{Valid: false},
				LastSeen:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
		}, nil)

	mock.EXPECT().
		GetObserverScopes(gomock.Any(), observerID).
		Return([]string{}, nil)

	mock.EXPECT().
		GetObserverLastIATA(gomock.Any(), observerID).
		Return("YVR", nil)

	store := &Store{q: mock}
	observer, err := store.GetObserver(context.Background(), observerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(observer.Brokers) != 1 {
		t.Fatalf("expected 1 broker, got %d", len(observer.Brokers))
	}
	if observer.Brokers[0].LastPacketAt != 0 {
		t.Errorf("expected LastPacketAt 0 for nil, got %d", observer.Brokers[0].LastPacketAt)
	}
}

func TestGetObserver_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	mock.EXPECT().
		GetObserverByID(gomock.Any(), observerID).
		Return(sqlc.Observer{}, errors.New("db error"))

	store := &Store{q: mock}
	_, err := store.GetObserver(context.Background(), observerID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetObserverTelemetry_Mapping(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	reportedAt := pgtype.Timestamptz{Time: time.UnixMilli(1700000000000), Valid: true}
	batteryMV := int32(3700)
	noiseFloor := float32(-90.0)
	uptime := int64(3600)

	mock.EXPECT().
		GetObserverTelemetry(gomock.Any(), gomock.Any()).
		Return([]sqlc.GetObserverTelemetryRow{
			{
				ReportedAt:       reportedAt,
				BatteryVoltageMv: &batteryMV,
				NoiseFloorDb:     &noiseFloor,
				UptimeSeconds:    &uptime,
			},
		}, nil)

	store := &Store{q: mock}
	result, err := store.GetObserverTelemetry(context.Background(), observerID, time.Time{}, time.Time{}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(result.Points))
	}
	if result.Points[0].T != 1700000000000 {
		t.Errorf("expected T 1700000000000, got %d", result.Points[0].T)
	}
	if *result.Points[0].BatteryMV != 3700 {
		t.Errorf("expected BatteryMV 3700, got %d", *result.Points[0].BatteryMV)
	}
}

func TestGetObserverTelemetry_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	mock.EXPECT().
		GetObserverTelemetry(gomock.Any(), gomock.Any()).
		Return([]sqlc.GetObserverTelemetryRow{}, nil)

	store := &Store{q: mock}
	result, err := store.GetObserverTelemetry(context.Background(), observerID, time.Time{}, time.Time{}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Points) != 0 {
		t.Errorf("expected 0 points, got %d", len(result.Points))
	}
}

func TestGetObserverTelemetryBucketed_Mapping(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	bucket := pgtype.Timestamptz{Time: time.UnixMilli(1700000000000), Valid: true}

	mock.EXPECT().
		GetObserverTelemetryBucketed(gomock.Any(), gomock.Any()).
		Return([]sqlc.GetObserverTelemetryBucketedRow{
			{
				Bucket:           bucket,
				BatteryVoltageMv: 3700,
				NoiseFloorDb:     -90.0,
				UptimeSeconds:    3600,
			},
		}, nil)

	store := &Store{q: mock}
	points, err := store.GetObserverTelemetryBucketed(context.Background(), observerID, time.Time{}, time.Time{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].T != 1700000000000 {
		t.Errorf("expected T 1700000000000, got %d", points[0].T)
	}
	if *points[0].BatteryMV != 3700 {
		t.Errorf("expected BatteryMV 3700, got %d", *points[0].BatteryMV)
	}
}

func TestListObserverAdverts_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	heardAt := pgtype.Timestamptz{Time: time.UnixMilli(1700000000000), Valid: true}

	rows := make([]sqlc.ListObserverAdvertsRow, 3)
	for i := range rows {
		rows[i] = sqlc.ListObserverAdvertsRow{
			ID:      int64(i + 1),
			HeardAt: heardAt,
		}
	}

	mock.EXPECT().
		ListObserverAdverts(gomock.Any(), gomock.Any()).
		Return(rows, nil)

	store := &Store{q: mock}
	page, err := store.ListObserverAdverts(context.Background(), observerID, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(page.Items))
	}
	if !page.HasMore {
		t.Error("expected HasMore true")
	}
	if page.NextCursor == nil {
		t.Error("expected NextCursor to be set")
	}
}

func TestListObserverAdverts_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	mock.EXPECT().
		ListObserverAdverts(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("db error"))

	store := &Store{q: mock}
	_, err := store.ListObserverAdverts(context.Background(), observerID, 0, 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetObserverRadio_NilFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	mock.EXPECT().
		GetObserverRadio(gomock.Any(), observerID).
		Return(sqlc.GetObserverRadioRow{}, nil)

	store := &Store{q: mock}
	settings, err := store.GetObserverRadio(context.Background(), observerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.FreqMHz != 0 {
		t.Errorf("expected FreqMHz 0, got %f", settings.FreqMHz)
	}
	if settings.SF != 0 {
		t.Errorf("expected SF 0, got %d", settings.SF)
	}
}

func TestGetObserverRadio_WithFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	observerID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	freq := float32(915.0)
	sf := int16(7)
	bw := float32(125.0)
	cr := int16(5)

	mock.EXPECT().
		GetObserverRadio(gomock.Any(), observerID).
		Return(sqlc.GetObserverRadioRow{
			RadioFreqMhz: &freq,
			RadioSf:      &sf,
			RadioBwKhz:   &bw,
			RadioCr:      &cr,
		}, nil)

	store := &Store{q: mock}
	settings, err := store.GetObserverRadio(context.Background(), observerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.FreqMHz != 915.0 {
		t.Errorf("expected FreqMHz 915.0, got %f", settings.FreqMHz)
	}
	if settings.SF != 7 {
		t.Errorf("expected SF 7, got %d", settings.SF)
	}
	if settings.BWKHz != 125.0 {
		t.Errorf("expected BWKHz 125.0, got %f", settings.BWKHz)
	}
}

func TestIsObserverByPubkey_Found(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	pubkey := []byte{0x01, 0x02}

	mock.EXPECT().
		GetObserverByPubkey(gomock.Any(), pubkey).
		Return(sqlc.Observer{}, nil)

	store := &Store{q: mock}
	if !store.IsObserverByPubkey(context.Background(), pubkey) {
		t.Error("expected true for found observer")
	}
}

func TestIsObserverByPubkey_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := mockdb.NewMockQuerier(ctrl)

	pubkey := []byte{0x01, 0x02}

	mock.EXPECT().
		GetObserverByPubkey(gomock.Any(), pubkey).
		Return(sqlc.Observer{}, errors.New("not found"))

	store := &Store{q: mock}
	if store.IsObserverByPubkey(context.Background(), pubkey) {
		t.Error("expected false for missing observer")
	}
}
