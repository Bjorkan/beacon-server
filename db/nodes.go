// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package db

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	sqlc "github.com/MeshCore-Beacon/beacon-server/db/sqlc"
	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/MeshCore-Beacon/beacon-server/internal/ingest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Store) UpsertNode(ctx context.Context, n ingest.UpsertNodeParams, radio ingest.RadioSettings) (uuid.UUID, bool, error) {
	// advert.Timestamp is only meaningful when the advert actually decoded (AdvertTimestamp
	// is left at its zero value otherwise); a zero epoch timestamp would misreport as ~55
	// years of drift, so only compute/store a delta when it's non-zero.
	var driftSeconds *int32
	if n.AdvertTimestamp != 0 {
		d := int32(int64(n.AdvertTimestamp) - time.Now().Unix())
		driftSeconds = &d
	}
	params := sqlc.UpsertNodeParams{
		PublicKey:               n.PublicKey,
		NodeType:                int16(n.NodeType),
		Name:                    &n.Name,
		Latitude:                n.Latitude,
		Longitude:               n.Longitude,
		DeviceClockDriftSeconds: driftSeconds,
	}
	if radio.FreqMHz != 0 {
		params.RadioFreqMhz = &radio.FreqMHz
		params.RadioSf = &radio.SF
		params.RadioBwKhz = &radio.BWKHz
	}
	row, err := s.q.UpsertNode(ctx, params)
	if err != nil {
		return uuid.Nil, false, err
	}
	return row.ID, row.CoordinatesChanged, nil
}

func (s *Store) UpsertNodeIATA(ctx context.Context, nodeID uuid.UUID, iata string) error {
	params := sqlc.UpsertNodeIATAParams{NodeID: nodeID, Iata: iata}
	return s.q.UpsertNodeIATA(ctx, params)
}

func (s *Store) UpsertNodeShortID(ctx context.Context, nodeID uuid.UUID, iata string, prefix4 []byte) error {
	return s.q.UpsertNodeShortID(ctx, sqlc.UpsertNodeShortIDParams{
		NodeID:  nodeID,
		Iata:    iata,
		Prefix4: prefix4,
	})
}

func (s *Store) UpsertNodeNeighbor(ctx context.Context, nodeID, neighborID uuid.UUID, iata string, snr *float32, regionScope *string) error {
	// A neighbor edge claims the two nodes hear each other over RF, which is
	// impossible beyond a bounded range. When both endpoints report
	// coordinates, refuse links longer than the cap — packets and /neighbors
	// reports cross IATA areas via MQTT interconnects, and those hops are
	// internet hops, not radio hops. Nodes without coordinates pass.
	if s.neighborMaxKm > 0 {
		nodes, err := s.GetNodesByIDs(ctx, []uuid.UUID{nodeID, neighborID})
		if err != nil {
			return err
		}
		a, b := nodes[nodeID], nodes[neighborID]
		if a != nil && b != nil && a.Latitude != nil && b.Latitude != nil && a.Longitude != nil && b.Longitude != nil {
			if km := api.HaversineKm(*a.Latitude, *a.Longitude, *b.Latitude, *b.Longitude); km > s.neighborMaxKm {
				return nil
			}
		}
	}
	return s.q.UpsertNodeNeighbor(ctx, sqlc.UpsertNodeNeighborParams{
		NodeID:      nodeID,
		NeighborID:  neighborID,
		Iata:        iata,
		Snr:         snr,
		RegionScope: regionScope,
	})
}

// DeleteStaleNodeNeighbors drops neighbor edges not re-confirmed since the
// given cutoff (see the cleanup task's neighbor retention).
func (s *Store) DeleteStaleNodeNeighbors(ctx context.Context, cutoff time.Time) error {
	return s.q.DeleteStaleNodeNeighbors(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
}

func (s *Store) SetNodeCapability(ctx context.Context, nodeID uuid.UUID, paths, traces bool) error {
	var errs []error
	if paths {
		errs = append(errs, s.q.SetNodeMultibytePaths(ctx, nodeID))
	}
	if traces {
		errs = append(errs, s.q.SetNodeMultibyteTraces(ctx, nodeID))
	}
	return errors.Join(errs...)
}

func (s *Store) SetNodeDefaultScope(ctx context.Context, nodeID uuid.UUID, scopeID int32) error {
	return s.q.SetNodeDefaultScope(ctx, sqlc.SetNodeDefaultScopeParams{
		ID:             nodeID,
		DefaultScopeID: &scopeID,
	})
}

func (s *Store) ListNodes(ctx context.Context, params api.NodeListParams) (api.Page[api.NodeSummary], error) {
	if params.Sort == "" {
		params.Sort = api.NodeSortLastSeen
	}
	if params.Direction == "" {
		params.Direction = api.SortDesc
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}

	var cursorTS pgtype.Timestamptz
	if params.LegacyCursor > 0 {
		cursorTS = pgtype.Timestamptz{Time: time.UnixMilli(params.LegacyCursor), Valid: true}
	}
	cursorValid, cursorEmpty, cursorKey, cursorID := listCursorValues(params.PageToken)
	rows, err := s.q.ListNodes(ctx, sqlc.ListNodesParams{
		Column1:  params.NodeType,
		Column2:  params.IATAs,
		Column3:  tristate(params.SupportsMultibytePaths),
		Column4:  tristate(params.SupportsMultibyteTraces),
		Column5:  params.PublicKey,
		Column6:  params.Name,
		Column7:  cursorTS,
		Limit:    params.Limit + 1,
		Column9:  params.Scope,
		Column10: params.IncludeNeighbors,
		Column11: params.PubkeyPrefix,
		Column12: params.Sort,
		Column13: string(params.Direction),
		Column14: cursorValid,
		Column15: cursorEmpty,
		Column16: cursorKey,
		Column17: cursorID,
	})
	if err != nil {
		return api.Page[api.NodeSummary]{}, err
	}
	hasMore := len(rows) > int(params.Limit)
	if hasMore {
		rows = rows[:params.Limit]
	}
	items := make([]api.NodeSummary, 0, len(rows))
	for _, v := range rows {
		node := api.NodeSummary{
			ID:                 v.ID,
			PublicKey:          hex.EncodeToString(v.PublicKey),
			NodeType:           v.NodeType,
			NodeTypeName:       api.NodeTypeName(v.NodeType),
			Name:               v.Name,
			Latitude:           v.Latitude,
			Longitude:          v.Longitude,
			IsObserver:         v.IsObserver,
			ObserverID:         nullableUUID(v.ObserverID),
			KnownNeighborCount: v.KnownNeighborCount,
			NeighborIDs:        v.NeighborIds,
			Stale:              v.LastSeen.Valid && v.LastSeen.Time.Before(time.Now().Add(-s.staleThreshold)),
		}
		if len(v.Iatas) > 0 {
			if err := json.Unmarshal(v.Iatas, &node.IATAs); err != nil {
				log.Printf("store: failed to unmarshal node iatas: %v", err)
				node.IATAs = []api.NodeIATA{}
			}
		}
		if v.RadioFreqMhz != nil && v.RadioSf != nil && v.RadioBwKhz != nil {
			s := fmt.Sprintf("%g,%g,%d", *v.RadioFreqMhz, *v.RadioBwKhz, *v.RadioSf)
			node.Radio = &s
		}
		items = append(items, node)
	}
	var nextCursor *int64
	var nextToken *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		// Preserve the historical numeric cursor for clients using the API's legacy newest-first order.
		if params.Sort == api.NodeSortLastSeen && params.Direction == api.SortDesc && last.LastSeen.Valid {
			ms := last.LastSeen.Time.UnixMilli()
			nextCursor = &ms
		}
		token := nextPageToken(api.PageCollectionNodes, params.Sort, params.Direction, last.PageSortEmpty, last.PageSortKey, last.ID)
		nextToken = &token
	}
	return api.Page[api.NodeSummary]{
		Items:         items,
		NextCursor:    nextCursor,
		NextPageToken: nextToken,
		HasMore:       hasMore,
	}, nil
}

func (s *Store) GetNode(ctx context.Context, nodeID uuid.UUID) (*api.Node, error) {
	row, err := s.q.GetNodeByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	node := &api.Node{
		NodeSummary: api.NodeSummary{
			ID:                 row.ID,
			PublicKey:          hex.EncodeToString(row.PublicKey),
			NodeType:           row.NodeType,
			NodeTypeName:       api.NodeTypeName(row.NodeType),
			Name:               row.Name,
			Latitude:           row.Latitude,
			Longitude:          row.Longitude,
			IsObserver:         row.IsObserver,
			ObserverID:         nullableUUID(row.ObserverID),
			DefaultScope:       row.DefaultScopeName,
			KnownNeighborCount: row.KnownNeighborCount,
			Stale:              row.LastSeen.Valid && row.LastSeen.Time.Before(time.Now().Add(-s.staleThreshold)),
		},
		LocationSource:          row.LocationSource,
		SupportsMultibytePaths:  row.SupportsMultibytePaths,
		SupportsMultibyteTraces: row.SupportsMultibyteTraces,
		MinFirmwareVersion:      row.MinFirmwareVersion,
		FirstSeen:               row.FirstSeen.Time.UnixMilli(),
		LastSeen:                row.LastSeen.Time.UnixMilli(),
		Metadata:                row.Metadata,
	}
	neighbors, err := s.GetNodeNeighbors(ctx, nodeID)
	if err != nil {
		log.Printf("store: GetNodeNeighbors failed for %s: %v", nodeID, err)
		neighbors = []api.NodeNeighbor{}
	}
	node.Neighbors = neighbors
	if len(row.Iatas) > 0 {
		if err := json.Unmarshal(row.Iatas, &node.IATAs); err != nil {
			log.Printf("store: failed to unmarshal node iatas: %v", err)
			node.IATAs = []api.NodeIATA{}
		}
	}
	if row.RadioFreqMhz != nil && row.RadioSf != nil && row.RadioBwKhz != nil {
		s := fmt.Sprintf("%g,%g,%d", *row.RadioFreqMhz, *row.RadioBwKhz, *row.RadioSf)
		node.Radio = &s
	}
	if row.LastAdvertAt.Valid {
		ms := row.LastAdvertAt.Time.UnixMilli()
		node.LastAdvertAt = &ms
	}
	// Only repeaters (2) and room servers (3) sign adverts with a device clock worth
	// checking; omit entirely (not just zero) for other node types or an unmeasured node
	// per the API contract, so the frontend can distinguish "unknown" from "in sync".
	if (row.NodeType == 2 || row.NodeType == 3) && row.DeviceClockDriftSeconds != nil && row.LastAdvertAt.Valid {
		drift := int(*row.DeviceClockDriftSeconds)
		node.ClockDriftSeconds = &drift
		checkedAt := row.LastAdvertAt.Time.UnixMilli()
		node.ClockCheckedAt = &checkedAt
		outOfSync := time.Duration(abs(*row.DeviceClockDriftSeconds))*time.Second > s.clockDriftThreshold
		node.ClockOutOfSync = &outOfSync
	}
	return node, nil
}

// abs returns the absolute value of an int32 without overflowing on math.MinInt32.
func abs(n int32) int32 {
	if n < 0 {
		if n == -2147483648 { // math.MinInt32; -n would overflow
			return 2147483647
		}
		return -n
	}
	return n
}

func (s *Store) GetNodesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*api.ResolvedNode, error) {
	rows, err := s.q.GetNodesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID]*api.ResolvedNode, len(rows))
	for _, r := range rows {
		result[r.ID] = &api.ResolvedNode{
			ID:        r.ID,
			Name:      r.Name,
			PublicKey: hex.EncodeToString(r.PublicKey),
			Latitude:  r.Latitude,
			Longitude: r.Longitude,
		}
	}
	return result, nil
}

func (s *Store) GetNodeByPubkey(ctx context.Context, pubkey []byte) (uuid.UUID, error) {
	return s.q.GetNodeByPubkey(ctx, pubkey)
}

func (s *Store) GetNodeNeighbors(ctx context.Context, nodeID uuid.UUID) ([]api.NodeNeighbor, error) {
	rows, err := s.q.GetNodeNeighbors(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	seen := make(map[uuid.UUID]int)
	items := make([]api.NodeNeighbor, 0, len(rows))
	for _, r := range rows {
		if idx, ok := seen[r.ID]; ok {
			items[idx].ObservationCount += r.ObservationCount
			if r.LastSeen.Time.After(time.UnixMilli(items[idx].LastSeen)) {
				items[idx].LastSeen = r.LastSeen.Time.UnixMilli()
				items[idx].IATA = r.Iata
				items[idx].SNR = r.Snr
			}
			if r.FirstSeen.Time.Before(time.UnixMilli(items[idx].FirstSeen)) {
				items[idx].FirstSeen = r.FirstSeen.Time.UnixMilli()
			}
			continue
		}
		seen[r.ID] = len(items)
		items = append(items, api.NodeNeighbor{
			ID:               r.ID,
			Name:             r.Name,
			PublicKey:        hex.EncodeToString(r.PublicKey),
			NodeType:         r.NodeType,
			NodeTypeName:     api.NodeTypeName(r.NodeType),
			Latitude:         r.Latitude,
			Longitude:        r.Longitude,
			IATA:             r.Iata,
			ObservationCount: r.ObservationCount,
			FirstSeen:        r.FirstSeen.Time.UnixMilli(),
			LastSeen:         r.LastSeen.Time.UnixMilli(),
			SNR:              r.Snr,
		})
	}
	return items, nil
}

func (s *Store) ReconfirmNeighbors(ctx context.Context) error {
	return s.q.ReconfirmNeighbors(ctx)
}

// DeleteOldNodes deletes nodes not seen since the given cutoff. See the DeleteOldNodes SQL
// query for the observer_owners exclusion and known_routes caveat.
func (s *Store) DeleteOldNodes(ctx context.Context, cutoff time.Time) error {
	return s.q.DeleteOldNodes(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
}
