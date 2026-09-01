// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// SortDirection is the canonical direction accepted by sortable list endpoints.
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// PageToken is an opaque keyset cursor. Callers should round-trip the encoded token unchanged.
// Collection, Sort and Direction bind a cursor to the endpoint and ordering that produced it so a
// token cannot accidentally be reused against another collection or sort.
type PageToken struct {
	Version    int           `json:"v"`
	Collection string        `json:"c"`
	Sort       string        `json:"s"`
	Direction  SortDirection `json:"d"`
	Empty      bool          `json:"e,omitempty"`
	Key        string        `json:"k"`
	ID         uuid.UUID     `json:"i"`
	NumericID  int64         `json:"n,omitempty"`
}

var ErrInvalidPageToken = errors.New("invalid page token")

const (
	PageTokenVersion        = 1
	PageCollectionNodes     = "nodes"
	PageCollectionObservers = "observers"
	PageCollectionRoutes    = "routes"
)

func EncodePageToken(token PageToken) string {
	b, _ := json.Marshal(token) // PageToken contains only JSON-safe scalar values.
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodePageToken(encoded string) (*PageToken, error) {
	if encoded == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidPageToken
	}
	var token PageToken
	if err := json.Unmarshal(b, &token); err != nil || token.Version != PageTokenVersion || token.Collection == "" || token.Sort == "" || (token.ID == uuid.Nil && token.NumericID == 0) || (!token.Empty && token.Key == "") || (token.Direction != SortAsc && token.Direction != SortDesc) {
		return nil, ErrInvalidPageToken
	}
	return &token, nil
}

// Node list sort keys. last_seen is the legacy/default API order and is also used by the map.
const (
	NodeSortLastSeen  = "last_seen"
	NodeSortName      = "name"
	NodeSortType      = "type"
	NodeSortRadio     = "radio"
	NodeSortNeighbors = "neighbors"
)

func ValidNodeSort(sort string) bool {
	switch sort {
	case NodeSortLastSeen, NodeSortName, NodeSortType, NodeSortRadio, NodeSortNeighbors:
		return true
	default:
		return false
	}
}

// Observer list sort keys.
const (
	ObserverSortLastSeen = "last_seen"
	ObserverSortName     = "name"
	ObserverSortType     = "type"
	ObserverSortRadio    = "radio"
	ObserverSortIATA     = "iata"
	ObserverSortStatus   = "status"
)

func ValidObserverSort(sort string) bool {
	switch sort {
	case ObserverSortLastSeen, ObserverSortName, ObserverSortType, ObserverSortRadio, ObserverSortIATA, ObserverSortStatus:
		return true
	default:
		return false
	}
}

// Packet search fields accepted by GET /packets.
const (
	PacketSearchHash    = "hash"
	PacketSearchPath    = "path"
	PacketSearchPayload = "payload"
)

func ValidPacketSearchField(field string) bool {
	switch field {
	case PacketSearchHash, PacketSearchPath, PacketSearchPayload:
		return true
	default:
		return false
	}
}

// Route list sort keys.
const (
	RouteSortIATA         = "iata"
	RouteSortHops         = "hops"
	RouteSortObservations = "observations"
	RouteSortFirstSeen    = "first_seen"
	RouteSortLastSeen     = "last_seen"
)

func ValidRouteSort(sort string) bool {
	switch sort {
	case RouteSortIATA, RouteSortHops, RouteSortObservations, RouteSortFirstSeen, RouteSortLastSeen:
		return true
	default:
		return false
	}
}

type NodeListParams struct {
	NodeType                int16
	IATAs                   []string
	SupportsMultibytePaths  *bool
	SupportsMultibyteTraces *bool
	PublicKey               []byte
	PubkeyPrefix            string
	Name                    string
	Scope                   string
	LegacyCursor            int64
	PageToken               *PageToken
	Sort                    string
	Direction               SortDirection
	Limit                   int32
	IncludeNeighbors        bool
}

type ObserverListParams struct {
	IATAs        []string
	ObserverType string
	Broker       string
	Status       string
	Name         string
	Scope        string
	LegacyCursor int64
	PageToken    *PageToken
	Sort         string
	Direction    SortDirection
	Limit        int32
}

type PacketListParams struct {
	PayloadTypes        []int16
	RouteTypes          []int16
	IATAs               []string
	Scopes              []string
	ObserverIDs         []uuid.UUID
	SearchField         string
	Search              string
	Since               time.Time
	Until               time.Time
	LegacyCursor        int64
	Limit               int32
	IncludeResolvedPath bool
}

type RouteListParams struct {
	IATAs        []string
	HopCount     int32
	LegacyCursor time.Time
	PageToken    *PageToken
	Sort         string
	Direction    SortDirection
	Limit        int32
}
