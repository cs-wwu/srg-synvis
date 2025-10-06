package spatial

import (
	"math"
	"testing"

	lin "github.com/xlab/linmath"
)

const (
	testChunkSize = 180.0
	tolerance     = 0.001
)

func TestNewChunkSelector(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)
	if cs == nil {
		t.Fatal("NewChunkSelector returned nil")
	}
	if cs.chunkSize != testChunkSize {
		t.Errorf("Expected chunk size %f, got %f", testChunkSize, cs.chunkSize)
	}
	if cs.lodStrategy == nil {
		t.Error("Expected default LOD strategy to be created")
	}
}

func TestWorldToGridConversion(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	tests := []struct {
		worldPos lin.Vec2
		expected GridCoord
	}{
		{lin.Vec2{0, 0}, GridCoord{0, 0}},
		{lin.Vec2{180, 180}, GridCoord{1, 1}},
		{lin.Vec2{-180, -180}, GridCoord{-1, -1}},
		{lin.Vec2{90, 90}, GridCoord{0, 0}},     // Center of chunk 0,0
		{lin.Vec2{270, 270}, GridCoord{1, 1}},   // Center of chunk 1,1
		{lin.Vec2{-90, -90}, GridCoord{-1, -1}}, // Center of chunk -1,-1
	}

	for _, test := range tests {
		result := cs.WorldToGrid(test.worldPos)
		if result != test.expected {
			t.Errorf("WorldToGrid(%v) = %v, expected %v", test.worldPos, result, test.expected)
		}
	}
}

func TestGridToWorldConversion(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	tests := []struct {
		gridCoord GridCoord
		expected  lin.Vec2
	}{
		{GridCoord{0, 0}, lin.Vec2{90, 90}},     // Center of chunk 0,0
		{GridCoord{1, 1}, lin.Vec2{270, 270}},   // Center of chunk 1,1
		{GridCoord{-1, -1}, lin.Vec2{-90, -90}}, // Center of chunk -1,-1
	}

	for _, test := range tests {
		result := cs.GridToWorld(test.gridCoord)
		if math.Abs(float64(result[0]-test.expected[0])) > tolerance ||
			math.Abs(float64(result[1]-test.expected[1])) > tolerance {
			t.Errorf("GridToWorld(%v) = %v, expected %v", test.gridCoord, result, test.expected)
		}
	}
}

func TestRoundTripConversion(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	testPositions := []lin.Vec2{
		{0, 0}, {100, 200}, {-150, 300}, {500, -400},
	}

	for _, pos := range testPositions {
		coord := cs.WorldToGrid(pos)
		backToWorld := cs.GridToWorld(coord)

		// Should be within the same chunk
		coordAgain := cs.WorldToGrid(backToWorld)
		if coord != coordAgain {
			t.Errorf("Round trip failed: %v -> %v -> %v -> %v", pos, coord, backToWorld, coordAgain)
		}
	}
}

func TestGetNeighborCoords(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	center := GridCoord{5, 10}
	neighbors := cs.GetNeighborCoords(center)

	expected := [4]GridCoord{
		{5, 9},  // North
		{6, 10}, // East
		{5, 11}, // South
		{4, 10}, // West
	}

	for i, expected := range expected {
		if neighbors[i] != expected {
			t.Errorf("Neighbor %d: got %v, expected %v", i, neighbors[i], expected)
		}
	}
}

func TestLODStrategy(t *testing.T) {
	lodStrategy := NewDefaultLODStrategy(3, 500.0, 2.0)
	cs := NewChunkSelector(testChunkSize, lodStrategy)

	tests := []struct {
		distance    float32
		expectedLOD int
	}{
		{100, 0},   // Close distance -> highest detail
		{500, 0},   // Base distance -> LOD 0
		{1100, 1},  // 2x base distance -> LOD 1
		{2000, 2},  // 4x base distance -> LOD 2
		{10000, 2}, // Very far -> clamped to max LOD
	}

	cameraPos := lin.Vec2{0, 0}
	for _, test := range tests {
		// Position chunk at the test distance
		coord := GridCoord{int(test.distance / testChunkSize), 0}
		result := cs.CalculateLOD(coord, cameraPos)
		if result != test.expectedLOD {
			t.Errorf("CalculateLOD at distance %f: got LOD %d, expected %d", test.distance, result, test.expectedLOD)
		}
	}
}

func TestPriorityCalculation(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)
	cameraPos := lin.Vec2{0, 0}

	// Test priority differences
	closeCoord := GridCoord{0, 0} // At camera position
	farCoord := GridCoord{10, 10} // Far from camera

	closePriorityInFrustum := cs.CalculatePriority(closeCoord, cameraPos, true)
	closePriorityOutFrustum := cs.CalculatePriority(closeCoord, cameraPos, false)
	farPriorityInFrustum := cs.CalculatePriority(farCoord, cameraPos, true)

	// In-frustum should have higher priority than out-of-frustum
	if closePriorityInFrustum <= closePriorityOutFrustum {
		t.Error("In-frustum priority should be higher than out-of-frustum")
	}

	// Closer chunks should have higher priority
	if farPriorityInFrustum >= closePriorityInFrustum {
		t.Error("Closer chunks should have higher priority")
	}
}

func TestUpdateSelection(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	// Create a simple frustum that includes everything (for testing)
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(500)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	desired := cs.GetDesiredChunks()
	if len(desired) == 0 {
		t.Error("Expected some chunks to be selected")
	}

	// Check that all desired chunks are within render distance
	for _, spec := range desired {
		if spec.Distance > renderDistance {
			t.Errorf("Chunk %v at distance %f exceeds render distance %f",
				spec.GridCoord, spec.Distance, renderDistance)
		}
	}
}

func TestChunkAvailabilityTracking(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	coord := GridCoord{1, 1}

	// Initially should not be available
	if cs.IsChunkAvailable(coord) {
		t.Error("Chunk should not be available initially")
	}

	// Mark as available
	cs.MarkChunkAvailable(ChunkSpec{
		GridCoord: coord,
		LODLevel:  1,
	})
	if !cs.IsChunkAvailable(coord) {
		t.Error("Chunk should be available after marking")
	}

	// Mark as unavailable
	cs.MarkChunkUnavailable(coord)
	if cs.IsChunkAvailable(coord) {
		t.Error("Chunk should not be available after unmarking")
	}
}

func TestMissingChunks(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	// Set up a selection with some chunks
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(300)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	// Initially, all should be missing
	missing := cs.GetDirtyChunks()
	desired := cs.GetDesiredChunks()

	if len(missing) != len(desired) {
		t.Errorf("Expected %d missing chunks, got %d", len(desired), len(missing))
	}

	// Mark some chunks as available
	if len(desired) > 0 {
		cs.MarkChunkAvailable(desired[0])

		newMissing := cs.GetDirtyChunks()
		if len(newMissing) != len(desired)-1 {
			t.Errorf("Expected %d missing chunks after marking one available, got %d",
				len(desired)-1, len(newMissing))
		}
	}
}

func TestHighestPriorityMissing(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	// Set up selection
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(400)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	highest := cs.GetHighestPriorityMissing()
	if highest == nil {
		t.Error("Expected a highest priority chunk")
		return
	}

	// Should be closest to camera (highest priority)
	missing := cs.GetDirtyChunks()
	for _, chunk := range missing {
		if chunk.Priority > highest.Priority {
			t.Errorf("Found chunk with higher priority than 'highest': %f > %f",
				chunk.Priority, highest.Priority)
		}
	}
}

func TestChunkDesiredStatus(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	coord := GridCoord{0, 0}

	// Initially should not be desired
	if cs.IsChunkDesired(coord) {
		t.Error("Chunk should not be desired initially")
	}

	// After selection that includes this chunk, it should be desired
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{90, 90} // Center of chunk 0,0
	renderDistance := float32(200)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	if !cs.IsChunkDesired(coord) {
		t.Error("Chunk should be desired after selection")
	}

	// After moving camera far away, it should not be desired
	farCameraPos := lin.Vec2{10000, 10000}
	cs.UpdateSelection(farCameraPos, frustum, renderDistance)

	if cs.IsChunkDesired(coord) {
		t.Error("Chunk should not be desired after moving camera away")
	}
}

func TestClear(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	// Set up some state
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(300)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	coord := GridCoord{1, 1}
	cs.MarkChunkAvailable(ChunkSpec{
		GridCoord: coord,
	})

	// Verify state exists
	if len(cs.GetDesiredChunks()) == 0 {
		t.Error("Expected desired chunks before clear")
	}

	// Clear and verify everything is reset
	cs.Clear()

	if len(cs.GetDesiredChunks()) != 0 {
		t.Error("Expected no desired chunks after clear")
	}

	if cs.IsChunkAvailable(coord) {
		t.Error("Expected no available chunks after clear")
	}

	highest := cs.GetHighestPriorityMissing()
	if highest != nil {
		t.Error("Expected no missing chunks after clear")
	}
}

func TestStats(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	// Initially should have empty stats
	stats := cs.Stats()
	if stats.DesiredChunks != 0 || stats.AvailableChunks != 0 || stats.MissingChunks != 0 {
		t.Error("Expected empty stats initially")
	}

	// Set up some chunks
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(300)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	// Mark some as available
	desired := cs.GetDesiredChunks()
	if len(desired) > 1 {
		cs.MarkChunkAvailable(desired[0])
	}

	// Check stats
	stats = cs.Stats()
	if stats.DesiredChunks != len(desired) {
		t.Errorf("Expected %d desired chunks in stats, got %d", len(desired), stats.DesiredChunks)
	}

	if len(desired) > 1 && stats.AvailableChunks != 1 {
		t.Errorf("Expected 1 available chunk in stats, got %d", stats.AvailableChunks)
	}

	expectedMissing := len(desired) - stats.AvailableChunks
	if stats.MissingChunks != expectedMissing {
		t.Errorf("Expected %d missing chunks in stats, got %d", expectedMissing, stats.MissingChunks)
	}

	t.Logf("Stats: %+v", stats)
	t.Logf("Detailed stats: %s", cs.GetDetailedStats())
	t.Logf("Visual Selection: %s", cs.VisualizeSelection())
	t.Logf("Visual Priorities: %s", cs.VisualizePriorities())
	t.Logf("Visual Loading Efficiency: %s", cs.VisualizeLoadingEfficiency())
	t.Logf("Neighbor Coverage: %s", cs.AnalyzeNeighborCoverage())
}

func TestConcurrentAccess(t *testing.T) {
	cs := NewChunkSelector(testChunkSize, nil)

	// Set up initial state
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(300)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	// Test concurrent read access
	done := make(chan bool, 2)

	go func() {
		for range 100 {
			cs.GetDesiredChunks()
			cs.GetDirtyChunks()
			cs.Stats()
		}
		done <- true
	}()

	go func() {
		for i := range 100 {
			coord := GridCoord{i % 10, i % 10}
			cs.MarkChunkAvailable(ChunkSpec{GridCoord: coord})
			cs.IsChunkAvailable(coord)
			cs.MarkChunkUnavailable(coord)
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// If we get here without deadlocking, the test passes
}

// Helper function to create a simple frustum for testing
func createTestFrustum() Frustum {
	// Create a wide frustum that includes most chunks around origin
	// 180 degrees wide
	return Frustum{
		Planes: [4]FrustumPlane{
			// Left plane (normal points right)
			{Normal: lin.Vec2{1, 0}, D: 5000},
			// Right plane (normal points left)
			{Normal: lin.Vec2{-1, 0}, D: 5000},
			// Near plane (normal points away)
			{Normal: lin.Vec2{0, 1}, D: 10},
			// Far plane (normal points toward)
			{Normal: lin.Vec2{0, -1}, D: 5000},
		},
	}
}

// Benchmark tests
func BenchmarkUpdateSelection(b *testing.B) {
	cs := NewChunkSelector(testChunkSize, nil)
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(1000)

	for b.Loop() {
		cs.UpdateSelection(cameraPos, frustum, renderDistance)
	}
}

func BenchmarkGetMissingChunks(b *testing.B) {
	cs := NewChunkSelector(testChunkSize, nil)
	frustum := createTestFrustum()
	cameraPos := lin.Vec2{0, 0}
	renderDistance := float32(1000)

	cs.UpdateSelection(cameraPos, frustum, renderDistance)

	for b.Loop() {
		cs.GetDirtyChunks()
	}
}

func BenchmarkMarkChunkAvailable(b *testing.B) {
	cs := NewChunkSelector(testChunkSize, nil)

	for i := 0; b.Loop(); i++ {
		coord := GridCoord{i % 100, i % 100}
		cs.MarkChunkAvailable(ChunkSpec{GridCoord: coord})
	}
}
