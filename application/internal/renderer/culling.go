package renderer

import (
	lin "github.com/xlab/linmath"
)

const (
	MAX_HEIGHT = 32768 // Maximum terrain height in the dataset
)

type Plane struct {
	Normal   lin.Vec3
	Distance float32
}

type Frustum struct {
	Planes [6]Plane // Left, Right, Top, Bottom, Near, Far
}

// Update updates the frustum planes based on the current view and projection matrices
func (f *Frustum) Update(view, proj *lin.Mat4x4) {
	// Combine view and projection matrices
	var clipMatrix lin.Mat4x4
	clipMatrix.Mult(proj, view)

	// Extract frustum planes from the combined matrix
	f.Planes[0] = extractPlane(&clipMatrix, 0, 3, -1) // Left
	f.Planes[1] = extractPlane(&clipMatrix, 0, 3, 1)  // Right
	f.Planes[2] = extractPlane(&clipMatrix, 1, 3, -1) // Bottom
	f.Planes[3] = extractPlane(&clipMatrix, 1, 3, 1)  // Top
	f.Planes[4] = extractPlane(&clipMatrix, 2, 3, -1) // Near
	f.Planes[5] = extractPlane(&clipMatrix, 2, 3, 1)  // Far

	// Normalize all plane normals
	for i := range f.Planes {
		f.Planes[i].normalize()
	}
}

// extractPlane extracts a plane from the combined view-projection matrix
func extractPlane(m *lin.Mat4x4, row, col int, sign float32) Plane {
	plane := Plane{
		Normal: lin.Vec3{
			m[0][col] + sign*m[0][row],
			m[1][col] + sign*m[1][row],
			m[2][col] + sign*m[2][row],
		},
		Distance: m[3][col] + sign*m[3][row],
	}
	return plane
}

// normalize normalizes the plane's normal vector and adjusts the distance accordingly
func (p *Plane) normalize() {
	length := p.Normal.Len()
	if length != 0 {
		p.Normal[0] /= length
		p.Normal[1] /= length
		p.Normal[2] /= length
		p.Distance /= length
	}
}

// IsChunkVisible determines if a chunk is visible within the frustum
func (f *Frustum) IsChunkVisible(chunk *Chunk) bool {
	// Define the chunk's bounding box corners
	corners := [8]lin.Vec3{
		// Bottom corners
		{float32(chunk.X), 0, float32(chunk.Z)},
		{float32(chunk.X + CHUNK_SIZE), 0, float32(chunk.Z)},
		{float32(chunk.X), 0, float32(chunk.Z + CHUNK_SIZE)},
		{float32(chunk.X + CHUNK_SIZE), 0, float32(chunk.Z + CHUNK_SIZE)},
		// Top corners
		{float32(chunk.X), MAX_HEIGHT * HEIGHT_SCALE, float32(chunk.Z)},
		{float32(chunk.X + CHUNK_SIZE), MAX_HEIGHT * HEIGHT_SCALE, float32(chunk.Z)},
		{float32(chunk.X), MAX_HEIGHT * HEIGHT_SCALE, float32(chunk.Z + CHUNK_SIZE)},
		{float32(chunk.X + CHUNK_SIZE), MAX_HEIGHT * HEIGHT_SCALE, float32(chunk.Z + CHUNK_SIZE)},
	}

	// Check if the chunk's bounding box is outside any of the frustum planes
	for _, plane := range f.Planes {
		inside := false
		for _, corner := range corners {
			// If any corner is on the positive side of the plane, the chunk intersects
			if plane.Normal[0]*corner[0]+plane.Normal[1]*corner[1]+plane.Normal[2]*corner[2]+plane.Distance > 0 {
				inside = true
				break
			}
		}
		// If all corners are outside this plane, the chunk is not visible
		if !inside {
			return false
		}
	}

	return true
}

// UpdateVisibleChunks updates the TerrainManager's visible chunks based on frustum culling
func (app *Application) updateVisibleChunks() {
	tm := app.TerrainManager

	// Get write lock since we're modifying visible chunks
	tm.chunkMutex.Lock()
	defer tm.chunkMutex.Unlock()

	// Store old visible chunks for comparison
	oldVisibleChunks := make(map[string]bool, len(tm.visibleChunks))
	for k, v := range tm.visibleChunks {
		oldVisibleChunks[k] = v
	}

	// Clear the previous visible chunk map
	for k := range tm.visibleChunks {
		delete(tm.visibleChunks, k)
	}

	visibleCount := 0
	// Check each chunk against the frustum
	for _, chunk := range tm.orderedChunks {
		if app.frustum.IsChunkVisible(chunk) {
			chunkKey := ChunkKey(chunk.X, chunk.Z)
			tm.visibleChunks[chunkKey] = true
			visibleCount++
		}
	}

	// Check if visibility actually changed
	if visibleCount > 0 && !mapsEqual(oldVisibleChunks, tm.visibleChunks) {
		tm.FillPendingBuffers()
		tm.updated.Store(true)
	}
}

// Helper function to compare visibility maps
func mapsEqual(m1, m2 map[string]bool) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		if v2, ok := m2[k]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}
