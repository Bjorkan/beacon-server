// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestPageTokenRoundTrip(t *testing.T) {
	want := PageToken{
		Version:    PageTokenVersion,
		Collection: PageCollectionNodes,
		Sort:       NodeSortName,
		Direction:  SortAsc,
		Key:        "alpha",
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000123"),
	}

	encoded := EncodePageToken(want)
	got, err := DecodePageToken(encoded)
	if err != nil {
		t.Fatalf("DecodePageToken() error = %v", err)
	}
	if got == nil || *got != want {
		t.Fatalf("DecodePageToken() = %#v, want %#v", got, want)
	}
}

func TestPageTokenRoundTripEmptySortValue(t *testing.T) {
	want := PageToken{
		Version:    PageTokenVersion,
		Collection: PageCollectionObservers,
		Sort:       ObserverSortRadio,
		Direction:  SortDesc,
		Empty:      true,
		Key:        "",
		ID:         uuid.MustParse("00000000-0000-0000-0000-000000000456"),
	}

	got, err := DecodePageToken(EncodePageToken(want))
	if err != nil {
		t.Fatalf("DecodePageToken() error = %v", err)
	}
	if got == nil || *got != want {
		t.Fatalf("DecodePageToken() = %#v, want %#v", got, want)
	}
}

func TestPageTokenRoundTripNumericID(t *testing.T) {
	want := PageToken{
		Version: PageTokenVersion, Collection: PageCollectionRoutes, Sort: RouteSortHops,
		Direction: SortAsc, Key: "00000000000000000002", NumericID: 42,
	}
	got, err := DecodePageToken(EncodePageToken(want))
	if err != nil {
		t.Fatalf("DecodePageToken() error = %v", err)
	}
	if got == nil || *got != want {
		t.Fatalf("DecodePageToken() = %#v, want %#v", got, want)
	}
}

func TestDecodePageTokenRejectsMalformedValues(t *testing.T) {
	tests := []string{
		"not-base64!",
		EncodePageToken(PageToken{Version: PageTokenVersion, Collection: PageCollectionNodes, Sort: NodeSortName, Direction: "sideways", Key: "alpha", ID: uuid.New()}),
		EncodePageToken(PageToken{Version: PageTokenVersion, Collection: PageCollectionNodes, Sort: "", Direction: SortAsc, Key: "alpha", ID: uuid.New()}),
		EncodePageToken(PageToken{Version: PageTokenVersion, Collection: PageCollectionNodes, Sort: NodeSortName, Direction: SortAsc, Key: "alpha", ID: uuid.Nil}),
		EncodePageToken(PageToken{Version: PageTokenVersion, Collection: "", Sort: NodeSortName, Direction: SortAsc, Key: "alpha", ID: uuid.New()}),
		EncodePageToken(PageToken{Version: 99, Collection: PageCollectionNodes, Sort: NodeSortName, Direction: SortAsc, Key: "alpha", ID: uuid.New()}),
		EncodePageToken(PageToken{Version: PageTokenVersion, Collection: PageCollectionNodes, Sort: NodeSortName, Direction: SortAsc, Key: "", ID: uuid.New()}),
	}

	for _, encoded := range tests {
		if _, err := DecodePageToken(encoded); !errors.Is(err, ErrInvalidPageToken) {
			t.Errorf("DecodePageToken(%q) error = %v, want ErrInvalidPageToken", encoded, err)
		}
	}
}

func TestValidListSorts(t *testing.T) {
	for _, sort := range []string{NodeSortLastSeen, NodeSortName, NodeSortType, NodeSortRadio, NodeSortNeighbors} {
		if !ValidNodeSort(sort) {
			t.Errorf("ValidNodeSort(%q) = false", sort)
		}
	}
	if ValidNodeSort("iata") {
		t.Error("ValidNodeSort(iata) = true")
	}

	for _, sort := range []string{ObserverSortLastSeen, ObserverSortName, ObserverSortType, ObserverSortRadio, ObserverSortIATA, ObserverSortStatus} {
		if !ValidObserverSort(sort) {
			t.Errorf("ValidObserverSort(%q) = false", sort)
		}
	}
	if ValidObserverSort("neighbors") {
		t.Error("ValidObserverSort(neighbors) = true")
	}

	for _, sort := range []string{RouteSortIATA, RouteSortHops, RouteSortObservations, RouteSortFirstSeen, RouteSortLastSeen} {
		if !ValidRouteSort(sort) {
			t.Errorf("ValidRouteSort(%q) = false", sort)
		}
	}
	if ValidRouteSort("route") {
		t.Error("ValidRouteSort(route) = true")
	}
}
