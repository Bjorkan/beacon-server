// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"os"
	"testing"
)

func TestLoad_FileNotFound(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected empty config, got nil")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	f, err := os.CreateTemp("", "beacon-config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	_, _ = f.WriteString(`
iatas:
  YVR:
    name: Vancouver
    lat: 49.1967
    lng: -123.1815
regions:
  - slug: bc
    name: British Columbia
    display_order: 1
    iatas: [YVR]
`)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.IATAs["YVR"]; !ok {
		t.Error("expected YVR in IATAs")
	}
	if len(cfg.Regions) != 1 {
		t.Errorf("expected 1 region, got %d", len(cfg.Regions))
	}
	if cfg.Regions[0].Slug != "bc" {
		t.Errorf("expected slug bc, got %s", cfg.Regions[0].Slug)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	f, err := os.CreateTemp("", "beacon-config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	_, _ = f.WriteString(`not: valid: yaml: [`)
	f.Close()

	_, err = Load(f.Name())
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}
