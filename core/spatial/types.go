package spatial

import (
	"fmt"
	"math"
	"sync"

	lin "github.com/xlab/linmath"
)

// GridCoord represents discrete coordinates in the grid
type GridCoord struct {
	X, Z int
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

// ChunkStatus represents the current state of a chunk
type ChunkStatus int

const (
	ChunkUnloaded ChunkStatus = iota
	ChunkLoading
	ChunkLoaded
	ChunkError
)

// ChunkInfo contains spatial and state information about a terrain chunk
type ChunkInfo struct {
	ID        string                     // Unique identifier
	GridCoord GridCoord                  // Coordinates in the grid
	WorldPos  lin.Vec2                   // World position (center of chunk)
	LODLevel  int                        // Current LOD level (0 = highest detail)
	Distance  float32                    // Distance from viewpoint (2D)
	InFrustum bool                       // Is in view frustum
	Neighbors [DirectionCount]*ChunkInfo // Adjacent chunks (N,S,E,W)
	Status    ChunkStatus                // Current loading status
	Priority  float32                    // For loading priority (higher = more important)
	LastUsed  int64                      // Timestamp for LRU calculations
	UserData  any                        // For terrain manager to store references

	mu sync.RWMutex // For thread-safe updates to this chunk's properties
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

// GridSystem is the interface for terrain grid management
type GridSystem interface {
	// Registration
	RegisterChunk(worldPos lin.Vec3, size float32, lodLevel int) *ChunkInfo
	UnregisterChunk(id string) bool

	// Retrieval
	GetChunkByID(id string) *ChunkInfo
	GetChunkAt(worldPos lin.Vec3) *ChunkInfo
	GetChunksInRadius(center lin.Vec3, radius float32) []*ChunkInfo
	GetChunksInRing(center lin.Vec3, innerRadius, outerRadius float32) []*ChunkInfo

	// Neighbor finding
	GetNeighbors(chunk *ChunkInfo) []*ChunkInfo
	GetNeighborInDirection(chunk *ChunkInfo, direction Direction) *ChunkInfo

	// Update chunk states
	UpdateVisibility(cameraPos lin.Vec3, viewFrustum Frustum)

	// Get visible chunks
	GetVisibleChunks() []*ChunkInfo
	GetVisibleChunksSorted() []*ChunkInfo

	// Utilities
	Clear()
	Stats() GridStats
}
