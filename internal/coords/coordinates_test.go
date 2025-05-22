package coords

import (
	"testing"
)

// TestMapToWorldRoundtrip tests that converting from map coordinates to world coordinates
// and back results in the original map coordinates
func TestMapToWorldRoundtrip(t *testing.T) {
	tests := []struct {
		name  string
		coord string
	}{
		{"SeattleArea", "n48w123"},
		{"NorthwestQuadrant", "n00w001"},
		{"NortheastQuadrant", "n00e000"},
		{"SouthwestQuadrant", "s01w001"},
		{"SoutheastQuadrant", "s01e000"},
		{"OuterNorthwest", "n01w002"},
		{"OuterNortheast", "n01e001"},
		{"OuterSouthwest", "s02w002"},
		{"OuterSoutheast", "s02e001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, z := MapToWorld(tt.coord)
			resultCoord := WorldToMap(x, z)

			if resultCoord != tt.coord {
				t.Errorf("MapToWorld -> WorldToMap roundtrip failed for %s: got %s, want %s",
					tt.coord, resultCoord, tt.coord)
				t.Errorf("World coordinates: (%d, %d)", x, z)
			}
		})
	}
}

// TestWorldToMap tests that world coordinates map to the expected map tile
func TestWorldToMap(t *testing.T) {
	tests := []struct {
		name     string
		x        int
		z        int
		expected string
	}{
		{"SeattleTopLeft", -442800, -176400, "n48w123"},
		{"SeattleMiddle", -440000, -174000, "n48w123"},
		{"SeattleBottomRight", -439201, -172801, "n48w123"},

		{"NorthwestTopLeft", -3600, -3600, "n00w001"},
		{"NorthwestMiddle", -1000, -1000, "n00w001"},
		{"NorthwestBottomRight", -1, -1, "n00w001"},

		{"NortheastTopLeft", 0, -3600, "n00e000"},
		{"NortheastMiddle", 1000, -1000, "n00e000"},
		{"NortheastBottomRight", 3599, -1, "n00e000"},

		{"SouthwestTopLeft", -3600, 0, "s01w001"},
		{"SouthwestMiddle", -1000, 1000, "s01w001"},
		{"SouthwestBottomRight", -1, 3599, "s01w001"},

		{"SoutheastTopLeft", 0, 0, "s01e000"},
		{"SoutheastMiddle", 1000, 1000, "s01e000"},
		{"SoutheastBottomRight", 3599, 3599, "s01e000"},

		{"OuterNorthwestTopLeft", -7200, -7200, "n01w002"},
		{"OuterNorthwestMiddle", -4000, -4000, "n01w002"},
		{"OuterNorthwestBottomRight", -3601, -3601, "n01w002"},

		{"OuterNortheastTopLeft", 3600, -3601, "n01e001"},
		{"OuterNortheastMiddle", 4000, -4000, "n01e001"},
		{"OuterNortheastBottomRight", 7199, -7200, "n01e001"},

		{"OuterSouthwestTopLeft", -7200, 3600, "s02w002"},
		{"OuterSouthwestMiddle", -4000, 4000, "s02w002"},
		{"OuterSouthwestBottomRight", -3601, 7199, "s02w002"},

		{"OuterSoutheastTopLeft", 3600, 3600, "s02e001"},
		{"OuterSoutheastMiddle", 4000, 4000, "s02e001"},
		{"OuterSoutheastBottomRight", 7199, 7199, "s02e001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WorldToMap(tt.x, tt.z)
			if result != tt.expected {
				t.Errorf("WorldToMap(%d, %d) = %s; want %s",
					tt.x, tt.z, result, tt.expected)
			}
		})
	}
}

// TestWorldToPixel tests conversion from world coordinates to image pixel coordinates
func TestWorldToPixel(t *testing.T) {
	tests := []struct {
		name      string
		worldX    int
		worldZ    int
		expectedX int
		expectedZ int
	}{
		{"OriginCorners1", -3600, 3600, 0, 0},
		{"OriginCorners2", -3600, 0, 0, 0},
		{"OriginCorners3", 0, 0, 0, 0},
		{"OriginCorners4", 0, 3600, 0, 0},

		{"NearOriginTopLeft", -3580, 3580, 20, 3580},
		{"NearOriginTopRight", -3580, -20, 20, 3580},
		{"NearOriginBottomLeft", 20, 3580, 20, 3580},
		{"NearOriginBottomRight", 20, -20, 20, 3580},

		{"SeattleArea1", -442800, 176400, 0, 0},
		{"SeattleArea2", -442800, 172800, 0, 0},
		{"SeattleArea3", -446400, 176400, 0, 0},
		{"SeattleArea4", -446400, 172800, 0, 0},

		{"SeattleAreaOffset", -446399, 172801, 1, 1},

		{"ZeroPlane1", 100, -100, 100, 3500},
		{"ZeroPlane2", 0, -100, 0, 3500},
		{"ZeroPlane3", 100, 100, 100, 100},
		{"ZeroPlane4", 0, 100, 0, 100},

		{"NegativePlane1", -100, 100, 3500, 100},
		{"NegativePlane2", -100, 0, 3500, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultX, resultZ := WorldToPixel(tt.worldX, tt.worldZ)
			if resultX != tt.expectedX || resultZ != tt.expectedZ {
				t.Errorf("WorldToPixel(%d, %d) = (%d, %d); want (%d, %d)",
					tt.worldX, tt.worldZ, resultX, resultZ, tt.expectedX, tt.expectedZ)
			}
		})
	}
}

// TestWorldToGPSFloat tests conversion from world coordinates to GPS coordinates
func TestWorldToGPSFloat(t *testing.T) {
	tests := []struct {
		name        string
		worldX      float32
		worldZ      float32
		expectedLat float32
		expectedLon float32
	}{
		{"Origin", 0, 0, 0, 0},
		{"Seattle", -441360, -163800, 45.5, -122.6}, // Value from failing test
		{"PositiveCoords", 3600, -3600, 1.0, 1.0},
		{"NegativeCoords", -3600, 3600, -1.0, -1.0},
		{"MixedCoords1", 3600, 3600, -1.0, 1.0},
		{"MixedCoords2", -3600, -3600, 1.0, -1.0},
		{"LargeValues", 360000, -360000, 100.0, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon := WorldToGPSFloat(tt.worldX, tt.worldZ)

			// Use epsilon for floating point comparison
			const epsilon = 0.00001
			if abs(lat-tt.expectedLat) > epsilon || abs(lon-tt.expectedLon) > epsilon {
				t.Errorf("WorldToGPSFloat(%f, %f) = (%f, %f); want (%f, %f)",
					tt.worldX, tt.worldZ, lat, lon, tt.expectedLat, tt.expectedLon)
			}
		})
	}
}

// TestGPSToWorld tests conversion from GPS coordinates to world coordinates
func TestGPSToWorld(t *testing.T) {
	tests := []struct {
		name      string
		lat       float32
		lon       float32
		expectedX float32
		expectedZ float32
	}{
		{"Origin", 0, 0, 0, 0},
		{"Seattle", 45.5, -122.6, -441360, -163800}, // Value from failing test
		{"PositiveCoords", 1.0, 1.0, 3600, -3600},
		{"NegativeCoords", -1.0, -1.0, -3600, 3600},
		{"MixedCoords1", -1.0, 1.0, 3600, 3600},
		{"MixedCoords2", 1.0, -1.0, -3600, -3600},
		{"LargeValues", 100.0, 100.0, 360000, -360000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, z := GPSToWorld(tt.lat, tt.lon)

			// Use epsilon for floating point comparison
			const epsilon = 0.00001
			if abs(x-tt.expectedX) > epsilon || abs(z-tt.expectedZ) > epsilon {
				t.Errorf("GPSToWorld(%f, %f) = (%f, %f); want (%f, %f)",
					tt.lat, tt.lon, x, z, tt.expectedX, tt.expectedZ)
			}
		})
	}
}

// TestGPSConversionRoundtrip tests that converting from GPS to world and back returns the original GPS coordinates
func TestGPSConversionRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		lat  float32
		lon  float32
	}{
		{"Origin", 0, 0},
		{"Seattle", 45.5, -122.6},
		{"PositiveCoords", 1.0, 1.0},
		{"NegativeCoords", -1.0, -1.0},
		{"MixedCoords1", -1.0, 1.0},
		{"MixedCoords2", 1.0, -1.0},
		{"LargeValues", 100.0, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, z := GPSToWorld(tt.lat, tt.lon)
			resultLat, resultLon := WorldToGPSFloat(x, z)

			// Use epsilon for floating point comparison
			const epsilon = 0.00001
			if abs(resultLat-tt.lat) > epsilon || abs(resultLon-tt.lon) > epsilon {
				t.Errorf("Roundtrip conversion failed for (%f, %f): got (%f, %f)",
					tt.lat, tt.lon, resultLat, resultLon)
			}
		})
	}
}

// Helper function for float comparison
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
