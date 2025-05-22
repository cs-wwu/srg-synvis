package spatial

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"slices"

	lin "github.com/xlab/linmath"
)

// TerrainGrid implements the GridSystem interface
type TerrainGrid struct {
	chunkSize     float32                  // Size of each chunk in world units
	chunks        map[string]*ChunkInfo    // Lookup by chunk ID
	chunksByCoord map[GridCoord]*ChunkInfo // Lookup by grid coordinates
	visibleChunks []*ChunkInfo             // Visible chunks
	lodStrategy   LODStrategy              // Strategy for LOD determination

	// Internal state
	lastCameraPos lin.Vec2
	lastUpdate    int64 // Timestamp of last update
	nextChunkID   int   // For generating unique chunk IDs

	// Thread safety
	mu sync.RWMutex
}

// NewTerrainGrid creates a new terrain grid system
func NewTerrainGrid(chunkSize float32, lodStrategy LODStrategy) *TerrainGrid {
	if lodStrategy == nil {
		// Default LOD strategy: 3 levels, base distance of 500, multiplier of 2
		lodStrategy = NewDefaultLODStrategy(3, 500.0, 2.0)
	}

	return &TerrainGrid{
		chunkSize:     chunkSize,
		chunks:        make(map[string]*ChunkInfo),
		chunksByCoord: make(map[GridCoord]*ChunkInfo),
		visibleChunks: make([]*ChunkInfo, 0, 100), // Preallocate for common case
		lodStrategy:   lodStrategy,
		nextChunkID:   1,
	}
}

// WorldToGrid converts 2D world position to grid coordinates
func (g *TerrainGrid) WorldToGrid(pos lin.Vec2) GridCoord {
	return GridCoord{
		X: int(math.Floor(float64(pos[0]) / float64(g.chunkSize))),
		Z: int(math.Floor(float64(pos[1]) / float64(g.chunkSize))),
	}
}

// GridToWorld converts grid coordinates to 2D world position (center of chunk)
func (g *TerrainGrid) GridToWorld(coord GridCoord) lin.Vec2 {
	return lin.Vec2{
		float32(coord.X)*g.chunkSize + g.chunkSize/2,
		float32(coord.Z)*g.chunkSize + g.chunkSize/2,
	}
}

// RegisterChunk adds a new chunk at the given 2D world position
func (g *TerrainGrid) RegisterChunk(worldPos lin.Vec2, lodLevel int) *ChunkInfo {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Convert to grid coordinates
	coord := g.WorldToGrid(worldPos)

	// Check if a chunk already exists at this coordinate
	if existing, exists := g.chunksByCoord[coord]; exists {
		return existing
	}

	// Generate a unique ID for this chunk
	id := fmt.Sprintf("chunk_%d", g.nextChunkID)
	g.nextChunkID++

	// Calculate the center position for this chunk
	centerPos := g.GridToWorld(coord)

	// Create the new chunk
	chunk := &ChunkInfo{
		ID:        id,
		GridCoord: coord,
		WorldPos:  centerPos, // Always use the center position
		LODLevel:  lodLevel,
		Status:    ChunkUnloaded,
		LastUsed:  time.Now().UnixNano(),
		Neighbors: [DirectionCount]*ChunkInfo{},
	}

	// Register it
	g.chunks[id] = chunk
	g.chunksByCoord[coord] = chunk

	// Update neighbor relationships
	g.updateChunkNeighbors(chunk)

	return chunk
}

// updateChunkNeighbors updates the neighbor relationships for a chunk
func (g *TerrainGrid) updateChunkNeighbors(chunk *ChunkInfo) {
	// Helper to get neighbors in each of the 4 cardinal directions
	directions := []struct {
		dir    Direction
		dx, dz int
	}{
		{North, 0, -1},
		{East, 1, 0},
		{South, 0, 1},
		{West, -1, 0},
	}

	for _, d := range directions {
		neighborCoord := GridCoord{
			X: chunk.GridCoord.X + d.dx,
			Z: chunk.GridCoord.Z + d.dz,
		}

		// Find the neighbor
		if neighbor, exists := g.chunksByCoord[neighborCoord]; exists {
			// Update this chunk's reference to its neighbor
			chunk.Neighbors[d.dir] = neighbor

			// Update the neighbor's reference to this chunk (opposite direction)
			oppositeDir := getOppositeDirection(d.dir)
			neighbor.Neighbors[oppositeDir] = chunk
		}
	}
}

// getOppositeDirection returns the opposite of the given direction
func getOppositeDirection(dir Direction) Direction {
	switch dir {
	case North:
		return South
	case South:
		return North
	case East:
		return West
	case West:
		return East
	default:
		return North // Fallback
	}
}

// UnregisterChunk removes a chunk from the grid
func (g *TerrainGrid) UnregisterChunk(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	chunk, exists := g.chunks[id]
	if !exists {
		return false
	}

	// Remove from all collections
	delete(g.chunks, id)
	delete(g.chunksByCoord, chunk.GridCoord)

	// Update neighbor relationships
	for i, neighbor := range chunk.Neighbors {
		if neighbor != nil {
			// Find the opposite direction
			oppositeDir := getOppositeDirection(Direction(i))

			// Clear the reference from neighbor to this chunk
			neighbor.Neighbors[oppositeDir] = nil
		}
	}

	// Remove from visible chunks if present
	for i := range g.visibleChunks {
		if g.visibleChunks[i].ID == id {
			g.visibleChunks = slices.Delete(g.visibleChunks, i, i+1)
			break
		}
	}

	return true
}

// GetChunkByID retrieves a chunk by its ID
func (g *TerrainGrid) GetChunkByID(id string) *ChunkInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.chunks[id]
}

// GetChunkAt retrieves the chunk at the given world position
func (g *TerrainGrid) GetChunkAt(worldPos lin.Vec2) *ChunkInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	coord := g.WorldToGrid(worldPos)
	return g.chunksByCoord[coord]
}

// GetChunksInRadius returns all chunks within the specified radius
func (g *TerrainGrid) GetChunksInRadius(center lin.Vec2, radius float32) []*ChunkInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	results := make([]*ChunkInfo, 0)
	radiusSqr := radius * radius

	for _, chunk := range g.chunks {
		// Calculate 2D distance from center to center
		dx := chunk.WorldPos[0] - center[0]
		dz := chunk.WorldPos[1] - center[1]
		distSqr := dx*dx + dz*dz

		if distSqr <= radiusSqr {
			results = append(results, chunk)
		}
	}

	return results
}

// GetChunksInRing returns all chunks within the specified ring (between inner and outer radius)
func (g *TerrainGrid) GetChunksInRing(center lin.Vec2, innerRadius, outerRadius float32) []*ChunkInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	results := make([]*ChunkInfo, 0)
	innerRadiusSqr := innerRadius * innerRadius
	outerRadiusSqr := outerRadius * outerRadius

	for _, chunk := range g.chunks {
		// Calculate 2D center-to-center distance
		dx := chunk.WorldPos[0] - center[0]
		dz := chunk.WorldPos[1] - center[1]
		distSqr := dx*dx + dz*dz

		if distSqr >= innerRadiusSqr && distSqr <= outerRadiusSqr {
			results = append(results, chunk)
		}
	}

	return results
}

// GetNeighbors returns all neighbors of the given chunk (up to 4)
func (g *TerrainGrid) GetNeighbors(chunk *ChunkInfo) []*ChunkInfo {
	if chunk == nil {
		return nil
	}

	chunk.mu.RLock()
	defer chunk.mu.RUnlock()

	neighbors := make([]*ChunkInfo, 0, DirectionCount)
	for _, neighbor := range chunk.Neighbors {
		if neighbor != nil {
			neighbors = append(neighbors, neighbor)
		}
	}

	return neighbors
}

// GetNeighborInDirection returns the neighbor in the specified direction
func (g *TerrainGrid) GetNeighborInDirection(chunk *ChunkInfo, direction Direction) *ChunkInfo {
	if chunk == nil || direction < 0 || direction >= DirectionCount {
		return nil
	}

	chunk.mu.RLock()
	defer chunk.mu.RUnlock()

	return chunk.Neighbors[direction]
}

// calculateDistance computes the distance from a point to a chunk
func calculateDistance(point lin.Vec2, chunk *ChunkInfo) float32 {
	// Calculate 2D center-to-center distance
	dx := chunk.WorldPos[0] - point[0]
	dz := chunk.WorldPos[1] - point[1]
	return float32(math.Sqrt(float64(dx*dx + dz*dz)))
}

// UpdateVisibility updates chunk visibility based on camera position and view frustum
func (g *TerrainGrid) UpdateVisibility(cameraPos lin.Vec2, viewFrustum Frustum) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.lastCameraPos = cameraPos
	g.lastUpdate = time.Now().UnixNano()

	// Reset visible chunks array
	visibleChunks := g.visibleChunks[:0]

	// Update each chunk's distance, visibility, and LOD level
	for _, chunk := range g.chunks {
		chunk.mu.Lock()

		// Calculate 2D distance
		chunk.Distance = calculateDistance(cameraPos, chunk)
		chunk.InFrustum = IsChunkInFrustum(chunk, viewFrustum)

		// Determine LOD level based on 2D distance
		chunk.LODLevel = g.lodStrategy.GetLODLevel(chunk.Distance)

		// Calculate priority
		// Priority is higher for closer chunks and those visible in frustum
		if chunk.InFrustum {
			// Base priority on inverse distance (closer = higher priority)
			chunk.Priority = 1000.0 / (chunk.Distance + 1.0)
			// Update last used time for LRU calculations
			chunk.LastUsed = g.lastUpdate
		} else {
			// Lower priority for chunks outside frustum
			chunk.Priority = 100.0 / (chunk.Distance + 1.0)
		}

		chunk.mu.Unlock()

		// If visible, add to visible chunks array
		if chunk.InFrustum {
			visibleChunks = append(visibleChunks, chunk)
		}
	}

	// Store the updated visible chunks
	g.visibleChunks = visibleChunks
}

// GetVisibleChunks returns all currently visible chunks
func (g *TerrainGrid) GetVisibleChunks() []*ChunkInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.visibleChunks
}

// GetVisibleChunksSorted returns visible chunks sorted by priority
func (g *TerrainGrid) GetVisibleChunksSorted() []*ChunkInfo {
	g.mu.RLock()
	chunks := make([]*ChunkInfo, len(g.visibleChunks))
	copy(chunks, g.visibleChunks)
	g.mu.RUnlock()

	// Sort by priority (highest first)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Priority > chunks[j].Priority
	})

	return chunks
}

// Clear removes all chunks from the grid
func (g *TerrainGrid) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.chunks = make(map[string]*ChunkInfo)
	g.chunksByCoord = make(map[GridCoord]*ChunkInfo)
	g.visibleChunks = g.visibleChunks[:0]
}
