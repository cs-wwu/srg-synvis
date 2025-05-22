package models

import lin "github.com/xlab/linmath"

// AppConfig holds all application configuration settings
type AppConfig struct {
	// Hardware integration
	UseGPS bool
	UseHat bool

	// Display settings
	MonitorNum int

	// Camera settings
	CameraStartPos lin.Vec3 // Starting position in world coordinates
	CameraStartDir lin.Vec3 // Starting direction vector (normalized)

	// Rendering settings
	RenderDistance         float32 // Maximum render distance
	DisableTerrainWarnings bool    // Disable terrain height warnings
}
