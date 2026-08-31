// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingest

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/MeshCore-Beacon/beacon-server/internal/keystore"
	"github.com/google/uuid"
	"github.com/meshcore-go/meshcore-go"
)

// mapKeys is a ChannelKeyStore stub that returns a fixed set of entries for
// any hash, letting tests control whether a channel's key is "known".
type mapKeys struct {
	entries map[byte][]keystore.Entry
}

func (k *mapKeys) GetKey(hash []byte) []keystore.Entry {
	if len(hash) == 0 {
		return nil
	}
	return k.entries[hash[0]]
}

// buildAdvertPacket signs (or, if tamper is true, signs then mutates) an
// advert payload and wraps it in a minimal Packet with no path (zero-hop).
func buildAdvertPacket(t *testing.T, tamper bool) *meshcore.Packet {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	id, err := meshcore.NewIdentityFromBytes(pub)
	if err != nil {
		t.Fatalf("new identity: %v", err)
	}
	advert := &meshcore.Advert{
		PublicKey:  id,
		Timestamp:  12345,
		RawAppData: []byte{meshcore.AdvertTypeRepeater}, // flags byte only, no optional fields
	}
	advert.Sign(priv)
	if tamper {
		// Flip the device-role bits after signing, as if a relay (or an
		// attacker) altered the payload in transit.
		advert.RawAppData = []byte{meshcore.AdvertTypeRoom}
	}
	payload, err := advert.ToBytes()
	if err != nil {
		t.Fatalf("advert to bytes: %v", err)
	}
	return &meshcore.Packet{
		Header:  meshcore.MakeHeader(meshcore.RouteTypeFlood, meshcore.PayloadTypeAdvert, 0),
		Payload: payload,
	}
}

func TestHandlePayloadTypeSideEffects_Advert_ValidSignature_UpsertsNode(t *testing.T) {
	w, db := newTestWorker()
	packet := buildAdvertPacket(t, false)

	w.handlePayloadTypeSideEffects(context.Background(), packet, "TEST", []byte{0x01}, RadioSettings{}, nil, nil, nil, 0)

	if db.upsertNodeCalls != 1 {
		t.Errorf("expected UpsertNode to be called once for a validly-signed advert, got %d", db.upsertNodeCalls)
	}
}

func TestHandlePayloadTypeSideEffects_Advert_MovedNodeInvalidatesAllNodeCaches(t *testing.T) {
	w, db := newTestWorker()
	db.nodeCoordinatesChanged = true
	invalidations := 0
	w.SetCacheInvalidators(
		func(context.Context, uuid.UUID) {},
		func(context.Context) { invalidations++ },
		func(context.Context, uuid.UUID) {},
	)

	w.handlePayloadTypeSideEffects(context.Background(), buildAdvertPacket(t, false), "TEST", []byte{0x01}, RadioSettings{}, nil, nil, nil, 0)

	if invalidations != 1 {
		t.Errorf("expected one full node-cache invalidation, got %d", invalidations)
	}
}

func TestHandlePayloadTypeSideEffects_Advert_InvalidSignature_SkipsUpsert(t *testing.T) {
	w, db := newTestWorker()
	packet := buildAdvertPacket(t, true)

	w.handlePayloadTypeSideEffects(context.Background(), packet, "TEST", []byte{0x01}, RadioSettings{}, nil, nil, nil, 0)

	if db.upsertNodeCalls != 0 {
		t.Errorf("expected UpsertNode NOT to be called for a tampered advert, got %d calls", db.upsertNodeCalls)
	}
}

func buildGrpTxtPacket(t *testing.T, channelHash byte, psk []byte) *meshcore.Packet {
	t.Helper()
	grpTxt, err := (&meshcore.GroupTextPayload{
		Timestamp: 1000,
		Sender:    "ded",
		Text:      "hello",
	}).Encrypt(channelHash, psk)
	if err != nil {
		t.Fatalf("encrypt group text: %v", err)
	}
	payload, err := grpTxt.ToBytes()
	if err != nil {
		t.Fatalf("group text to bytes: %v", err)
	}
	return &meshcore.Packet{
		Header:  meshcore.MakeHeader(meshcore.RouteTypeFlood, meshcore.PayloadTypeGrpTxt, 0),
		Payload: payload,
	}
}

func TestHandlePayloadTypeSideEffects_GrpTxt_KnownKey_OnlyUpsertsKeyedChannel(t *testing.T) {
	w, db := newTestWorker()
	psk := make([]byte, 16)
	channelHash := byte(0x42)
	w.keys = &mapKeys{entries: map[byte][]keystore.Entry{
		channelHash: {{Key: psk, Fingerprint: []byte{0xAA}, Name: "Public", Hashtag: "public"}},
	}}
	packet := buildGrpTxtPacket(t, channelHash, psk)

	w.handlePayloadTypeSideEffects(context.Background(), packet, "TEST", []byte{0x02}, RadioSettings{}, nil, nil, nil, 0)

	if db.upsertChannelCalls != 1 {
		t.Errorf("expected UpsertChannel to be called once, got %d", db.upsertChannelCalls)
	}
	if db.upsertChannelHashOnlyCalls != 0 {
		t.Errorf("expected UpsertChannelHashOnly NOT to be called when the key is known, got %d calls", db.upsertChannelHashOnlyCalls)
	}
}

func TestHandlePayloadTypeSideEffects_GrpTxt_UnknownKey_OnlyUpsertsHashOnlyChannel(t *testing.T) {
	w, db := newTestWorker() // default stubKeys returns no entries for any hash
	channelHash := byte(0x99)
	packet := buildGrpTxtPacket(t, channelHash, make([]byte, 16))

	w.handlePayloadTypeSideEffects(context.Background(), packet, "TEST", []byte{0x03}, RadioSettings{}, nil, nil, nil, 0)

	if db.upsertChannelHashOnlyCalls != 1 {
		t.Errorf("expected UpsertChannelHashOnly to be called once for an unknown-key channel, got %d", db.upsertChannelHashOnlyCalls)
	}
	if db.upsertChannelCalls != 0 {
		t.Errorf("expected UpsertChannel NOT to be called when the key is unknown, got %d calls", db.upsertChannelCalls)
	}
}

// packetEnvelope wraps a packet in the minimal broker JSON that handlePacket expects.
func packetEnvelope(t *testing.T, packet *meshcore.Packet) []byte {
	t.Helper()
	raw, err := packet.ToBytes()
	if err != nil {
		t.Fatalf("packet to bytes: %v", err)
	}
	env, err := json.Marshal(map[string]string{
		"raw":       hex.EncodeToString(raw),
		"timestamp": time.Now().UTC().Format("2006-01-02T15:04:05.000000"),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return env
}

func TestHandlePacket_GrpTxt_UpsertsChannelIATA(t *testing.T) {
	w, db := newTestWorker()
	db.observationInserted = true
	envelope := packetEnvelope(t, buildGrpTxtPacket(t, 0x1a, make([]byte, 16)))

	w.handlePacket(context.Background(), "YOW", "0102", envelope)

	if db.upsertChannelIATACalls != 1 {
		t.Errorf("expected UpsertChannelIATA to be called once for a stored group text, got %d", db.upsertChannelIATACalls)
	}
}

func TestHandlePacket_GrpTxt_DedupObservation_StillUpsertsChannelIATA(t *testing.T) {
	w, db := newTestWorker() // stub reports the observation as a duplicate
	envelope := packetEnvelope(t, buildGrpTxtPacket(t, 0x1a, make([]byte, 16)))

	w.handlePacket(context.Background(), "YOW", "0102", envelope)

	if db.upsertChannelIATACalls != 1 {
		t.Errorf("expected UpsertChannelIATA to run for a duplicate observation too, got %d calls", db.upsertChannelIATACalls)
	}
}

func TestHandlePacket_Advert_SkipsChannelIATA(t *testing.T) {
	w, db := newTestWorker()
	db.observationInserted = true
	envelope := packetEnvelope(t, buildAdvertPacket(t, false))

	w.handlePacket(context.Background(), "YOW", "0102", envelope)

	if db.upsertChannelIATACalls != 0 {
		t.Errorf("expected UpsertChannelIATA NOT to be called for a non-channel packet, got %d calls", db.upsertChannelIATACalls)
	}
	if db.upsertTraceIATACalls != 0 {
		t.Errorf("expected UpsertTraceIATA NOT to be called for a non-trace packet, got %d calls", db.upsertTraceIATACalls)
	}
}

func buildTracePacket(t *testing.T) *meshcore.Packet {
	t.Helper()
	payload, err := (&meshcore.Trace{Tag: 0xdeadbeef, AuthCode: 1}).ToBytes()
	if err != nil {
		t.Fatalf("trace to bytes: %v", err)
	}
	return &meshcore.Packet{
		Header:  meshcore.MakeHeader(meshcore.RouteTypeFlood, meshcore.PayloadTypeTrace, 0),
		Payload: payload,
	}
}

func TestHandlePacket_Trace_UpsertsTraceIATA(t *testing.T) {
	w, db := newTestWorker()
	db.observationInserted = true
	envelope := packetEnvelope(t, buildTracePacket(t))

	w.handlePacket(context.Background(), "YOW", "0102", envelope)

	if db.upsertTraceIATACalls != 1 {
		t.Errorf("expected UpsertTraceIATA to be called once for a stored trace, got %d", db.upsertTraceIATACalls)
	}
}

// attachPath wraps an already-built packet in raw bytes carrying `hashes`
// (each `hashSize` bytes) as its path, parsed back through PacketFromBytes so
// the ingest code sees the header-encoded hash size/count exactly as it would
// on the wire.
func attachPath(t *testing.T, base *meshcore.Packet, hashSize int, hashes [][]byte) *meshcore.Packet {
	t.Helper()
	raw := []byte{base.Header, byte((hashSize-1)<<6 | len(hashes))}
	for _, h := range hashes {
		raw = append(raw, h...)
	}
	raw = append(raw, base.Payload...)
	pkt, err := meshcore.PacketFromBytes(raw)
	if err != nil {
		t.Fatalf("parse packet: %v", err)
	}
	return pkt
}

// buildPathedAdvertPacket signs a repeater advert and gives it a path of
// `count` hashes of `hashSize` bytes (0xA0, 0xA1, ... repeating per hash).
func buildPathedAdvertPacket(t *testing.T, hashSize, count int) *meshcore.Packet {
	t.Helper()
	hashes := make([][]byte, count)
	for i := range hashes {
		hashes[i] = bytes.Repeat([]byte{byte(0xA0 + i)}, hashSize)
	}
	return attachPath(t, buildAdvertPacket(t, false), hashSize, hashes)
}

// A forwarded advert with 3-byte path hashes links the advertiser to its
// first relay AND chains relay to relay — regardless of advert role, so a
// Companion advertising through relays shows up on the map too.
func TestHandlePacket_Advert_ThreeBytePathHashes_UpsertsOriginAndRelayChain(t *testing.T) {
	w, db := newTestWorker()
	db.observationInserted = true
	origin := uuid.New()
	db.nodeByPubkey = &origin
	db.pathResolves = map[string][]api.ResolvedPathEntry{
		"a0a0a0": {{NodeID: uuid.New()}},
		"a1a1a1": {{NodeID: uuid.New()}},
	}

	packet := buildPathedAdvertPacket(t, 3, 2)
	w.handlePacket(context.Background(), "YOW", "0102", packetEnvelope(t, packet))

	if db.upsertNeighborCalls != 2 {
		t.Errorf("expected 2 neighbor upserts (origin->relay, relay->relay), got %d", db.upsertNeighborCalls)
	}
}

// 1- and 2-byte path hashes are too ambiguous to hang a neighbor edge on, so
// no edge is recorded even when the hash resolves cleanly — only 3-byte
// packets, traces with unique hashes, or /neighbors reports confirm neighbors.
func TestHandlePacket_Advert_ShortPathHashes_SkipNeighbor(t *testing.T) {
	for _, hashSize := range []int{1, 2} {
		w, db := newTestWorker()
		db.observationInserted = true
		origin := uuid.New()
		db.nodeByPubkey = &origin
		db.pathResolves = map[string][]api.ResolvedPathEntry{
			hex.EncodeToString(bytes.Repeat([]byte{0xAB}, hashSize)): {{NodeID: uuid.New()}},
		}

		packet := attachPath(t, buildAdvertPacket(t, false), hashSize, [][]byte{bytes.Repeat([]byte{0xAB}, hashSize)})
		w.handlePacket(context.Background(), "YOW", "0102", packetEnvelope(t, packet))

		if db.upsertNeighborCalls != 0 {
			t.Errorf("hash size %d: expected 0 neighbor upserts, got %d", hashSize, db.upsertNeighborCalls)
		}
	}
}

// Group texts carry no sender identity, so only the relay chain is linked:
// a 3-byte path of three hashes yields relay->relay edges, not origin edges.
func TestHandlePacket_GrpTxt_ThreeBytePath_UpsertsRelayChain(t *testing.T) {
	w, db := newTestWorker()
	db.observationInserted = true
	db.pathResolves = map[string][]api.ResolvedPathEntry{
		"a0a0a0": {{NodeID: uuid.New()}},
		"a1a1a1": {{NodeID: uuid.New()}},
		"a2a2a2": {{NodeID: uuid.New()}},
	}

	packet := attachPath(t, buildGrpTxtPacket(t, 0x1a, make([]byte, 16)), 3,
		[][]byte{{0xA0, 0xA0, 0xA0}, {0xA1, 0xA1, 0xA1}, {0xA2, 0xA2, 0xA2}})
	w.handlePacket(context.Background(), "YOW", "0102", packetEnvelope(t, packet))

	if db.upsertNeighborCalls != 2 {
		t.Errorf("expected 2 relay-chain neighbor upserts, got %d", db.upsertNeighborCalls)
	}
}

// buildSNRTracePacket builds a TRACE packet whose payload carries `count`
// path hashes of `hashSize` bytes (the trace Flags field selects the width:
// 1<<(flags&3), so traces can use 1/2/4/8 — there is no 3) and whose path
// carries one recorded SNR byte per consumed hop. Parsed back through
// PacketFromBytes so the ingest code sees it exactly as on the wire.
func buildSNRTracePacket(t *testing.T, hashSize, count int, snrs ...byte) *meshcore.Packet {
	t.Helper()
	var flags byte
	for size := 1; size < hashSize; size *= 2 {
		flags++
	}
	hashes := make([]byte, 0, hashSize*count)
	for i := 0; i < count; i++ {
		hashes = append(hashes, bytes.Repeat([]byte{byte(0xA0 + i)}, hashSize)...)
	}
	payload, err := (&meshcore.Trace{Tag: 0xdeadbeef, AuthCode: 1, Flags: flags, PathHashes: hashes}).ToBytes()
	if err != nil {
		t.Fatalf("trace to bytes: %v", err)
	}
	return &meshcore.Packet{
		Header:     meshcore.MakeHeader(meshcore.RouteTypeFlood, meshcore.PayloadTypeTrace, 0),
		PathLength: byte(len(snrs)), // size code 0 (1B entries), count = len(snrs)
		Path:       snrs,
		Payload:    payload,
	}
}

// 2-byte trace hashes are accepted as neighbor evidence: each hash was
// appended by a node that actually forwarded the trace, and the edge is only
// recorded when the hash resolves to exactly one node — no other node it
// could have been. The per-hop SNR measured by the receiving hop is stored.
func TestHandlePacket_Trace_TwoByteHashes_UniqueResolution_UpsertsHopNeighbor(t *testing.T) {
	w, db := newTestWorker()
	db.observationInserted = true
	db.pathResolves = map[string][]api.ResolvedPathEntry{
		"a0a0": {{NodeID: uuid.New()}},
		"a1a1": {{NodeID: uuid.New()}},
	}

	w.handlePacket(context.Background(), "YOW", "0102", packetEnvelope(t, buildSNRTracePacket(t, 2, 2, 8, 12)))

	if db.upsertNeighborCalls != 1 {
		t.Errorf("expected 1 trace neighbor upsert for a uniquely-resolved 2-byte pair, got %d", db.upsertNeighborCalls)
	}
}

// If a trace hash matches more than one node in the database — i.e. there IS
// another node it could have been — the pair is skipped entirely.
func TestHandlePacket_Trace_AmbiguousResolution_SkipsHopNeighbor(t *testing.T) {
	w, db := newTestWorker()
	db.observationInserted = true
	db.pathResolves = map[string][]api.ResolvedPathEntry{
		"a0a0": {{NodeID: uuid.New()}, {NodeID: uuid.New()}},
		"a1a1": {{NodeID: uuid.New()}},
	}

	w.handlePacket(context.Background(), "YOW", "0102", packetEnvelope(t, buildSNRTracePacket(t, 2, 2, 8, 12)))

	if db.upsertNeighborCalls != 0 {
		t.Errorf("expected 0 trace neighbor upserts for an ambiguous hash, got %d", db.upsertNeighborCalls)
	}
}

// neighborsReport builds the JSON body of a /neighbors MQTT message.
func neighborsReport(selfScopes string, entries ...neighborReportEntry) []byte {
	raw, err := json.Marshal(neighborReport{
		Self: struct {
			Scopes string `json:"scopes"`
		}{Scopes: selfScopes},
		Neighbors: entries,
	})
	if err != nil {
		panic(err)
	}
	return raw
}

func coordsNode(lat, lon float64) *api.ResolvedNode {
	return &api.ResolvedNode{Latitude: &lat, Longitude: &lon}
}

// One reported neighbor beyond LoRa range invalidates the ENTIRE report:
// nothing at all is written from it — no observer, no region scope, no edges.
func TestHandleNeighbors_ImpossibleDistance_IgnoresWholeReport(t *testing.T) {
	w, db := newTestWorker()
	w.cfg.NeighborMaxKm = 150
	observer := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	near := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	far := uuid.MustParse("00000000-0000-0000-0000-00000000000c")
	db.nodesByPubkey = map[string]uuid.UUID{
		"aaaa": observer, // reporter
		"bbbb": near,     // plausible neighbor
		"cccc": far,      // impossible neighbor
	}
	db.nodesByIDs = map[uuid.UUID]*api.ResolvedNode{
		observer: coordsNode(59.61, 16.54), // Västmanland
		near:     coordsNode(59.63, 16.56), // ~2 km away
		far:      coordsNode(55.68, 12.57), // Copenhagen, ~430 km
	}

	w.handleNeighbors(context.Background(), "VST", "aaaa", neighborsReport("SE01",
		neighborReportEntry{PubKey: "bbbb", SNR: 6, Status: "responded"},
		neighborReportEntry{PubKey: "cccc", SNR: -10, Status: "responded"},
	))

	if db.upsertObserverCalls != 0 {
		t.Errorf("expected the observer upsert to be skipped, got %d", db.upsertObserverCalls)
	}
	if len(db.updatedScopes) != 0 {
		t.Errorf("expected no region scope writes, got %v", db.updatedScopes)
	}
	if db.upsertNeighborCalls != 0 {
		t.Errorf("expected 0 neighbor upserts, got %d", db.upsertNeighborCalls)
	}
}

// A report whose neighbors are all within LoRa range is processed normally.
func TestHandleNeighbors_AllInRange_ProcessesReport(t *testing.T) {
	w, db := newTestWorker()
	w.cfg.NeighborMaxKm = 150
	observer := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	near := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	db.nodesByPubkey = map[string]uuid.UUID{
		"aaaa": observer,
		"bbbb": near,
	}
	db.nodesByIDs = map[uuid.UUID]*api.ResolvedNode{
		observer: coordsNode(59.61, 16.54),
		near:     coordsNode(59.63, 16.56),
	}

	w.handleNeighbors(context.Background(), "VST", "aaaa", neighborsReport("SE01",
		neighborReportEntry{PubKey: "bbbb", SNR: 6, Status: "responded"},
	))

	if db.upsertObserverCalls != 1 {
		t.Errorf("expected the observer upsert, got %d", db.upsertObserverCalls)
	}
	if len(db.updatedScopes) != 1 || db.updatedScopes[0] != "SE01" {
		t.Errorf("expected the region scope write, got %v", db.updatedScopes)
	}
	if db.upsertNeighborCalls != 1 {
		t.Errorf("expected 1 neighbor upsert, got %d", db.upsertNeighborCalls)
	}
}
