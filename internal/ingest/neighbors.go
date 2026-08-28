// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingest

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/google/uuid"
)

// neighborReportEntry is one entry in the "neighbors" array of a /neighbors report.
type neighborReportEntry struct {
	PubKey       string  `json:"pubkey"`
	SNR          float32 `json:"snr"`
	HeardSecsAgo int64   `json:"heard_secs_ago"`
	Scopes       string  `json:"scopes"`
	Status       string  `json:"status"` // "responded" or "timeout"
}

// neighborReport is the JSON payload for an observer /neighbors message.
type neighborReport struct {
	Self struct {
		Scopes string `json:"scopes"`
	} `json:"self"`
	Neighbors []neighborReportEntry `json:"neighbors"`
}

// handleNeighbors processes a /neighbors message: the observer's own OTA-configured
// region scope, plus a snapshot of its zero-hop neighbors and (best-effort) their
// OTA-queried region scopes. See the package doc comment for the full pipeline.
//
// A /neighbors report claims DIRECT radio adjacency, so it is validated before
// anything is written: if any reported neighbor is beyond LoRa range of the
// reporter (both endpoints have coordinates and the great-circle distance
// exceeds NeighborMaxKm), the whole report is discarded — packets cross IATA
// areas via MQTT interconnects, and a report containing even one such
// impossible pair is bad data, not something to cherry-pick from.
func (w *Worker) handleNeighbors(ctx context.Context, iata, pubkeyHex string, raw []byte) {
	var report neighborReport
	if err := json.Unmarshal(raw, &report); err != nil {
		log.Printf("ingest[%s]: malformed neighbors envelope from %s: %v", w.cfg.BrokerName, pubkeyHex, err)
		return
	}

	pubkey, err := hex.DecodeString(pubkeyHex)
	if err != nil {
		log.Printf("ingest[%s]: invalid pubkey hex in neighbors from %s: %v", w.cfg.BrokerName, pubkeyHex, err)
		return
	}

	if w.cfg.NeighborMaxKm > 0 {
		if rejected := w.neighborsReportOutOfRange(ctx, pubkey, &report); rejected {
			return
		}
	}

	w.writeNeighbors(ctx, iata, pubkeyHex, pubkey, &report)
}

// neighborsReportOutOfRange checks every reported neighbor against the
// reporter over ALL IATA areas and reports whether any pair is beyond LoRa
// range. Entries that resolve to nothing (or lack coordinates) can't be judged
// and don't block the report.
func (w *Worker) neighborsReportOutOfRange(ctx context.Context, reporterPubkey []byte, report *neighborReport) bool {
	observerNodeID, err := w.db.GetNodeByPubkey(ctx, reporterPubkey)
	if err != nil {
		return false // reporter has no node row: nothing is distance-checkable
	}
	ids := []uuid.UUID{observerNodeID}
	seen := map[uuid.UUID]struct{}{observerNodeID: {}}
	for _, entry := range report.Neighbors {
		nbPubkey, err := hex.DecodeString(entry.PubKey)
		if err != nil {
			continue
		}
		id, err := w.db.GetNodeByPubkey(ctx, nbPubkey)
		if err != nil || id == observerNodeID {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	nodes, err := w.db.GetNodesByIDs(ctx, ids)
	if err != nil || nodes[observerNodeID] == nil {
		return false
	}
	origin := nodes[observerNodeID]
	if origin.Latitude == nil || origin.Longitude == nil {
		return false
	}
	for _, id := range ids[1:] {
		n := nodes[id]
		if n == nil || n.Latitude == nil || n.Longitude == nil {
			continue
		}
		if km := api.HaversineKm(*origin.Latitude, *origin.Longitude, *n.Latitude, *n.Longitude); km > w.cfg.NeighborMaxKm {
			name := "unknown"
			if n.Name != nil {
				name = *n.Name
			}
			log.Printf("ingest[%s]: ignoring neighbors report from %s: %s is %.0f km away, beyond LoRa range",
				w.cfg.BrokerName, reportReporterLabel(reporterPubkey), name, km)
			return true
		}
	}
	return false
}

// reportReporterLabel is a short hex label for log lines.
func reportReporterLabel(pubkey []byte) string {
	const maxLen = 8
	if len(pubkey) > maxLen {
		pubkey = pubkey[:maxLen]
	}
	return hex.EncodeToString(pubkey)
}

func (w *Worker) writeNeighbors(ctx context.Context, iata, pubkeyHex string, pubkey []byte, report *neighborReport) {

	observerID, _, err := w.db.UpsertObserver(ctx, pubkey)
	if err != nil {
		log.Printf("ingest[%s]: db: upsert observer failed in neighbors from %s: %v", w.cfg.BrokerName, pubkeyHex, err)
		return
	}
	if err := w.db.UpsertObserverBroker(ctx, observerID, w.cfg.BrokerName); err != nil {
		log.Printf("ingest[%s]: db: upsert observer broker failed in neighbors from %s: %v", w.cfg.BrokerName, pubkeyHex, err)
	}

	// self.scopes is always known (it's the observer's own config, not an OTA
	// query), so this is an unconditional write -- unlike the per-neighbor
	// scopes below, there's no "query failed" case to protect against here.
	if err := w.db.UpdateObserverRegionScope(ctx, observerID, report.Self.Scopes); err != nil {
		log.Printf("ingest[%s]: db: update observer region scope failed for %s: %v", w.cfg.BrokerName, pubkeyHex, err)
	}

	// The observer's own node row (as opposed to its observers row above) is
	// what node_neighbors edges hang off of. As elsewhere in this package
	// (see GetNodeByPubkey's doc comment), an observer that hasn't advertised
	// yet has no node row, in which case we simply can't record edges for it
	// this round -- not an error, just nothing to attach the neighbors to.
	observerNodeID, err := w.db.GetNodeByPubkey(ctx, pubkey)
	if err != nil {
		return
	}

	for _, n := range report.Neighbors {
		neighborPubkey, err := hex.DecodeString(n.PubKey)
		if err != nil {
			log.Printf("ingest[%s]: invalid neighbor pubkey hex %q from %s: %v", w.cfg.BrokerName, n.PubKey, pubkeyHex, err)
			continue
		}
		neighborNodeID, err := w.db.GetNodeByPubkey(ctx, neighborPubkey)
		if err != nil {
			// Neighbor hasn't advertised yet -- nothing to link to. Its absence
			// here doesn't mean it's gone; it'll show up once it advertises.
			continue
		}
		if neighborNodeID == observerNodeID {
			continue // don't record a node as its own neighbor
		}

		snr := n.SNR

		// OTA region scope queries are unreliable (per the firmware author, even
		// the mobile app sees this) -- only "responded" is a trustworthy read.
		// A "timeout" must not be taken as "the neighbor cleared its scope", so
		// we pass nil and let UpsertNodeNeighbor's COALESCE preserve whatever
		// was last known.
		var regionScope *string
		if n.Status == "responded" {
			regionScope = &n.Scopes
		}

		if err := w.db.UpsertNodeNeighbor(ctx, observerNodeID, neighborNodeID, iata, &snr, regionScope); err != nil {
			log.Printf("ingest[%s]: db: upsert neighbor failed for %s -> %s: %v", w.cfg.BrokerName, pubkeyHex, n.PubKey, err)
		}
	}
}
