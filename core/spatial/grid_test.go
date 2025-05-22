package spatial

import (
	"math"
	"strings"
	"testing"

	lin "github.com/xlab/linmath"
)

func TestWorldToGridConversion(t *testing.T) {
	grid := NewTerrainGrid(100.0, nil)

	tests := []struct {
		name     string
		worldPos lin.Vec2
		expected GridCoord
	}{
		{"Origin", lin.Vec2{0, 0}, GridCoord{0, 0}},
		{"Positive", lin.Vec2{150, 350}, GridCoord{1, 3}},
		{"Negative", lin.Vec2{-150, -350}, GridCoord{-2, -4}},
		{"Edge", lin.Vec2{100, 100}, GridCoord{1, 1}},
		{"Edge minus epsilon", lin.Vec2{99.9, 99.9}, GridCoord{0, 0}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := grid.WorldToGrid(test.worldPos)
			if result.X != test.expected.X || result.Z != test.expected.Z {
				t.Errorf("WorldToGrid(%v) = %v, expected %v", test.worldPos, result, test.expected)
			}
		})
	}
}

func TestGridToWorldConversion(t *testing.T) {
	grid := NewTerrainGrid(100.0, nil)

	tests := []struct {
		name      string
		gridCoord GridCoord
		expected  lin.Vec2
	}{
		{"Origin", GridCoord{0, 0}, lin.Vec2{50, 50}},
		{"Positive", GridCoord{1, 3}, lin.Vec2{150, 350}},
		{"Negative", GridCoord{-2, -4}, lin.Vec2{-150, -350}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := grid.GridToWorld(test.gridCoord)
			epsilon := float32(0.001)
			if !vec2Equal(result, test.expected, epsilon) {
				t.Errorf("GridToWorld(%v) = %v, expected %v", test.gridCoord, result, test.expected)
			}
		})
	}
}

func TestChunkRegistration(t *testing.T) {
	grid := NewTerrainGrid(100.0, nil)

	// Register a chunk
	pos := lin.Vec2{150, 350}
	chunk := grid.RegisterChunk(pos, 0)

	if chunk == nil {
		t.Fatalf("RegisterChunk returned nil")
	}

	// Check chunk properties
	if chunk.GridCoord.X != 1 || chunk.GridCoord.Z != 3 {
		t.Errorf("Chunk has incorrect grid coord: %v", chunk.GridCoord)
	}

	if !vec2Equal(chunk.WorldPos, lin.Vec2{150, 350}, 0.001) {
		t.Errorf("Chunk has incorrect world pos: %v", chunk.WorldPos)
	}

	// Check retrieval
	retrieved := grid.GetChunkAt(pos)
	if retrieved != chunk {
		t.Errorf("GetChunkAt did not return the registered chunk")
	}

	byID := grid.GetChunkByID(chunk.ID)
	if byID != chunk {
		t.Errorf("GetChunkByID did not return the registered chunk")
	}

	// Unregister the chunk
	if !grid.UnregisterChunk(chunk.ID) {
		t.Errorf("UnregisterChunk returned false")
	}

	// Ensure it's gone
	if grid.GetChunkByID(chunk.ID) != nil {
		t.Errorf("Chunk still exists after unregistering")
	}
}

func TestNeighborRelationships(t *testing.T) {
	grid := NewTerrainGrid(100.0, nil)

	// Create a 3x3 grid of chunks (2D grid)
	chunks := make(map[GridCoord]*ChunkInfo)
	for x := -1; x <= 1; x++ {
		for z := -1; z <= 1; z++ {
			pos := lin.Vec2{float32(x) * 100, float32(z) * 100}
			chunk := grid.RegisterChunk(pos, 0)
			coord := GridCoord{x, z}
			chunks[coord] = chunk
		}
	}

	// Check center chunk's neighbors
	center := chunks[GridCoord{0, 0}]

	tests := []struct {
		dir      Direction
		expected GridCoord
	}{
		{North, GridCoord{0, -1}},
		{East, GridCoord{1, 0}},
		{South, GridCoord{0, 1}},
		{West, GridCoord{-1, 0}},
	}

	for _, test := range tests {
		t.Run(DirectionNames[test.dir], func(t *testing.T) {
			neighbor := grid.GetNeighborInDirection(center, test.dir)
			if neighbor == nil {
				t.Fatalf("No neighbor found in direction %s", DirectionNames[test.dir])
			}

			expected := chunks[test.expected]
			if neighbor != expected {
				t.Errorf("Wrong neighbor in direction %s. Got %v, expected %v",
					DirectionNames[test.dir], neighbor.GridCoord, expected.GridCoord)
			}
		})
	}

	// Check that the relationship is bidirectional
	east := chunks[GridCoord{1, 0}]
	westNeighbor := grid.GetNeighborInDirection(east, West)
	if westNeighbor != center {
		t.Errorf("Bidirectional neighbor relationship failed")
	}
}

func TestRadiusQueries(t *testing.T) {
	grid := NewTerrainGrid(100.0, nil)

	// Create a 5x5 grid of chunks centered at (0,0)
	for x := -2; x <= 2; x++ {
		for z := -2; z <= 2; z++ {
			// Use coordinates that will be at the center of each chunk
			pos := lin.Vec2{float32(x)*100 + 50, float32(z)*100 + 50}
			grid.RegisterChunk(pos, 0)
		}
	}

	center := lin.Vec2{50, 50} // Center of center chunk

	// Test with different radii (updated for 2D distances)
	tests := []struct {
		radius        float32
		expectedCount int
	}{
		{50.0, 1},   // Just the center chunk
		{150.0, 9},  // Center + 8 adjacent (same as 3D case)
		{220.0, 21}, // Center + 8 adjacent + 12 in next ring (adjusted for 2D)
		{320.0, 25}, // All 25 chunks (adjusted for 2D)
	}

	for i, test := range tests {
		chunks := grid.GetChunksInRadius(center, test.radius)
		if len(chunks) != test.expectedCount {
			t.Errorf("Test %d: Expected %d chunks within radius %.1f, got %d",
				i, test.expectedCount, test.radius, len(chunks))
		}
	}

	// Test ring query (adjusted for 2D)
	ring := grid.GetChunksInRing(center, 120.0, 180.0)
	if len(ring) != 8 {
		t.Errorf("Expected 8 chunks in ring, got %d", len(ring))
	}
}

func TestLODDetermination(t *testing.T) {
	// Create a custom LOD strategy for testing
	strategy := NewDefaultLODStrategy(3, 100.0, 2.0)
	grid := NewTerrainGrid(100.0, strategy)

	// Place chunks at specific 2D distances from (0,0)
	positions := []lin.Vec2{
		{50, 50},  // Center chunk
		{50, 140}, // 90 units away from center (2D distance)
		{50, 250}, // 200 units away from center
		{50, 350}, // 300 units away from center
		{50, 500}, // 450 units away from center
	}

	chunks := make([]*ChunkInfo, len(positions))
	for i, pos := range positions {
		chunks[i] = grid.RegisterChunk(pos, 0)
	}

	// Update visibility with camera at (50,50)
	cameraPos := lin.Vec2{50, 50}
	// Create a simple frustum for testing (looking north with wide FOV)
	dummyFrustum := NewFrustumFromCamera(cameraPos, 0, math.Pi/2, 1.0, 1000.0)
	grid.UpdateVisibility(cameraPos, dummyFrustum)

	// Check LOD assignments
	expectedLODs := []int{0, 0, 1, 1, 2}
	for i, chunk := range chunks {
		if chunk.LODLevel != expectedLODs[i] {
			t.Errorf("Chunk at distance %.1f expected LOD %d, got %d",
				chunk.Distance, expectedLODs[i], chunk.LODLevel)
		}
	}
}

func TestGridStatsAndVisualization(t *testing.T) {
	grid := NewTerrainGrid(100.0, nil)

	// Create a test grid with various chunk states
	for x := -2; x <= 2; x++ {
		for z := -2; z <= 2; z++ {
			pos := lin.Vec2{float32(x)*100 + 50, float32(z)*100 + 50}
			chunk := grid.RegisterChunk(pos, 0)

			// Assign different LOD levels based on position
			lodLevel := (abs(x) + abs(z)) / 2
			chunk.LODLevel = lodLevel

			// Set some chunks as visible
			if abs(x) <= 1 && abs(z) <= 1 {
				chunk.InFrustum = true
			}

			// Set different statuses
			if x == 0 && z == 0 {
				chunk.Status = ChunkLoaded
			} else if abs(x) <= 1 && abs(z) <= 1 {
				chunk.Status = ChunkLoading
			}
		}
	}

	// Test stats
	stats := grid.Stats()
	if stats.TotalChunks != 25 {
		t.Errorf("Expected 25 total chunks, got %d", stats.TotalChunks)
	}

	// Test detailed stats output
	detailedStats := grid.GetDetailedStats()
	if !strings.Contains(detailedStats, "Total Chunks: 25") {
		t.Errorf("Detailed stats should contain 'Total Chunks: 25'")
	}

	// Test grid visualization
	gridViz := grid.VisualizeGrid()
	t.Logf("Grid Visualization:\n%s", gridViz)
	if !strings.Contains(gridViz, "Grid Visualization") {
		t.Errorf("Grid visualization missing header")
	}

	// Get center chunk and test chunk visualization
	centerChunk := grid.GetChunkAt(lin.Vec2{50, 50})
	if centerChunk == nil {
		t.Fatalf("Center chunk not found")
	}

	chunkViz := grid.VisualizeChunk(centerChunk.ID)
	if !strings.Contains(chunkViz, "Chunk Visualization") {
		t.Errorf("Chunk visualization missing header")
	}

	// Test visibility visualization
	visViz := grid.VisualizeVisibility()
	if !strings.Contains(visViz, "Visibility and LOD Distribution") {
		t.Errorf("Visibility visualization missing header")
	}
}

func TestFrustumCulling(t *testing.T) {
	grid := NewTerrainGrid(100.0, nil)

	// Create chunks in a line along the Z axis
	positions := []lin.Vec2{
		{0, 100},    // Directly ahead
		{100, 100},  // To the right
		{-100, 100}, // To the left
		{0, -100},   // Behind
	}

	chunks := make([]*ChunkInfo, len(positions))
	for i, pos := range positions {
		chunks[i] = grid.RegisterChunk(pos, 0)
	}

	// Camera at origin looking north (positive Z direction)
	cameraPos := lin.Vec2{0, 0}
	direction := float32(math.Pi / 2) // 90 degrees (north)
	fov := float32(math.Pi / 3)       // 60 degrees
	frustum := NewFrustumFromCamera(cameraPos, direction, fov, 10.0, 500.0)

	grid.UpdateVisibility(cameraPos, frustum)

	// Check visibility
	tests := []struct {
		index    int
		name     string
		expected bool
	}{
		{0, "Ahead", true},   // Should be visible
		{1, "Right", false},  // Outside FOV
		{2, "Left", false},   // Outside FOV
		{3, "Behind", false}, // Behind camera
	}

	for _, test := range tests {
		if chunks[test.index].InFrustum != test.expected {
			t.Errorf("Chunk %s: expected InFrustum=%v, got %v",
				test.name, test.expected, chunks[test.index].InFrustum)
		}
	}
}

func TestFrustumFromVectors(t *testing.T) {
	// Test creating frustum from position and front vector
	position := lin.Vec2{0, 0}
	front := lin.Vec2{0, 1}     // Looking north
	fov := float32(math.Pi / 3) // 60 degrees

	frustum := NewFrustumFromVectors(position, front, fov, 1.0, 100.0)

	// Test point inside frustum
	pointInside := lin.Vec2{0, 50} // Directly ahead
	if !IsPointInFrustum(pointInside, frustum) {
		t.Errorf("Point directly ahead should be in frustum")
	}

	// Test point outside frustum (to the side)
	pointOutside := lin.Vec2{100, 50} // Far to the right
	if IsPointInFrustum(pointOutside, frustum) {
		t.Errorf("Point far to the right should not be in frustum")
	}

	// Test point behind
	pointBehind := lin.Vec2{0, -50} // Behind camera
	if IsPointInFrustum(pointBehind, frustum) {
		t.Errorf("Point behind camera should not be in frustum")
	}
}

// Helper function to compare Vec2 values with epsilon
func vec2Equal(a, b lin.Vec2, epsilon float32) bool {
	for i := range 2 {
		if float32(math.Abs(float64(a[i]-b[i]))) > epsilon {
			return false
		}
	}
	return true
}

// Helper function to get absolute value of int
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
