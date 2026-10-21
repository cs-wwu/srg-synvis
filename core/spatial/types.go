package spatial

import (
	"fmt"

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

// ChunkInfo contains minimal information about a chunk for spatial queries
// This is kept lightweight for frustum testing and similar operations
type ChunkInfo struct {
	GridCoord GridCoord // Grid coordinates
	WorldPos  lin.Vec2  // World position (center of chunk)
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
