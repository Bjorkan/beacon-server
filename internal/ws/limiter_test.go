// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ws

import (
	"testing"
)

func TestIPLimiter_AcquireWithinLimit(t *testing.T) {
	l := newIPLimiter(3)
	if !l.acquire("1.2.3.4") {
		t.Error("expected acquire to succeed within limit")
	}
	if !l.acquire("1.2.3.4") {
		t.Error("expected second acquire to succeed within limit")
	}
	if !l.acquire("1.2.3.4") {
		t.Error("expected third acquire to succeed at limit")
	}
}

func TestIPLimiter_AcquireAtLimit_Fails(t *testing.T) {
	l := newIPLimiter(2)
	l.acquire("1.2.3.4")
	l.acquire("1.2.3.4")
	if l.acquire("1.2.3.4") {
		t.Error("expected acquire to fail at limit")
	}
}

func TestIPLimiter_Release_AllowsNewConnection(t *testing.T) {
	l := newIPLimiter(1)
	l.acquire("1.2.3.4")
	l.release("1.2.3.4")
	if !l.acquire("1.2.3.4") {
		t.Error("expected acquire to succeed after release")
	}
}

func TestIPLimiter_Release_CleansUpMap(t *testing.T) {
	l := newIPLimiter(2)
	l.acquire("1.2.3.4")
	l.release("1.2.3.4")
	if _, ok := l.count["1.2.3.4"]; ok {
		t.Error("expected map entry to be deleted after count reaches zero")
	}
}

func TestIPLimiter_DifferentIPs_Independent(t *testing.T) {
	l := newIPLimiter(1)
	if !l.acquire("1.2.3.4") {
		t.Error("expected first IP to acquire")
	}
	if !l.acquire("5.6.7.8") {
		t.Error("expected second IP to acquire independently")
	}
	if l.acquire("1.2.3.4") {
		t.Error("expected first IP to be at limit")
	}
}
