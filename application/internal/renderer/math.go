package renderer

import (
	"fmt"
	"strconv"
)

// This file should be used for coordinate math and rendering math

const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0

	ARC_SECONDS_TO_MINUTES = 1.0 / 60.0
	ARC_SECONDS_TO_DEGREES = ARC_SECONDS_TO_MINUTES / 60.0
)

// Takes X, Z float coordinates and returns the corresponding map coords e.g.
// 	0.0, 0.0 -> "n00e000"
// 	3600.0, -3600.0 -> "n01w001"
func CoordinateFloatToMap(x, z float32) string {
	lat := int(z / ARC_DEGREES_TO_SECONDS)
	lon := int(x / ARC_DEGREES_TO_SECONDS)

	latDir := 'n'
	if lat < 0 {
		latDir = 's'
		lat = -lat
	}

	lonDir := 'e'
	if lon < 0 {
		lonDir = 'w'
		lon = -lon
	}

	return fmt.Sprintf("%c%02d%c%03d", latDir, lat, lonDir, lon)
}

// Takes a map coordinate and returns the corresponding float x, z coords e.g.
//
//	"n48w123" -> 172800.0, -460800.0
//	"n00e000" -> 0.0, 0.0
//	"s01e001" -> 3600.0, -3600.0
func CoordinateMapToFloat(coord string) (x, z float32) {
	if len(coord) != 7 {
		return 0, 0 // Invalid coordinate format
	}

	latDir := coord[0]
	lonDir := coord[3]
	lat, _ := strconv.Atoi(coord[1:3])
	lon, _ := strconv.Atoi(coord[4:7])

	x = float32(lon) * ARC_DEGREES_TO_SECONDS
	if lonDir == 'w' {
		x = -x
	}

	z = float32(lat) * ARC_DEGREES_TO_SECONDS
	if latDir == 's' {
		z = -z
	}

	return x, z
}
