// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
)

type stubSeeder struct {
	regions     map[string]int32
	iatas       []string
	regionIATAs map[int32][]string
	scopes      []string
	upsertErr   error
}

func newStubSeeder() *stubSeeder {
	return &stubSeeder{
		regions:     make(map[string]int32),
		regionIATAs: make(map[int32][]string),
	}
}

func (s *stubSeeder) UpsertIATA(_ context.Context, iata string) error {
	s.iatas = append(s.iatas, iata)
	return s.upsertErr
}

func (s *stubSeeder) UpsertIATADetails(_ context.Context, iata, _ string, _, _ *float64) error {
	s.iatas = append(s.iatas, iata)
	return s.upsertErr
}

func (s *stubSeeder) UpsertRegion(_ context.Context, slug, _, _ string, _ int, _, _ *float64, _ *int) (int32, error) {
	id := int32(len(s.regions) + 1)
	s.regions[slug] = id
	return id, s.upsertErr
}

func (s *stubSeeder) UpsertRegionIATA(_ context.Context, regionID int32, iata string) error {
	s.regionIATAs[regionID] = append(s.regionIATAs[regionID], iata)
	return s.upsertErr
}

func (s *stubSeeder) UpsertTransportScope(_ context.Context, name, _ string, _, _ []byte) error {
	s.scopes = append(s.scopes, name)
	return s.upsertErr
}

func TestSeed_IATAsAndRegions(t *testing.T) {
	cfg := &Config{
		IATAs: map[string]IATAConfig{
			"YVR": {Name: "Vancouver"},
		},
		Regions: []RegionConfig{
			{Slug: "bc", Name: "British Columbia", IATAs: []string{"YVR", "YYJ"}},
		},
	}

	db := newStubSeeder()
	if err := Seed(context.Background(), cfg, db); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := db.regions["bc"]; !ok {
		t.Error("expected bc region to be upserted")
	}
	if len(db.regionIATAs[1]) != 2 {
		t.Errorf("expected 2 IATAs for region 1, got %d", len(db.regionIATAs[1]))
	}
}

func TestSeed_Scopes(t *testing.T) {
	cfg := &Config{
		Scopes: []ScopeConfig{
			{Name: "bc"},
			{Name: "#west"},
		},
	}

	db := newStubSeeder()
	if err := Seed(context.Background(), cfg, db); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(db.scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(db.scopes))
	}
	if db.scopes[0] != "#bc" {
		t.Errorf("expected #bc, got %s", db.scopes[0])
	}
	if db.scopes[1] != "#west" {
		t.Errorf("expected #west, got %s", db.scopes[1])
	}
}

func TestSeed_DBError(t *testing.T) {
	cfg := &Config{
		IATAs: map[string]IATAConfig{
			"YVR": {Name: "Vancouver"},
		},
	}

	db := newStubSeeder()
	db.upsertErr = errors.New("db error")

	if err := Seed(context.Background(), cfg, db); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNormalizeScopeName_WithHash(t *testing.T) {
	if normalizeScopeName("#bc") != "#bc" {
		t.Error("expected #bc unchanged")
	}
}

func TestNormalizeScopeName_WithDollar(t *testing.T) {
	if normalizeScopeName("$bc") != "$bc" {
		t.Error("expected $bc unchanged")
	}
}

func TestNormalizeScopeName_WithoutPrefix(t *testing.T) {
	if normalizeScopeName("bc") != "#bc" {
		t.Error("expected bc to become #bc")
	}
}

func TestNormalizeScopeName_Empty(t *testing.T) {
	if normalizeScopeName("") != "#" {
		t.Error("expected empty string to become #")
	}
}

func TestDeriveScopeKey_Length(t *testing.T) {
	key := deriveScopeKey("#bc")
	if len(key) != 16 {
		t.Errorf("expected 16 bytes, got %d", len(key))
	}
}

func TestDeriveScopeKey_Deterministic(t *testing.T) {
	a := deriveScopeKey("#bc")
	b := deriveScopeKey("#bc")
	if hex.EncodeToString(a) != hex.EncodeToString(b) {
		t.Error("expected same key for same input")
	}
}

func TestDeriveScopeKey_KnownValue(t *testing.T) {
	// SHA256("#bc")[:16] — pin the exact derivation so changes are caught
	key := deriveScopeKey("#bc")
	got := hex.EncodeToString(key)
	// generate this once: echo -n "#bc" | sha256sum | cut -c1-32
	const want = "84509cfe73d94f7f6a8299e6bcdb8a3c"
	if got != want {
		t.Errorf("deriveScopeKey(\"#bc\") = %s, want %s", got, want)
	}
}

func TestDeriveScopeKey_DifferentInputs(t *testing.T) {
	a := deriveScopeKey("#bc")
	b := deriveScopeKey("#other")
	if hex.EncodeToString(a) == hex.EncodeToString(b) {
		t.Error("expected different keys for different inputs")
	}
}
