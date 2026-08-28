// Copyright 2026 Beacon Contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import "math"

// HaversineKm returns the great-circle distance between two coordinates in km.
// Used to sanity-check neighbor claims: two nodes further apart than a direct
// LoRa hop can possibly reach are not neighbors, no matter what a packet path
// or a /neighbors report says.
func HaversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthKm = 6371.0
	rad := math.Pi / 180
	phi1, phi2 := lat1*rad, lat2*rad
	dPhi := (lat2 - lat1) * rad
	dLambda := (lon2 - lon1) * rad
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return earthKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
