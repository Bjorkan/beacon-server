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

// buildPathedAdvertPacket signs a repeater advert and wraps it in a packet
// carrying `count` path hashes of `hashSize` bytes (flood route, no transport
// codes), parsed back through PacketFromBytes so the ingest code sees the
// header-encoded hash size/count exactly as it would on the wire.
func buildPathedAdvertPacket(t *testing.T, hashSize, count int) *meshcore.Packet {
	t.Helper()
	base := buildAdvertPacket(t, false)
	raw := []byte{base.Header, byte((hashSize-1)<<6 | count)}
	for i := 0; i < count; i++ {
		raw = append(raw, bytes.Repeat([]byte{byte(0xAB - i)}, hashSize)...)
	}
	raw = append(raw, base.Payload...)
	pkt, err := meshcore.PacketFromBytes(raw)
	if err != nil {
		t.Fatalf("parse packet: %v", err)
	}
	return pkt
}

// A forwarded repeater advert with 3-byte path hashes resolves its first hop
// and records the advertiser->first-hop neighbor edge.
func TestHandlePayloadTypeSideEffects_Advert_ThreeBytePathHashes_UpsertsFirstHopNeighbor(t *testing.T) {
	packet := buildPathedAdvertPacket(t, 3, 1)
	w, db := newTestWorker()
	db.pathResolves = map[string][]api.ResolvedPathEntry{
		hex.EncodeToString(packet.PathHashes()[0]): {{NodeID: uuid.New()}},
	}

	w.handlePayloadTypeSideEffects(context.Background(), packet, "TEST", []byte{0x01}, RadioSettings{}, nil, nil, nil, 0)

	if db.upsertNeighborCalls != 1 {
		t.Errorf("expected 1 neighbor upsert for a 3-byte path hash, got %d", db.upsertNeighborCalls)
	}
}

// 1- and 2-byte path hashes are too ambiguous to hang a neighbor edge on, so
// no edge is recorded even when the hash resolves cleanly — only a 3-byte
// packet or a /neighbors report may confirm neighbors.
func TestHandlePayloadTypeSideEffects_Advert_ShortPathHashes_SkipNeighbor(t *testing.T) {
	for _, hashSize := range []int{1, 2} {
		packet := buildPathedAdvertPacket(t, hashSize, 1)
		w, db := newTestWorker()
		db.pathResolves = map[string][]api.ResolvedPathEntry{
			hex.EncodeToString(packet.PathHashes()[0]): {{NodeID: uuid.New()}},
		}

		w.handlePayloadTypeSideEffects(context.Background(), packet, "TEST", []byte{0x01}, RadioSettings{}, nil, nil, nil, 0)

		if db.upsertNeighborCalls != 0 {
			t.Errorf("hash size %d: expected 0 neighbor upserts, got %d", hashSize, db.upsertNeighborCalls)
		}
	}
}
