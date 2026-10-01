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
	"strings"
	"sync"
	"time"
)

type zoneEntry struct {
	url         string
	hasBoundary bool
}

type zoneList struct {
	zones                  map[string]zoneEntry
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
		zones, decodeErr := decodeZones(body, country)
		if decodeErr != nil {
			problem = "invalid response"
		} else {
			list.zones, list.etag = zones, header.Get("ETag")
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

func decodeZones(body []byte, country string) (map[string]zoneEntry, error) {
	var document struct {
		Country string `json:"country"`
		Zones   *[]struct {
			Code        string `json:"code"`
			URL         string `json:"url"`
			HasBoundary bool   `json:"has_boundary"`
		} `json:"zones"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if !strings.EqualFold(document.Country, country) || document.Zones == nil {
		return nil, fmt.Errorf("invalid zone list")
	}
	zones := make(map[string]zoneEntry, len(*document.Zones))
	for _, zone := range *document.Zones {
		zones[strings.ToUpper(zone.Code)] = zoneEntry{url: zone.URL, hasBoundary: zone.HasBoundary}
	}
	return zones, nil
}
