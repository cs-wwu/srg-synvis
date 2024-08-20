package renderer

import (
	"fmt"
	"strconv"
)

// This file should be used for coordinate math and rendering math

// Each maps coordinates start in the bottom left of the square. They also
// overlap by a single point for easy matching to the next map.

// World map looks as follows for a single coordinate around (0, 0):
// Each square represents a coordinate degree map
// -3600      0        3599
//  |---------|---------| 3600
//  |         |         |
//  | n00w001 | n00e000 |
//  |         |         |
//  |---------|---------| 0
//  |         |         |
//  | s01w001 | s01e000 |
//  |         |         |
//  |---------|---------| -3599

// There is overlap between coordinates as follows:
// Each square represents a single POINT
//     3599      3600/0      1
//  |---------|---------|---------|
//  |         | n00w001 |         |
//  | n00w001 | n00e000 | n00e000 |   3599
//  |         |         |         |
//  |---------|---------|---------|
//  | n00w001 |         | n00e000 |
//  | s01w001 |  All 4  | s01e000 |  3600/0
//  |         |         |         |
//  |---------|---------|---------|
//  |         | s01w001 |         |
//  | s01w001 | s01e000 | s01e000 |    1
//  |         |         |         |
//  |---------|---------|---------|

const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0

	ARC_SECONDS_TO_MINUTES = 1.0 / 60.0
	ARC_SECONDS_TO_DEGREES = ARC_SECONDS_TO_MINUTES / 60.0
)

// Takes X, Z float coordinates and returns the corresponding map coords e.g.
//
//	0, 3600  -> "n00e000"
//	0, 0 	 -> "s01e000"
//	-3600, 0 -> "s01w001"
func CoordinateIntToMap(x, z int) string {
	lat := int(z/ARC_DEGREES_TO_SECONDS) - 1 // for top-left coord
	lon := int(x / ARC_DEGREES_TO_SECONDS)

	latDir := 'n'
	if lat < 0 {
		latDir = 's'
		lat = -lat // Make positive number
	}

	lonDir := 'e'
	if lon < 0 {
		lonDir = 'w'
		lon = -lon // Make positive number
	}

	return fmt.Sprintf("%c%02d%c%03d", latDir, lat, lonDir, lon)
}

// Takes a map coordinate and returns the corresponding float x, z coords.
// Specifically the top-left coord of the map. e.g.
//
//	"n48w123" -> 176400, -442800
//	"n00w001" -> -3600, 3600
//	"n00e000" -> 0, 3600
//	"s01e001" -> 3600, 0
func CoordinateMapToInt(coord string) (x, z int) {
	if len(coord) != 7 {
		return 0, 0 // Invalid coordinate format
	}

	latDir := coord[0]
	lonDir := coord[3]
	lat, _ := strconv.Atoi(coord[1:3])
	lon, _ := strconv.Atoi(coord[4:7])

	x = lon * ARC_DEGREES_TO_SECONDS
	if lonDir == 'w' {
		x = -x
	}

	z = lat * ARC_DEGREES_TO_SECONDS
	if latDir == 's' {
		z = -z
	}
	z += ARC_DEGREES_TO_SECONDS // for top-left coord

	return x, z
}
