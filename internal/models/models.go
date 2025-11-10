package models

import (
	lin "github.com/xlab/linmath"
)

// AppConfig holds all application configuration settings
type AppConfig struct {
	// Hardware integration
	UseGPS bool
	UseHat bool

	// Display settings
	MonitorNum int

	// Camera settings
	Fov            float32  // Field of view in Y direction
	CameraStartPos lin.Vec3 // Starting position in world coordinates
	CameraStartDir lin.Vec3 // Starting direction vector (normalized)

	// Rendering settings
	RenderDistance         float32 // Maximum render distance
	DisableTerrainWarnings bool    // Disable terrain height warnings
	DisableFog             bool    // Disable fog rendering
	UncapFPS               bool    // Uncap FPS for performance testing
	BenchmarkMode          bool    // Enable benchmark mode for performance tracking

	// Render the world to a file and exit.
	CaptureOnly bool

	// Debug settings
	DebugMode bool // Enable debug mode
}
