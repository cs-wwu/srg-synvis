// Package utils provides utility functions that may be used across packages
package utils

import (
	"fmt"
	
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// Global util for panicking on different errors
func OrPanic(err interface{}) {
	switch v := err.(type) {
	case error:
		if v != nil {
			panic(v)
		}
	case vk.Result:
		if err := vk.Error(v); err != nil {
			panic(err)
		}
	case bool:
		if !v {
			panic("condition failed: != true")
		}
	default:
		// Check if err is nil before panicking
		if err != nil {
			panic(fmt.Sprintf("unsupported type: %T", err))
		}
	}
}
// Takes a Vec3 of (long (Deg), height (m), lat (Deg)) and converts them to
// Vec3 in arc minutes for a global uniform coordinate system
func DegToMins(coords lin.Vec3) lin.Vec3 {
	latitude := ((3600 - coords[0]) / 3600)  // X - Latitude from Degrees
	height := coords[1] * 60.0 / 1852        // Y - Height from Meters
	longitude := ((3600 - coords[2]) / 3600) // Z - Longitude from Degrees

	return lin.Vec3{latitude, height, longitude}
}

// Returns cardinal direction from degrees
func GetDirection(degrees float64) string { // TODO float32/float64
	if degrees < -157.25 {
		return "W"
	} else if degrees < -112.25 {
		return "NW"
	} else if degrees < -67.75 {
		return "N"
	} else if degrees < -22.25 {
		return "NE"
	} else if degrees < 22.25 {
		return "E"
	} else if degrees < 67.75 {
		return "SE"
	} else if degrees < 112.25 {
		return "S"
	} else if degrees < 157.25 {
		return "SW"
	} else {
		return "W"
	}
}

