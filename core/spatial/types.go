package spatial

import (
	"fmt"
	"math"

	lin "github.com/xlab/linmath"
)

// GridCoord represents discrete coordinates in the grid
type GridCoord struct {
	X, Z int
}

// MapCoord represents a 1-degree by 1-degree chunk on the map
type MapCoord struct {
	Lat, Lon int
}

// String returns a string representation of GridCoord
func (c GridCoord) String() string {
	return fmt.Sprintf("(%d,%d)", c.X, c.Z)
}

// Direction enum for neighbor lookup
type Direction int

const (
	North Direction = iota
	East
	South
	West
	DirectionCount // Total number of directions
)

// DirectionNames maps Direction enum values to string names
var DirectionNames = map[Direction]string{
	North: "North",
	East:  "East",
	South: "South",
	West:  "West",
}

// ChunkSpec represents a desired chunk specification
type ChunkSpec struct {
	GridCoord GridCoord // Grid coordinates
	WorldPos  lin.Vec2  // World position (center of chunk)
	Distance  float32   // Distance from camera
	LODLevel  int       // Calculated LOD level
	Priority  float32   // Loading priority (higher = more important)
	InFrustum bool      // Whether chunk is in view frustum
}

// ChunkSelectorStats holds statistics about the chunk selection system
type ChunkSelectorStats struct {
	DesiredChunks   int      // Number of chunks that should exist
	AvailableChunks int      // Number of chunks marked as available
	MissingChunks   int      // Number of desired chunks that aren't available
	LastCameraPos   lin.Vec2 // Last camera position used for selection
	LastRenderDist  float32  // Last render distance used for selection
}

// ChunkInfo contains minimal information about a chunk for spatial queries
// This is kept lightweight for frustum testing and similar operations
type ChunkInfo struct {
	GridCoord GridCoord // Grid coordinates
	WorldPos  lin.Vec2  // World position (center of chunk)
}

// LODStrategy determines which LOD level to use based on distance
type LODStrategy interface {
	GetLODLevel(distance float32) int
}

// DefaultLODStrategy implements a simple distance-based LOD strategy
type DefaultLODStrategy struct {
	LODLevels     int     // Number of available LOD levels
	BaseDistance  float32 // Distance at which LOD 0 is used
	LODMultiplier float32 // Multiplier for each LOD level
}

// NewDefaultLODStrategy creates a new default LOD strategy
func NewDefaultLODStrategy(levels int, baseDistance, multiplier float32) *DefaultLODStrategy {
	return &DefaultLODStrategy{
		LODLevels:     levels,
		BaseDistance:  baseDistance,
		LODMultiplier: multiplier,
	}
}

// GetLODLevel returns the appropriate LOD level for the given distance
func (s *DefaultLODStrategy) GetLODLevel(distance float32) int {
	if distance <= s.BaseDistance {
		return 0
	}

	// Calculate LOD level based on distance
	level := int(float32(math.Log(float64(distance/s.BaseDistance))) /
		float32(math.Log(float64(s.LODMultiplier))))

	// Clamp to valid range
	if level < 0 {
		return 0
	}
	if level >= s.LODLevels {
		return s.LODLevels - 1
	}

	return level
}

// Frustum represents a 2D view frustum for culling
type Frustum struct {
	Planes [4]FrustumPlane // Left, Right, Near, Far
}

// FrustumPlane represents a plane in the 2D view frustum
type FrustumPlane struct {
	Normal lin.Vec2 // 2D normal vector
	D      float32  // Distance from origin
}
