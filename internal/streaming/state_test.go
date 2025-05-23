package streaming

import (
	"sync"
	"testing"
	"time"

	"github.com/cs-wwu/srg-synvis/core/spatial"
)

func TestString(t *testing.T) {
	tests := []struct {
		state    ChunkState
		expected string
	}{
		{StateUnloaded, "Unloaded"},
		{StateQueued, "Queued"},
		{StateLoading, "Loading"},
		{StateReady, "Ready"},
		{StateEvicting, "Evicting"},
		{ChunkState(999), "Unknown(999)"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("ChunkState(%d).String() = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestTransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		fromState ChunkState
		toState   ChunkState
		wantError bool
	}{
		// Valid transitions
		{"Unloaded to Queued", StateUnloaded, StateQueued, false},
		{"Queued to Loading", StateQueued, StateLoading, false},
		{"Loading to Ready", StateLoading, StateReady, false},
		{"Ready to Evicting", StateReady, StateEvicting, false},
		{"Any to Unloaded", StateReady, StateUnloaded, false},
		{"Evicting to Unloaded", StateEvicting, StateUnloaded, false},

		// Invalid transitions
		{"Unloaded to Loading", StateUnloaded, StateLoading, true},
		{"Queued to Ready", StateQueued, StateReady, true},
		{"Loading to Evicting", StateLoading, StateEvicting, true},
		{"Ready to Queued", StateReady, StateQueued, true},
		{"Evicting to Ready", StateEvicting, StateReady, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &ChunkRecord{State: tt.fromState}
			err := record.TransitionTo(tt.toState)

			if tt.wantError && err == nil {
				t.Errorf("Expected error for transition %s -> %s, but got none", tt.fromState, tt.toState)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error for transition %s -> %s: %v", tt.fromState, tt.toState, err)
			}
			if !tt.wantError && record.State != tt.toState {
				t.Errorf("State not updated: expected %s, got %s", tt.toState, record.State)
			}
		})
	}
}

func TestTouch(t *testing.T) {
	record := &ChunkRecord{
		LastUsed: time.Now().Add(-time.Hour),
	}

	oldTime := record.LastUsed
	record.Touch()

	if !record.LastUsed.After(oldTime) {
		t.Error("Touch() should update LastUsed to a more recent time")
	}

	// Should be very recent (within last second)
	if time.Since(record.LastUsed) > time.Second {
		t.Error("Touch() should set LastUsed to approximately now")
	}
}

func TestAge(t *testing.T) {
	record := &ChunkRecord{
		StateChanged: time.Now().Add(-time.Minute),
	}

	age := record.Age()
	if age < 50*time.Second || age > 70*time.Second {
		t.Errorf("Age() returned %v, expected approximately 1 minute", age)
	}
}

func TestNewChunkStateManager(t *testing.T) {
	csm := NewChunkStateManager()

	if csm == nil {
		t.Fatal("NewChunkStateManager() returned nil")
	}

	if csm.chunks == nil {
		t.Error("chunks map not initialized")
	}

	if csm.Count() != 0 {
		t.Errorf("Expected count 0 for new manager, got %d", csm.Count())
	}
}

func TestGetChunkState(t *testing.T) {
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// Non-existent chunk should return StateUnloaded
	if state := csm.GetChunkState(coord); state != StateUnloaded {
		t.Errorf("Expected StateUnloaded for non-existent chunk, got %s", state)
	}

	// Add a chunk and verify state
	err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
	if err != nil {
		t.Fatalf("SetChunkState failed: %v", err)
	}

	if state := csm.GetChunkState(coord); state != StateQueued {
		t.Errorf("Expected StateQueued, got %s", state)
	}
}

func TestSetChunkState(t *testing.T) {
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// New chunks must start with StateQueued
	err := csm.SetChunkState(coord, StateLoading, 1.0, 0)
	if err == nil {
		t.Error("Expected error when creating chunk with non-Queued state")
	}

	// Proper chunk creation
	err = csm.SetChunkState(coord, StateQueued, 2.5, 1)
	if err != nil {
		t.Fatalf("SetChunkState failed: %v", err)
	}

	// Verify the record was created correctly
	record := csm.GetChunkRecord(coord)
	if record == nil {
		t.Fatal("Record not created")
	}
	if record.Priority != 2.5 {
		t.Errorf("Expected priority 2.5, got %f", record.Priority)
	}
	if record.LODLevel != 1 {
		t.Errorf("Expected LOD level 1, got %d", record.LODLevel)
	}

	// Test state progression
	err = csm.SetChunkState(coord, StateLoading, 3.0, 2)
	if err != nil {
		t.Errorf("Transition to Loading failed: %v", err)
	}

	// Verify priority and LOD were updated
	record = csm.GetChunkRecord(coord)
	if record.Priority != 3.0 {
		t.Errorf("Priority not updated: expected 3.0, got %f", record.Priority)
	}
	if record.LODLevel != 2 {
		t.Errorf("LOD not updated: expected 2, got %d", record.LODLevel)
	}
}

func TestTouchChunk(t *testing.T) {
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// Touching non-existent chunk should not panic
	csm.TouchChunk(coord)

	// Create chunk and test touching
	err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
	if err != nil {
		t.Fatalf("SetChunkState failed: %v", err)
	}

	oldRecord := csm.GetChunkRecord(coord)
	time.Sleep(time.Millisecond) // Ensure time difference

	csm.TouchChunk(coord)

	newRecord := csm.GetChunkRecord(coord)
	if !newRecord.LastUsed.After(oldRecord.LastUsed) {
		t.Error("TouchChunk should update LastUsed time")
	}
}

func TestRemoveChunk(t *testing.T) {
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// Removing non-existent chunk should not error
	err := csm.RemoveChunk(coord)
	if err != nil {
		t.Errorf("RemoveChunk on non-existent chunk failed: %v", err)
	}

	// Create chunk and try to remove while in wrong state
	err = csm.SetChunkState(coord, StateQueued, 1.0, 0)
	if err != nil {
		t.Fatalf("SetChunkState failed: %v", err)
	}

	err = csm.RemoveChunk(coord)
	if err == nil {
		t.Error("Expected error when removing chunk in Queued state")
	}

	// Transition to valid state for removal
	err = csm.SetChunkState(coord, StateUnloaded, 1.0, 0)
	if err != nil {
		t.Fatalf("Transition to Unloaded failed: %v", err)
	}

	err = csm.RemoveChunk(coord)
	if err != nil {
		t.Errorf("RemoveChunk failed: %v", err)
	}

	// Verify removal
	if csm.GetChunkState(coord) != StateUnloaded {
		t.Error("Chunk should be unloaded after removal")
	}
	if csm.Count() != 0 {
		t.Errorf("Expected count 0 after removal, got %d", csm.Count())
	}
}

func TestListChunksByState(t *testing.T) {
	csm := NewChunkStateManager()

	// Create chunks in different states
	coords := []spatial.GridCoord{
		{X: 1, Z: 1},
		{X: 2, Z: 2},
		{X: 3, Z: 3},
	}

	// Add chunks
	for _, coord := range coords {
		err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
		if err != nil {
			t.Fatalf("SetChunkState failed: %v", err)
		}
	}

	// Transition one to Loading
	err := csm.SetChunkState(coords[1], StateLoading, 1.0, 0)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	// Test listing
	queuedChunks := csm.ListChunksByState(StateQueued)
	if len(queuedChunks) != 2 {
		t.Errorf("Expected 2 queued chunks, got %d", len(queuedChunks))
	}

	loadingChunks := csm.ListChunksByState(StateLoading)
	if len(loadingChunks) != 1 {
		t.Errorf("Expected 1 loading chunk, got %d", len(loadingChunks))
	}

	readyChunks := csm.ListChunksByState(StateReady)
	if len(readyChunks) != 0 {
		t.Errorf("Expected 0 ready chunks, got %d", len(readyChunks))
	}
}

func TestGetStats(t *testing.T) {
	csm := NewChunkStateManager()

	// Initially empty
	stats := csm.GetStats()
	if len(stats) != 0 {
		t.Errorf("Expected empty stats, got %v", stats)
	}

	// Add chunks in different states
	coords := []spatial.GridCoord{{X: 1, Z: 1}, {X: 2, Z: 2}, {X: 3, Z: 3}}
	for _, coord := range coords {
		err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
		if err != nil {
			t.Fatalf("SetChunkState failed: %v", err)
		}
	}

	// Transition one
	err := csm.SetChunkState(coords[0], StateLoading, 1.0, 0)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}

	stats = csm.GetStats()
	if stats[StateQueued] != 2 {
		t.Errorf("Expected 2 queued chunks, got %d", stats[StateQueued])
	}
	if stats[StateLoading] != 1 {
		t.Errorf("Expected 1 loading chunk, got %d", stats[StateLoading])
	}

	t.Logf("Stats: %v", stats)
}

func TestClear(t *testing.T) {
	csm := NewChunkStateManager()

	// Add some chunks
	coords := []spatial.GridCoord{{X: 1, Z: 1}, {X: 2, Z: 2}}
	for _, coord := range coords {
		err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
		if err != nil {
			t.Fatalf("SetChunkState failed: %v", err)
		}
	}

	if csm.Count() != 2 {
		t.Errorf("Expected count 2, got %d", csm.Count())
	}

	csm.Clear()

	if csm.Count() != 0 {
		t.Errorf("Expected count 0 after Clear(), got %d", csm.Count())
	}
}

func TestConcurrentAccess(t *testing.T) {
	csm := NewChunkStateManager()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Test concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				coord := spatial.GridCoord{X: id, Z: j}

				// Create chunk
				err := csm.SetChunkState(coord, StateQueued, float32(j), id)
				if err != nil {
					t.Errorf("SetChunkState failed: %v", err)
					continue
				}

				// Read state
				state := csm.GetChunkState(coord)
				if state != StateQueued {
					t.Errorf("Expected StateQueued, got %s", state)
				}

				// Touch chunk
				csm.TouchChunk(coord)

				// Get record
				record := csm.GetChunkRecord(coord)
				if record == nil {
					t.Error("GetChunkRecord returned nil")
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	expectedCount := numGoroutines * numOperations
	if csm.Count() != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, csm.Count())
	}

	stats := csm.GetStats()
	if stats[StateQueued] != expectedCount {
		t.Errorf("Expected %d queued chunks, got %d", expectedCount, stats[StateQueued])
	}
}

func TestGetChunkRecordReturnsCopy(t *testing.T) {
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
	if err != nil {
		t.Fatalf("SetChunkState failed: %v", err)
	}

	record1 := csm.GetChunkRecord(coord)
	record2 := csm.GetChunkRecord(coord)

	// Should be equal but different pointers
	if record1 == record2 {
		t.Error("GetChunkRecord should return copies, not the same pointer")
	}

	if record1.Priority != record2.Priority {
		t.Error("Copies should have same values")
	}

	// Modifying returned record should not affect internal state
	record1.Priority = 999.0
	record3 := csm.GetChunkRecord(coord)
	if record3.Priority == 999.0 {
		t.Error("Modifying returned record affected internal state")
	}
}
