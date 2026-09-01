// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handlers

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/MeshCore-Beacon/beacon-server/internal/api"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const packetIncludeResolvedPath = "resolvedPath"

func parsePacketIncludes(r *http.Request) (bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("include"))
	if raw == "" {
		return false, nil
	}
	includeResolvedPath := false
	for _, part := range strings.Split(raw, ",") {
		include := strings.TrimSpace(part)
		switch include {
		case "":
			continue
		case packetIncludeResolvedPath:
			includeResolvedPath = true
		default:
			return false, fmt.Errorf("unsupported include %q", include)
		}
	}
	return includeResolvedPath, nil
}

// PacketsRouter mounts all /packets routes onto a subrouter.
//
// GET  /packets              → listPackets
// GET  /packets/{packetHash} → getPacket
func PacketsRouter(reader api.Reader) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listPackets(reader))
	r.Get("/backfill", listPacketsBackfill(reader))
	r.Get("/{packetHash}", getPacket(reader))
	return r
}

// listPackets godoc
//
//	@Summary	List packets
//	@Tags		Packets
//	@Produce	json
//	@Param		payloadType		query		int		false	"Filter by payload type integer"
//	@Param		payloadTypes	query		string	false	"Filter by multiple payload types, comma-separated e.g. 2,4"
//	@Param		payloadTypeName	query		string	false	"Filter by payload type name (advert, grp_txt, txt_msg, trace, anon_req)"
//	@Param		routeType		query		int		false	"Filter by route type (0=transport_flood, 1=flood, 2=direct, 3=transport_direct)"
//	@Param		routeTypes		query		string	false	"Filter by multiple route types, comma-separated e.g. 0,1"
//	@Param		iata			query		string	false	"Filter by single IATA code (case-insensitive)"
//	@Param		iatas			query		string	false	"Filter by multiple IATA codes, comma-separated e.g. YVR,YYJ"
//	@Param		scope	query		string	false	"Filter by transport scope name e.g. %23bc (URL-encoded #bc)"
//	@Param		scopes	query		string	false	"Filter by multiple transport scope names, comma-separated e.g. %23bc,%23west"
//	@Param		observer	query		string	false	"Filter by observer UUID"
//	@Param		observers	query		string	false	"Filter by multiple observer UUIDs, comma-separated"
//	@Param		q	query		string	false	"Search text"
//	@Param		searchField	query		string	false	"Search field: hash, path or payload (default hash)"
//	@Param		regionId		query		int		false	"Filter by region ID, expands to member IATAs"
//	@Param		region			query		string	false	"Filter by region slug, expands to member IATAs"
//	@Param		since			query		int		false	"Filter by first_heard_at >= since (epoch ms)"
//	@Param		until			query		int		false	"Filter by first_heard_at <= until (epoch ms)"
//	@Param		cursor			query		int		false	"epoch ms of last item for pagination; last_heard_at, or site-local heard_at when iatas is set"
//	@Param		limit			query		int		false	"Max results (default 50)"
//	@Param		include			query		string	false	"Optional summary enrichment: resolvedPath"
//	@Success	200				{object}	api.Page[api.PacketSummary]
//	@Failure	400				{object}	handlers.APIError
//	@Failure	500				{object}	handlers.APIError
//	@Router		/packets [get]
func listPackets(reader api.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payloadTypes, err := parseInt16CSVOrSingle(r, "payloadTypes", "payloadType")
		if err != nil {
			respondError(w, http.StatusBadRequest, "payloadType/payloadTypes must be integers")
			return
		}
		if len(payloadTypes) == 0 {
			if p := r.URL.Query().Get("payloadTypeName"); p != "" {
				payloadTypes = []int16{api.PayloadTypeFromString(p)}
			}
		}
		routeTypes, err := parseInt16CSVOrSingle(r, "routeTypes", "routeType")
		if err != nil {
			respondError(w, http.StatusBadRequest, "routeType/routeTypes must be integers")
			return
		}
		var since, until time.Time
		if p := r.URL.Query().Get("since"); p != "" {
			ms, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "since must be epoch milliseconds")
				return
			}
			since = time.UnixMilli(ms)
		}
		if p := r.URL.Query().Get("until"); p != "" {
			ms, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "until must be epoch milliseconds")
				return
			}
			until = time.UnixMilli(ms)
		}
		var cursor int64
		if p := r.URL.Query().Get("cursor"); p != "" {
			c, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "cursor must be an integer")
				return
			}
			cursor = c
		}
		var limit int32 = 50
		if p := r.URL.Query().Get("limit"); p != "" {
			l, err := strconv.ParseInt(p, 10, 32)
			if err != nil {
				respondError(w, http.StatusBadRequest, "limit must be an integer")
				return
			}
			limit = int32(l)
		}
		iatas := parseIATAs(r)
		if regionIDStr := r.URL.Query().Get("regionId"); regionIDStr != "" || r.URL.Query().Get("region") != "" {
			regionIATAs, err := resolveRegionIATAs(r.Context(), regionIDStr, r.URL.Query().Get("region"), reader)
			if err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			iatas = append(iatas, regionIATAs...)
		}
		scopes := parseCSVOrSingle(r, "scopes", "scope")
		observerValues := parseCSVOrSingle(r, "observers", "observer")
		observerIDs := make([]uuid.UUID, 0, len(observerValues))
		for _, value := range observerValues {
			id, err := uuid.Parse(value)
			if err != nil {
				respondError(w, http.StatusBadRequest, "observer/observers must contain UUIDs")
				return
			}
			observerIDs = append(observerIDs, id)
		}
		search := strings.TrimSpace(r.URL.Query().Get("q"))
		searchField := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("searchField")))
		if searchField == "" {
			searchField = api.PacketSearchHash
		}
		if !api.ValidPacketSearchField(searchField) {
			respondError(w, http.StatusBadRequest, "searchField must be hash, path or payload")
			return
		}
		if searchField == api.PacketSearchHash || searchField == api.PacketSearchPath {
			search = strings.Map(func(r rune) rune {
				if unicode.IsSpace(r) || r == ':' || r == '-' {
					return -1
				}
				return unicode.ToLower(r)
			}, search)
		}
		includeResolvedPath, err := parsePacketIncludes(r)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		packets, err := reader.ListPackets(r.Context(), api.PacketListParams{
			PayloadTypes: payloadTypes, RouteTypes: routeTypes, IATAs: iatas, Scopes: scopes,
			ObserverIDs: observerIDs, SearchField: searchField, Search: search,
			Since: since, Until: until, LegacyCursor: cursor, Limit: limit,
			IncludeResolvedPath: includeResolvedPath,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		respond(w, http.StatusOK, packets)
	}
}

// listPacketsBackfill godoc
//
//	@Summary	Backfill packets after a given observation ID
//	@Tags		Packets
//	@Produce	json
//	@Param		afterObservationId	query		int		true	"Return packets with observations after this ID (use last WS event observation ID)"
//	@Param		payloadType			query		int		false	"Filter by payload type integer"
//	@Param		payloadTypeName		query		string	false	"Filter by payload type name"
//	@Param		routeType			query		int		false	"Filter by route type"
//	@Param		iatas				query		string	false	"Filter by IATA code(s), comma-separated"
//	@Param		region				query		string	false	"Filter by region slug"
//	@Param		regionId			query		int		false	"Filter by region ID"
//	@Param		scope				query		string	false	"Filter by transport scope name"
//	@Param		limit				query		int		false	"Max results (default 100)"
//	@Param		include				query		string	false	"Optional summary enrichment: resolvedPath"
//	@Success	200					{object}	[]api.PacketSummary
//	@Failure	400					{object}	handlers.APIError
//	@Failure	500					{object}	handlers.APIError
//	@Router		/packets/backfill [get]
func listPacketsBackfill(reader api.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		afterIDStr := r.URL.Query().Get("afterObservationId")
		if afterIDStr == "" {
			respondError(w, http.StatusBadRequest, "afterObservationId is required")
			return
		}
		afterID, err := strconv.ParseInt(afterIDStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "afterObservationId must be an integer")
			return
		}
		var limit int32 = 100
		if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
			l, err := strconv.ParseInt(limitParam, 10, 32)
			if err != nil {
				respondError(w, http.StatusBadRequest, "limit must be an integer")
				return
			}
			limit = int32(l)
		}
		var payloadType int16 = -1
		if v := r.URL.Query().Get("payloadType"); v != "" {
			t, err := strconv.ParseInt(v, 10, 16)
			if err == nil {
				payloadType = int16(t)
			}
		} else if p := r.URL.Query().Get("payloadTypeName"); p != "" {
			payloadType = api.PayloadTypeFromString(p)
		}
		var routeType int16 = -1
		if v := r.URL.Query().Get("routeType"); v != "" {
			t, err := strconv.ParseInt(v, 10, 16)
			if err == nil {
				routeType = int16(t)
			}
		}
		iatas := parseIATAs(r)
		if regionIDStr := r.URL.Query().Get("regionId"); regionIDStr != "" || r.URL.Query().Get("region") != "" {
			regionIATAs, err := resolveRegionIATAs(r.Context(), r.URL.Query().Get("regionId"), r.URL.Query().Get("region"), reader)
			if err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			iatas = append(iatas, regionIATAs...)
		}
		scope := r.URL.Query().Get("scope")
		includeResolvedPath, err := parsePacketIncludes(r)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		packets, err := reader.ListPacketsAfterID(r.Context(), afterID, payloadType, routeType, iatas, scope, limit, includeResolvedPath)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		respond(w, http.StatusOK, packets)
	}
}

// getPacket godoc
//
//	@Summary	Get full packet detail. For trace packets (payloadType=9), includes resolvedRoute
//	@Tags		Packets
//	@Produce	json
//	@Param		packetHash	path		string	true	"Packet hash (hex)"
//	@Success	200			{object}	api.Packet
//	@Failure	400			{object}	handlers.APIError
//	@Failure	404			{object}	handlers.APIError
//	@Failure	500			{object}	handlers.APIError
//	@Router		/packets/{packetHash} [get]
func getPacket(reader api.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hashHex := strings.ToLower(chi.URLParam(r, "packetHash"))
		hash, err := hex.DecodeString(hashHex)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid packet hash")
			return
		}
		packet, err := reader.GetPacket(r.Context(), hash)
		if err != nil {
			respondError(w, http.StatusNotFound, "packet not found")
			return
		}
		respond(w, http.StatusOK, packet)
	}
}
