// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package meshmapper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

type zoneEntry struct {
	url         string
	hasBoundary bool
}

type zoneGroup struct {
	name    string
	members []string
}

type zoneList struct {
	zones                  map[string]zoneEntry
	groups                 map[string]zoneGroup
	etag                   string
	fetchedAt, nextAttempt time.Time
}

// Directory caches MeshMapper's per-country zone lists, shared by the zones and scopes imports.
type Directory struct {
	mu         sync.Mutex
	client     *http.Client
	listURL    string
	lists      map[string]*zoneList
	retryAfter time.Time
	version    int // bumped whenever a list is replaced
}

func NewDirectory() *Directory {
	return &Directory{listURL: ZonesURL, lists: map[string]*zoneList{}, client: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// pausedUntil is when a list 429 stops pausing every MeshMapper request.
func (d *Directory) pausedUntil() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.retryAfter
}

// List returns the country's fresh zone list. fetched reports that this call
// spent the caller's one request; nil zones without fetched means back off.
func (d *Directory) List(ctx context.Context, country string, now time.Time) (zones map[string]zoneEntry, fetched bool, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	list := d.lists[country]
	if list != nil && list.zones != nil && now.Sub(list.fetchedAt) < zoneListFresh {
		return list.zones, false, nil
	}
	if now.Before(d.retryAfter) || (list != nil && now.Before(list.nextAttempt)) {
		return nil, false, nil
	}
	return nil, true, d.fetch(ctx, country, now)
}

func (d *Directory) fetch(ctx context.Context, country string, now time.Time) error {
	list := d.lists[country]
	if list == nil {
		list = &zoneList{}
		d.lists[country] = list
	}
	endpoint := d.listURL + "?country=" + url.QueryEscape(country)
	etag := ""
	if list.zones != nil {
		etag = list.etag
	}
	status, body, header, err := get(ctx, d.client, "Beacon-MeshMapper-Zones/1", endpoint, etag, MaxZoneList)
	if err != nil {
		return err
	}
	problem := ""
	switch status {
	case http.StatusOK:
		zones, groups, decodeErr := decodeZones(body, country)
		if decodeErr != nil {
			problem = "invalid response"
		} else {
			list.zones, list.groups, list.etag = zones, groups, header.Get("ETag")
			d.version++
		}
	case http.StatusNotModified:
		if list.zones == nil {
			problem = "304 without cached list"
		}
	default:
		var retryAt time.Time
		problem = statusProblem(status, header, now, &retryAt)
		if retryAt.After(list.nextAttempt) {
			list.nextAttempt = retryAt
		}
		if until, ok := rateLimited(status, retryAt, now); ok {
			d.retryAfter = until
		}
	}
	if problem == "" {
		list.fetchedAt = now
	} else if list.nextAttempt.Before(now.Add(zoneFailureRetry)) {
		list.nextAttempt = now.Add(zoneFailureRetry)
	}
	level := slog.LevelInfo
	if problem != "" {
		level = slog.LevelWarn
	}
	slog.Log(ctx, level, "MeshMapper zone list checked", "component", "meshmapper.zones", "country", country,
		"zones", len(list.zones), "last_error", problem)
	return nil
}

// groups merges every loaded list's groups by code. complete is false until each
// country has a list, so a cold start can't look like every group disappearing.
func (d *Directory) groups(countries []string) (merged map[string]zoneGroup, version int, complete bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	merged = map[string]zoneGroup{}
	for _, country := range countries {
		list := d.lists[country]
		if list == nil || list.zones == nil {
			return nil, d.version, false
		}
		for code, g := range list.groups {
			if have, ok := merged[code]; ok {
				g.members = append(slices.Clone(have.members), g.members...)
				slices.Sort(g.members)
				g.members = slices.Compact(g.members)
				g.name = have.name
			}
			merged[code] = g
		}
	}
	return merged, d.version, true
}

var (
	groupCode  = regexp.MustCompile(`^[A-Z0-9]{2,16}$`)
	memberIATA = regexp.MustCompile(`^[A-Z]{3}$`)
)

func decodeZones(body []byte, country string) (map[string]zoneEntry, map[string]zoneGroup, error) {
	var document struct {
		Country string `json:"country"`
		Zones   *[]struct {
			Code        string `json:"code"`
			URL         string `json:"url"`
			HasBoundary bool   `json:"has_boundary"`
		} `json:"zones"`
		Groups []struct {
			Code    string   `json:"code"`
			Name    string   `json:"name"`
			Members []string `json:"members"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, nil, err
	}
	if !strings.EqualFold(document.Country, country) || document.Zones == nil {
		return nil, nil, fmt.Errorf("invalid zone list")
	}
	zones := make(map[string]zoneEntry, len(*document.Zones))
	for _, zone := range *document.Zones {
		zones[strings.ToUpper(zone.Code)] = zoneEntry{url: zone.URL, hasBoundary: zone.HasBoundary}
	}
	groups := map[string]zoneGroup{}
	for _, g := range document.Groups {
		code, name := strings.ToUpper(g.Code), strings.TrimSpace(g.Name)
		if !groupCode.MatchString(code) || name == "" || len(name) > 128 || strings.ContainsFunc(name, unicode.IsControl) {
			continue // a bad group never blocks the zone list
		}
		var members []string
		for _, m := range g.Members {
			if m = strings.ToUpper(m); memberIATA.MatchString(m) {
				members = append(members, m)
			}
		}
		slices.Sort(members)
		if members = slices.Compact(members); len(members) > 0 {
			groups[code] = zoneGroup{name: name, members: members}
		}
	}
	return zones, groups, nil
}
