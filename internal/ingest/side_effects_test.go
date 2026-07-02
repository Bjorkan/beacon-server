// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingest

import (
	"context"
	"crypto/ed25519"
	"testing"

	"github.com/MeshCore-Beacon/beacon-server/internal/keystore"
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
