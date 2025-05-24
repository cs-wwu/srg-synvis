package spatial

import (
	"math"
	"sort"
	"sync"

	lin "github.com/xlab/linmath"
)

// ChunkSelector implements spatial chunk selection with proper distance-based priority
type ChunkSelector struct {
	chunkSize   float32
	lodStrategy LODStrategy

	// Current selection state
	desiredChunks   map[GridCoord]ChunkSpec
	availableChunks map[GridCoord]bool

	// Cache for performance
	lastCameraPos      lin.Vec2
	lastRenderDist     float32
	missingChunks      []ChunkSpec
	missingChunksDirty bool

	// Thread safety
	mu sync.RWMutex
}

// NewChunkSelector creates a new chunk selection system
func NewChunkSelector(chunkSize float32, lodStrategy LODStrategy) *ChunkSelector {
	if lodStrategy == nil {
		lodStrategy = NewDefaultLODStrategy(5, 500.0, 1.75)
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

	cs.desiredChunks = make(map[GridCoord]ChunkSpec)
	cs.lastCameraPos = cameraPos
	cs.lastRenderDist = renderDistance

	// Start from camera and spiral outward
	centerCoord := cs.WorldToGrid(cameraPos)
	radiusInChunks := int((renderDistance / cs.chunkSize) + 1)

	// Use spiral pattern to ensure closest chunks are processed first
	for radius := 0; radius <= radiusInChunks; radius++ {
		cs.addChunksAtRadius(centerCoord, radius, cameraPos, viewFrustum, renderDistance)
	}

	cs.missingChunksDirty = true
}

// addChunksAtRadius adds chunks at a specific distance from center
func (cs *ChunkSelector) addChunksAtRadius(center GridCoord, radius int, cameraPos lin.Vec2, viewFrustum Frustum, renderDistance float32) {
	if radius == 0 {
		// Just the center chunk
		cs.evaluateChunk(center, cameraPos, viewFrustum, renderDistance)
		return
	}

	// Walk around the perimeter of the square at this radius
	for i := range radius * 8 {
		var coord GridCoord
		side := i / (radius * 2)
		offset := i % (radius * 2)

		switch side {
		case 0: // Top edge
			coord = GridCoord{center.X - radius + offset, center.Z - radius}
		case 1: // Right edge
			coord = GridCoord{center.X + radius, center.Z - radius + offset}
		case 2: // Bottom edge
			coord = GridCoord{center.X + radius - offset, center.Z + radius}
		case 3: // Left edge
			coord = GridCoord{center.X - radius, center.Z + radius - offset}
		}

		cs.evaluateChunk(coord, cameraPos, viewFrustum, renderDistance)
	}
}

// evaluateChunk checks if a chunk should be included and calculates its properties
func (cs *ChunkSelector) evaluateChunk(coord GridCoord, cameraPos lin.Vec2, viewFrustum Frustum, renderDistance float32) {
	worldPos := cs.GridToWorld(coord)

	// Distance check first (cheapest)
	dx := worldPos[0] - cameraPos[0]
	dz := worldPos[1] - cameraPos[1]
	distanceSq := dx*dx + dz*dz
	if distanceSq > renderDistance*renderDistance {
		return
	}

	// Create chunk info for frustum test
	chunkInfo := &ChunkInfo{
		GridCoord: coord,
		WorldPos:  worldPos,
	}

	// Frustum culling
	inFrustum := IsChunkInFrustum(chunkInfo, viewFrustum)

	distance := float32(math.Sqrt(float64(distanceSq)))
	lodLevel := cs.lodStrategy.GetLODLevel(distance)
	priority := cs.calculatePriorityByDistance(distance, inFrustum)

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

// calculatePriorityByDistance creates priority based purely on distance with frustum bonus
func (cs *ChunkSelector) calculatePriorityByDistance(distance float32, inFrustum bool) float32 {
	// Base priority: closer = higher (inverse distance)
	basePriority := 10000.0 / (distance + 1.0)

	// Huge bonus for being in frustum
	if inFrustum {
		return basePriority * 10.0 // 10x multiplier for visible chunks
	}

	return basePriority
}

// GetMissingChunks returns desired chunks that are not available, sorted by priority (highest first)
func (cs *ChunkSelector) GetMissingChunks() []ChunkSpec {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.missingChunksDirty {
		return cs.missingChunks
	}

	// Rebuild missing chunks list
	cs.missingChunks = cs.missingChunks[:0]

	for coord, spec := range cs.desiredChunks {
		if !cs.availableChunks[coord] {
			cs.missingChunks = append(cs.missingChunks, spec)
		}
	}

	// Sort by priority (highest first), with distance as tiebreaker
	sort.Slice(cs.missingChunks, func(i, j int) bool {
		a, b := cs.missingChunks[i], cs.missingChunks[j]

		// Primary sort: priority (higher first)
		if math.Abs(float64(a.Priority-b.Priority)) > 0.001 {
			return a.Priority > b.Priority
		}

		// Tiebreaker: distance (closer first)
		return a.Distance < b.Distance
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
		X: int(math.Floor(float64(pos[0]) / float64(cs.chunkSize))),
		Z: int(math.Floor(float64(pos[1]) / float64(cs.chunkSize))),
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
	distance := float32(math.Sqrt(float64(dx*dx + dz*dz)))

	return cs.lodStrategy.GetLODLevel(distance)
}

// CalculatePriority calculates the loading priority for a chunk
func (cs *ChunkSelector) CalculatePriority(coord GridCoord, cameraPos lin.Vec2, inFrustum bool) float32 {
	worldPos := cs.GridToWorld(coord)
	dx := worldPos[0] - cameraPos[0]
	dz := worldPos[1] - cameraPos[1]
	distance := float32(math.Sqrt(float64(dx*dx + dz*dz)))

	return cs.calculatePriorityByDistance(distance, inFrustum)
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
