package coords

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cs-wwu/srg-synvis/core/spatial"
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

	MAP_LENGTH = 3601 // Length of a map tile in arcseconds
)

// Takes X, Z float coordinates and returns the corresponding map coords e.g.
//
//	0, 3600  -> "n00e000"
//	0, 0 	 -> "s01e000"
//	-3600, 0 -> "s01w001"
func WorldToMap(x, z int) spatial.MapCoord {
	z = -z // Flip for vulkan
	// Adjust coordinates to handle the overlap
	lat := z / int(ARC_DEGREES_TO_SECONDS)
	lon := x / int(ARC_DEGREES_TO_SECONDS)
	if z%int(ARC_DEGREES_TO_SECONDS) == 0 || z < 0 {
		lat -= 1
	}
	if x%int(ARC_DEGREES_TO_SECONDS) != 0 && x < 0 {
		lon -= 1
	}

	return spatial.MapCoord{Lat: lat, Lon: lon}
}

// Takes an x, z coordinate and returns the corresponding pixel coordinates.
// This wraps around the image as you move across the world.
// Images start in the top-left, due to vulkan, need to flip z coordinate
func WorldToPixel(x, z int) (int, int) {
	// Handle longitude (x)
	x %= int(ARC_DEGREES_TO_SECONDS)
	if x < 0 {
		x = x + int(ARC_DEGREES_TO_SECONDS)
	}

	// Handle latitude (z)
	z %= int(ARC_DEGREES_TO_SECONDS)
	if z < 0 {
		z = z + int(ARC_DEGREES_TO_SECONDS)
	}
	return x, z
}

// Returns the x, z float as a gps readable value
func WorldToGPS(x, z float32) string {
	return fmt.Sprintf("%f, %f", -z/3600, x/3600)
}

// GPSStringToWorld converts a GPS coordinate string to x, z float coordinates
func GPSStringToWorld(coord string) (float32, float32) {
	// Split the coordinate string into latitude and longitude
	parts := strings.Split(coord, ",")
	if len(parts) != 2 {
		fmt.Printf("invalid GPS coordinate format")
		return 0.0, 0.0
	}

	// Parse latitude
	lat, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 32)

	// Parse longitude
	lon, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32)

	// Convert to x, z float coordinates
	x := float32(lon * 3600)
	z := float32(-lat * 3600)

	return x, z
}

func GPSToWorld(lat, lon float32) (float32, float32) {
	x := lon * 3600
	z := -lat * 3600

	return x, z
}

func WorldToGPSFloat(x, z float32) (float32, float32) {
	// Convert to GPS coordinates
	lat := -z / 3600.0
	lon := x / 3600.0

	return lat, lon
}
