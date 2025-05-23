package spatial

import (
	"sort"
	"sync"

	lin "github.com/xlab/linmath"
)

// ChunkSelector implements spatial chunk selection and basic state tracking
type ChunkSelector struct {
	chunkSize   float32     // Size of each chunk in world units
	lodStrategy LODStrategy // Strategy for LOD determination

	// Current selection state
	desiredChunks   map[GridCoord]ChunkSpec // What should exist
	availableChunks map[GridCoord]bool      // What StreamManager says is available

	// Cache for performance
	lastCameraPos      lin.Vec2
	lastRenderDist     float32
	missingChunks      []ChunkSpec // Cached list of missing chunks, sorted by priority
	missingChunksDirty bool

	// Thread safety
	mu sync.RWMutex
}

// NewChunkSelector creates a new chunk selection system
func NewChunkSelector(chunkSize float32, lodStrategy LODStrategy) *ChunkSelector {
	if lodStrategy == nil {
		// Default LOD strategy: 3 levels, base distance of 500, multiplier of 2
		lodStrategy = NewDefaultLODStrategy(3, 500.0, 2.0)
	}

	return &ChunkSelector{
		chunkSize:          chunkSize,
		lodStrategy:        lodStrategy,
		desiredChunks:      make(map[GridCoord]ChunkSpec),
		availableChunks:    make(map[GridCoord]bool),
		missingChunks:      make([]ChunkSpec, 0, 100),
		missingChunksDirty: true,
	}
}

// UpdateSelection determines what chunks should exist given camera position and view parameters
func (cs *ChunkSelector) UpdateSelection(cameraPos lin.Vec2, viewFrustum Frustum, renderDistance float32) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Clear previous selection
	cs.desiredChunks = make(map[GridCoord]ChunkSpec)
	cs.lastCameraPos = cameraPos
	cs.lastRenderDist = renderDistance

	// Calculate the grid bounds to check
	radiusInChunks := int((renderDistance / cs.chunkSize) + 1)
	centerCoord := cs.WorldToGrid(cameraPos)

	// Check all chunks within render distance
	for x := centerCoord.X - radiusInChunks; x <= centerCoord.X+radiusInChunks; x++ {
		for z := centerCoord.Z - radiusInChunks; z <= centerCoord.Z+radiusInChunks; z++ {
			coord := GridCoord{X: x, Z: z}
			worldPos := cs.GridToWorld(coord)

			// Distance check
			dx := worldPos[0] - cameraPos[0]
			dz := worldPos[1] - cameraPos[1]
			distance := dx*dx + dz*dz
			if distance > renderDistance*renderDistance {
				continue
			}

			// Create temporary chunk info for frustum test
			chunkInfo := &ChunkInfo{
				GridCoord: coord,
				WorldPos:  worldPos,
			}

			// Frustum culling
			inFrustum := IsChunkInFrustum(chunkInfo, viewFrustum)

			// Calculate final properties
			distance = float32(sqrt(float64(distance)))
			lodLevel := cs.lodStrategy.GetLODLevel(distance)
			priority := cs.CalculatePriority(coord, cameraPos, inFrustum)

			// Add to desired chunks
			cs.desiredChunks[coord] = ChunkSpec{
				GridCoord: coord,
				WorldPos:  worldPos,
				Distance:  distance,
				LODLevel:  lodLevel,
				Priority:  priority,
				InFrustum: inFrustum,
			}
		}
	}

	// Mark missing chunks list as dirty for recalculation
	cs.missingChunksDirty = true
}

// GetDesiredChunks returns all chunks that should exist
func (cs *ChunkSelector) GetDesiredChunks() []ChunkSpec {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	chunks := make([]ChunkSpec, 0, len(cs.desiredChunks))
	for _, spec := range cs.desiredChunks {
		chunks = append(chunks, spec)
	}
	return chunks
}

// GetMissingChunks returns desired chunks that are not available, sorted by priority
func (cs *ChunkSelector) GetMissingChunks() []ChunkSpec {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.missingChunksDirty {
		return cs.missingChunks
	}

	// Rebuild missing chunks list
	cs.missingChunks = cs.missingChunks[:0] // Reset slice but keep capacity

	for coord, spec := range cs.desiredChunks {
		if !cs.availableChunks[coord] {
			cs.missingChunks = append(cs.missingChunks, spec)
		}
	}

	// Sort by priority (highest first)
	sort.Slice(cs.missingChunks, func(i, j int) bool {
		return cs.missingChunks[i].Priority > cs.missingChunks[j].Priority
	})

	cs.missingChunksDirty = false
	return cs.missingChunks
}

// GetHighestPriorityMissing returns the missing chunk with highest priority
func (cs *ChunkSelector) GetHighestPriorityMissing() *ChunkSpec {
	missing := cs.GetMissingChunks()
	if len(missing) == 0 {
		return nil
	}
	return &missing[0]
}

// MarkChunkAvailable marks a chunk as available (loaded by StreamManager)
func (cs *ChunkSelector) MarkChunkAvailable(coord GridCoord) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.availableChunks[coord] = true
	cs.missingChunksDirty = true
}

// MarkChunkUnavailable marks a chunk as unavailable (unloaded by StreamManager)
func (cs *ChunkSelector) MarkChunkUnavailable(coord GridCoord) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.availableChunks, coord)
	cs.missingChunksDirty = true
}

// IsChunkAvailable returns true if StreamManager has reported this chunk as available
func (cs *ChunkSelector) IsChunkAvailable(coord GridCoord) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.availableChunks[coord]
}

// IsChunkDesired returns true if this chunk should exist given current camera position
func (cs *ChunkSelector) IsChunkDesired(coord GridCoord) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	_, exists := cs.desiredChunks[coord]
	return exists
}

// GetNeighborCoords returns the coordinates of the 4 cardinal neighbors
func (cs *ChunkSelector) GetNeighborCoords(coord GridCoord) [4]GridCoord {
	return [4]GridCoord{
		{coord.X, coord.Z - 1}, // North
		{coord.X + 1, coord.Z}, // East
		{coord.X, coord.Z + 1}, // South
		{coord.X - 1, coord.Z}, // West
	}
}

// WorldToGrid converts 2D world position to grid coordinates
func (cs *ChunkSelector) WorldToGrid(pos lin.Vec2) GridCoord {
	return GridCoord{
		X: int(floor(float64(pos[0]) / float64(cs.chunkSize))),
		Z: int(floor(float64(pos[1]) / float64(cs.chunkSize))),
	}
}

// GridToWorld converts grid coordinates to 2D world position (center of chunk)
func (cs *ChunkSelector) GridToWorld(coord GridCoord) lin.Vec2 {
	return lin.Vec2{
		float32(coord.X)*cs.chunkSize + cs.chunkSize/2,
		float32(coord.Z)*cs.chunkSize + cs.chunkSize/2,
	}
}

// CalculateLOD determines the appropriate LOD level for a chunk
func (cs *ChunkSelector) CalculateLOD(coord GridCoord, cameraPos lin.Vec2) int {
	worldPos := cs.GridToWorld(coord)
	dx := worldPos[0] - cameraPos[0]
	dz := worldPos[1] - cameraPos[1]
	distance := float32(sqrt(float64(dx*dx + dz*dz)))

	return cs.lodStrategy.GetLODLevel(distance)
}

// CalculatePriority calculates the loading priority for a chunk
func (cs *ChunkSelector) CalculatePriority(coord GridCoord, cameraPos lin.Vec2, inFrustum bool) float32 {
	worldPos := cs.GridToWorld(coord)
	dx := worldPos[0] - cameraPos[0]
	dz := worldPos[1] - cameraPos[1]
	distance := float32(sqrt(float64(dx*dx + dz*dz)))

	var priority float32
	if inFrustum {
		// Higher priority for chunks in view
		priority = 1000.0 / (distance + 1.0)
	} else {
		// Lower priority for chunks outside view
		priority = 100.0 / (distance + 1.0)
	}

	return priority
}

// Stats returns statistics about the chunk selection system
func (cs *ChunkSelector) Stats() ChunkSelectorStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	desired := len(cs.desiredChunks)
	available := len(cs.availableChunks)

	// Count how many desired chunks are actually available
	desiredAndAvailable := 0
	for coord := range cs.desiredChunks {
		if cs.availableChunks[coord] {
			desiredAndAvailable++
		}
	}

	missing := desired - desiredAndAvailable

	return ChunkSelectorStats{
		DesiredChunks:   desired,
		AvailableChunks: available,
		MissingChunks:   missing,
		LastCameraPos:   cs.lastCameraPos,
		LastRenderDist:  cs.lastRenderDist,
	}
}

// Clear resets all internal state
func (cs *ChunkSelector) Clear() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.desiredChunks = make(map[GridCoord]ChunkSpec)
	cs.availableChunks = make(map[GridCoord]bool)
	cs.missingChunks = cs.missingChunks[:0]
	cs.missingChunksDirty = true
}

// Helper functions for math operations
func floor(x float64) float64 {
	if x >= 0 {
		return float64(int(x))
	}
	i := int(x)
	if float64(i) == x {
		return x
	}
	return float64(i - 1)
}

func sqrt(x float64) float64 {
	if x == 0 || x == 1 {
		return x
	}

	// Newton's method for square root
	guess := x
	for range 10 { // 10 iterations should be enough for float32 precision
		guess = (guess + x/guess) / 2
	}
	return guess
}
