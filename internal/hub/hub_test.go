// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package hub

import (
	"testing"
	"time"
)

func TestScopeMatches_EmptyScope(t *testing.T) {
	// empty scope matches everything — no filters means no restrictions
	s := Scope{}
	e := Event{Type: EventPacketObservation, IATA: "YVR", PayloadType: 4}
	if !scopeMatches(s, e) {
		t.Error("empty scope should match all events")
	}
}

func TestScopeMatches_EventFilter(t *testing.T) {
	s := Scope{Events: []EventType{EventNodeUpdate}}
	if !scopeMatches(s, Event{Type: EventNodeUpdate, IATA: "YVR"}) {
		t.Error("expected nodeUpdate to match")
	}
	if scopeMatches(s, Event{Type: EventPacketObservation, IATA: "YVR"}) {
		t.Error("expected packetObservation not to match")
	}
}

func TestScopeMatches_IATAFilter(t *testing.T) {
	s := Scope{Events: []EventType{EventPacketObservation}, IATAs: []string{"YVR", "YYJ"}}
	if !scopeMatches(s, Event{Type: EventPacketObservation, IATA: "YVR"}) {
		t.Error("expected YVR to match")
	}
	if !scopeMatches(s, Event{Type: EventPacketObservation, IATA: "YYJ"}) {
		t.Error("expected YYJ to match")
	}
	if scopeMatches(s, Event{Type: EventPacketObservation, IATA: "YYC"}) {
		t.Error("expected YYC not to match")
	}
}

func TestScopeMatches_PayloadTypeFilter(t *testing.T) {
	s := Scope{Events: []EventType{EventPacketObservation}, PayloadTypes: []uint8{4}}
	if !scopeMatches(s, Event{Type: EventPacketObservation, PayloadType: 4}) {
		t.Error("expected payload type 4 to match")
	}
	if scopeMatches(s, Event{Type: EventPacketObservation, PayloadType: 5}) {
		t.Error("expected payload type 5 not to match")
	}
}

func TestScopeMatches_ChannelHashFilter(t *testing.T) {
	s := Scope{Events: []EventType{EventChannelMessage}, ChannelHashes: []string{"ab"}}
	if !scopeMatches(s, Event{Type: EventChannelMessage, ChannelHash: "ab"}) {
		t.Error("expected channel hash ab to match")
	}
	if scopeMatches(s, Event{Type: EventChannelMessage, ChannelHash: "cd"}) {
		t.Error("expected channel hash cd not to match")
	}
}

func TestScopeMatches_AllFiltersPass(t *testing.T) {
	s := Scope{
		Events:       []EventType{EventPacketObservation},
		IATAs:        []string{"YVR"},
		PayloadTypes: []uint8{4},
	}
	if !scopeMatches(s, Event{Type: EventPacketObservation, IATA: "YVR", PayloadType: 4}) {
		t.Error("expected all-matching event to pass")
	}
}

func TestScopeMatches_OneFilterFails(t *testing.T) {
	s := Scope{
		Events:       []EventType{EventPacketObservation},
		IATAs:        []string{"YVR"},
		PayloadTypes: []uint8{4},
	}
	if scopeMatches(s, Event{Type: EventPacketObservation, IATA: "YYC", PayloadType: 4}) {
		t.Error("expected wrong IATA to fail")
	}
	if scopeMatches(s, Event{Type: EventPacketObservation, IATA: "YVR", PayloadType: 5}) {
		t.Error("expected wrong payload type to fail")
	}
}

func runHub(t *testing.T) *Hub {
	t.Helper()
	h := New()
	go h.Run()
	return h
}

func TestHub_NewClient_RegistersClient(t *testing.T) {
	h := runHub(t)
	c := h.NewClient()
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.Send == nil {
		t.Error("expected Send channel to be initialized")
	}
}

func TestHub_Broadcast_DeliveredToSubscriber(t *testing.T) {
	h := runHub(t)
	c := h.NewClient()
	h.AddScope(c, "sub1", Scope{Events: []EventType{EventPacketObservation}})

	// give hub time to process
	time.Sleep(10 * time.Millisecond)

	h.Broadcast(Event{Type: EventPacketObservation, IATA: "YVR"})

	select {
	case evt := <-c.Send:
		if evt.Type != EventPacketObservation {
			t.Errorf("expected packetObservation, got %s", evt.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event, timed out")
	}
}

func TestHub_Broadcast_NotDeliveredWithoutMatchingScope(t *testing.T) {
	h := runHub(t)
	c := h.NewClient()
	h.AddScope(c, "sub1", Scope{Events: []EventType{EventNodeUpdate}})

	time.Sleep(10 * time.Millisecond)

	h.Broadcast(Event{Type: EventPacketObservation, IATA: "YVR"})

	select {
	case <-c.Send:
		t.Error("expected no event for non-matching scope")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_Broadcast_NoSubscriptions_NotDelivered(t *testing.T) {
	h := runHub(t)
	c := h.NewClient()

	time.Sleep(10 * time.Millisecond)

	h.Broadcast(Event{Type: EventPacketObservation, IATA: "YVR"})

	select {
	case <-c.Send:
		t.Error("expected no event for client with no subscriptions")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_RemoveScope_StopsDelivery(t *testing.T) {
	h := runHub(t)
	c := h.NewClient()
	h.AddScope(c, "sub1", Scope{Events: []EventType{EventPacketObservation}})

	time.Sleep(10 * time.Millisecond)

	h.RemoveScope(c, "sub1")
	time.Sleep(20 * time.Millisecond) // wait for hub to process RemoveScope

	// drain anything that snuck in before removal was processed
	for len(c.Send) > 0 {
		<-c.Send
	}

	h.Broadcast(Event{Type: EventPacketObservation, IATA: "YVR"})

	select {
	case <-c.Send:
		t.Error("expected no event after scope removed")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_Remove_ClosesChannels(t *testing.T) {
	h := runHub(t)
	c := h.NewClient()

	time.Sleep(10 * time.Millisecond)

	h.Remove(c)

	select {
	case _, ok := <-c.Send:
		if ok {
			t.Error("expected Send channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected Send channel to be closed, timed out")
	}
}

func TestHub_Broadcast_FullBuffer_SendsLaggedNotification(t *testing.T) {
	h := runHub(t)
	c := h.NewClient()
	h.AddScope(c, "sub1", Scope{Events: []EventType{EventPacketObservation}})

	time.Sleep(10 * time.Millisecond)

	// fill the send buffer
	for i := 0; i < cap(c.Send)+10; i++ {
		h.Broadcast(Event{Type: EventPacketObservation, IATA: "YVR"})
	}

	select {
	case notif := <-c.LaggedCH():
		if notif.DroppedCount < 1 {
			t.Errorf("expected DroppedCount >= 1, got %d", notif.DroppedCount)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected lagged notification, timed out")
	}
}

func TestClientMatches_NoSubscriptions(t *testing.T) {
	c := &Client{subscriptions: make(map[string]Scope)}
	if c.matches(Event{Type: EventPacketObservation}) {
		t.Error("expected no match with empty subscriptions")
	}
}

func TestClientMatches_ORSemantics(t *testing.T) {
	c := &Client{
		subscriptions: map[string]Scope{
			"s1": {Events: []EventType{EventNodeUpdate}},
			"s2": {Events: []EventType{EventPacketObservation}},
		},
	}
	if !c.matches(Event{Type: EventPacketObservation}) {
		t.Error("expected match on second scope")
	}
	if !c.matches(Event{Type: EventNodeUpdate}) {
		t.Error("expected match on first scope")
	}
	if c.matches(Event{Type: EventChannelMessage}) {
		t.Error("expected no match for unsubscribed event type")
	}
}
