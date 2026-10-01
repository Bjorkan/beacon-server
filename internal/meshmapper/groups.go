// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package meshmapper

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

// RegionState is a stored region; imported ones are also what SaveImportedRegion writes.
type RegionState struct {
	Slug, Name   string
	DisplayOrder int
	Imported     bool
	IATAs        []string
}

// OnRegionsChange is told whenever the imported regions change, for cache invalidation.
func (z *Zones) OnRegionsChange(fn func(ctx context.Context)) { z.onRegions = fn }

// pruneGroups drops every imported region when group import is off.
func (z *Zones) pruneGroups(ctx context.Context) error {
	pruned, err := z.store.PruneImportedRegions(ctx, nil)
	if err != nil {
		return fmt.Errorf("prune MeshMapper regions: %w", err)
	}
	z.regionsRemoved(pruned)
	if len(pruned) > 0 {
		z.regionsChanged(ctx)
	}
	return nil
}

// syncGroups reconciles imported regions with the loaded zone lists. It only runs
// once every tracked country has a list, and again when a list or the IATA set changes.
func (z *Zones) syncGroups(ctx context.Context, iatas []string) error {
	if !z.importGroups {
		return nil
	}
	var countries []string
	for _, r := range z.regions {
		countries = append(countries, r.country)
	}
	slices.Sort(countries)
	groups, version, complete := z.dir.groups(slices.Compact(countries))
	if !complete || (z.groupsSynced && version == z.groupsVersion && len(iatas) == z.groupsKnown) {
		return nil
	}
	known := make(map[string]bool, len(iatas))
	for _, iata := range iatas {
		known[iata] = true
	}
	state, err := z.store.ListRegionState(ctx)
	if err != nil {
		return fmt.Errorf("list regions for MeshMapper groups: %w", err)
	}
	stored, handOrder := map[string]RegionState{}, 0
	for _, r := range state {
		stored[r.Slug] = r
		if !r.Imported {
			handOrder = max(handOrder, r.DisplayOrder)
		}
	}
	var want []RegionState
	for code, g := range groups {
		if slices.ContainsFunc(g.members, func(m string) bool { return known[m] }) {
			want = append(want, RegionState{Slug: strings.ToLower(code), Name: g.name, Imported: true, IATAs: g.members})
		}
	}
	slices.SortFunc(want, func(a, b RegionState) int { return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Slug, b.Slug)) })
	keep, changed := []string{}, false
	for n, r := range want {
		r.DisplayOrder = handOrder + 1 + n
		if cur, ok := stored[r.Slug]; ok && !cur.Imported {
			slog.Info("MeshMapper group skipped: a configured region uses its slug", "component", "meshmapper.zones", "slug", r.Slug)
			continue
		} else if ok && cur.Name == r.Name && cur.DisplayOrder == r.DisplayOrder && slices.Equal(cur.IATAs, r.IATAs) {
			keep = append(keep, r.Slug)
			continue
		}
		saved, err := z.store.SaveImportedRegion(ctx, r)
		if err != nil {
			return fmt.Errorf("save MeshMapper region %s: %w", r.Slug, err)
		}
		if !saved {
			slog.Info("MeshMapper group skipped: a configured region uses its slug", "component", "meshmapper.zones", "slug", r.Slug)
			continue
		}
		keep, changed = append(keep, r.Slug), true
		slog.Info("MeshMapper region imported", "component", "meshmapper.zones", "slug", r.Slug, "name", r.Name, "iatas", r.IATAs)
	}
	pruned, err := z.store.PruneImportedRegions(ctx, keep)
	if err != nil {
		return fmt.Errorf("prune MeshMapper regions: %w", err)
	}
	z.regionsRemoved(pruned)
	if changed || len(pruned) > 0 {
		z.regionsChanged(ctx)
	}
	z.groupsSynced, z.groupsVersion, z.groupsKnown = true, version, len(iatas)
	return nil
}

func (z *Zones) regionsRemoved(slugs []string) {
	for _, slug := range slugs {
		slog.Info("MeshMapper region removed", "component", "meshmapper.zones", "slug", slug)
	}
}

func (z *Zones) regionsChanged(ctx context.Context) {
	if z.onRegions != nil {
		z.onRegions(ctx)
	}
}
