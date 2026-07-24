// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateBorder_ValidPolygon(t *testing.T) {
	raw := []byte(`{
		"type": "Feature",
		"properties": {"name": "Ottawa"},
		"geometry": {
			"type": "Polygon",
			"coordinates": [[[-76.4,45.0],[-75.2,45.0],[-75.2,45.6],[-76.4,45.6],[-76.4,45.0]]]
		}
	}`)
	out, err := ValidateBorder(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var feat struct {
		Type string    `json:"type"`
		BBox []float64 `json:"bbox"`
	}
	if err := json.Unmarshal(out, &feat); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if feat.Type != "Feature" {
		t.Errorf("expected type Feature, got %q", feat.Type)
	}
	wantBBox := []float64{-76.4, 45.0, -75.2, 45.6}
	if len(feat.BBox) != 4 {
		t.Fatalf("expected a 4-element bbox, got %v", feat.BBox)
	}
	for i, v := range wantBBox {
		if feat.BBox[i] != v {
			t.Errorf("bbox[%d] = %v, want %v", i, feat.BBox[i], v)
		}
	}
}

func TestValidateBorder_ValidMultiPolygon(t *testing.T) {
	raw := []byte(`{
		"type": "Feature",
		"properties": {},
		"geometry": {
			"type": "MultiPolygon",
			"coordinates": [
				[[[-76.4,45.0],[-75.2,45.0],[-75.2,45.6],[-76.4,45.6],[-76.4,45.0]]],
				[[[-77.0,44.0],[-76.8,44.0],[-76.8,44.2],[-77.0,44.2],[-77.0,44.0]]]
			]
		}
	}`)
	if _, err := ValidateBorder(raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBorder_UnclosedRing(t *testing.T) {
	raw := []byte(`{
		"type": "Feature",
		"properties": {},
		"geometry": {
			"type": "Polygon",
			"coordinates": [[[-76.4,45.0],[-75.2,45.0],[-75.2,45.6]]]
		}
	}`)
	_, err := ValidateBorder(raw)
	if err == nil {
		t.Fatal("expected an error for an unclosed ring")
	}
	if !strings.Contains(err.Error(), "not closed") {
		t.Errorf("expected a ring-closure error, got: %v", err)
	}
}

func TestValidateBorder_LongitudeOutOfRange(t *testing.T) {
	raw := []byte(`{
		"type": "Feature",
		"properties": {},
		"geometry": {
			"type": "Polygon",
			"coordinates": [[[-200.0,45.0],[-75.2,45.0],[-75.2,45.6],[-200.0,45.6],[-200.0,45.0]]]
		}
	}`)
	_, err := ValidateBorder(raw)
	if err == nil {
		t.Fatal("expected an error for out-of-range longitude")
	}
	if !strings.Contains(err.Error(), "longitude") {
		t.Errorf("expected a longitude range error, got: %v", err)
	}
}

func TestValidateBorder_LatitudeOutOfRange(t *testing.T) {
	raw := []byte(`{
		"type": "Feature",
		"properties": {},
		"geometry": {
			"type": "Polygon",
			"coordinates": [[[-76.4,95.0],[-75.2,95.0],[-75.2,45.6],[-76.4,45.6],[-76.4,95.0]]]
		}
	}`)
	_, err := ValidateBorder(raw)
	if err == nil {
		t.Fatal("expected an error for out-of-range latitude")
	}
	if !strings.Contains(err.Error(), "latitude") {
		t.Errorf("expected a latitude range error, got: %v", err)
	}
}

func TestValidateBorder_WrongGeometryType(t *testing.T) {
	raw := []byte(`{
		"type": "Feature",
		"properties": {},
		"geometry": {"type": "Point", "coordinates": [-75.6692, 45.3225]}
	}`)
	_, err := ValidateBorder(raw)
	if err == nil {
		t.Fatal("expected an error for a non-polygon geometry")
	}
	if !strings.Contains(err.Error(), "Polygon or MultiPolygon") {
		t.Errorf("expected a geometry-type error, got: %v", err)
	}
}

func TestValidateBorder_MalformedJSON(t *testing.T) {
	if _, err := ValidateBorder([]byte(`not json at all`)); err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestValidateBorder_NotAFeature(t *testing.T) {
	raw := []byte(`{
		"type": "FeatureCollection",
		"features": []
	}`)
	_, err := ValidateBorder(raw)
	if err == nil {
		t.Fatal("expected an error for a non-Feature top-level type")
	}
}
