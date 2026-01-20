package models

import (
	dvec3 "github.com/ungerik/go3d/float64/vec3"
)

// AppConfig holds all application configuration settings
type AppConfig struct {
	// Hardware integration
	UseGPS bool
	UseHat bool

	// Display settings
	MonitorNum int

	// Camera settings
	Fov            float32 // Field of view in Y direction
	CameraStartPos dvec3.T // Starting position in world coordinates
	CameraStartDir dvec3.T // Starting direction vector (normalized)

	// Rendering settings
	RenderDistance         float32 // Maximum render distance
	DisableTerrainWarnings bool    // Disable terrain height warnings
	DisableFog             bool    // Disable fog rendering
	UncapFPS               bool    // Uncap FPS for performance testing
	BenchmarkMode          bool    // Enable benchmark mode for performance tracking

	// Debug settings
	DebugMode bool // Enable debug mode
}
