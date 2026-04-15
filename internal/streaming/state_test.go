package streaming

import (
	"sync"
	"testing"
	"time"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	assert := assert.New(t)

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
		got := tt.state.String()
		assert.Equal(tt.expected, got)
	}
}

func TestTransitionTo(t *testing.T) {
	assert := assert.New(t)
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
	assert.NotEmpty(tests)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &ChunkRecord{State: tt.fromState}
			err := record.TransitionTo(tt.toState)

			if tt.wantError {
				assert.Error(err)
			} else {
				assert.NoError(err)
				assert.Equal(tt.toState, record.State)
			}
		})
	}
}

func TestTouch(t *testing.T) {
	assert := assert.New(t)

	record := &ChunkRecord{
		LastUsed: time.Now().Add(-time.Hour),
	}

	oldTime := record.LastUsed
	record.Touch()

	assert.True(record.LastUsed.After(oldTime))

	// Should be very recent (within last second)
	assert.LessOrEqual(time.Since(record.LastUsed), time.Second)
}

func TestAge(t *testing.T) {
	assert := assert.New(t)

	record := &ChunkRecord{
		StateChanged: time.Now().Add(-time.Minute),
	}

	age := record.Age()
	assert.GreaterOrEqual(age, 50*time.Second)
	assert.LessOrEqual(age, 70*time.Second)
}

func TestNewChunkStateManager(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()

	assert.NotNil(csm)
	assert.NotNil(csm.chunks)
	assert.Equal(0, csm.Count())
}

func TestGetChunkState(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// Non-existent chunk should return StateUnloaded
	state := csm.GetChunkState(coord)
	assert.Equal(StateUnloaded, state)

	// Add a chunk and verify state
	err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
	assert.NoError(err)

	state = csm.GetChunkState(coord)
	assert.Equal(StateQueued, state)
}

func TestSetChunkState(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// New chunks must start with StateQueued
	err := csm.SetChunkState(coord, StateLoading, 1.0, 0)
	assert.Error(err)

	// Proper chunk creation
	err = csm.SetChunkState(coord, StateQueued, 2.5, 1)
	assert.NoError(err)

	// Verify the record was created correctly
	record := csm.GetChunkRecord(coord)
	assert.NotNil(record)
	if record == nil {
		return
	}
	assert.Equal(float32(2.5), record.Priority)
	assert.Equal(1, record.LODLevel)

	// Test state progression
	err = csm.SetChunkState(coord, StateLoading, 3.0, 2)
	assert.NoError(err)

	// Verify priority and LOD were updated
	record = csm.GetChunkRecord(coord)
	assert.Equal(float32(3.0), record.Priority)
	assert.Equal(2, record.LODLevel)
}

func TestTouchChunk(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// Touching non-existent chunk should not panic
	csm.TouchChunk(coord)

	// Create chunk and test touching
	err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
	assert.NoError(err)

	oldRecord := csm.GetChunkRecord(coord)
	time.Sleep(time.Millisecond) // Ensure time difference

	csm.TouchChunk(coord)

	newRecord := csm.GetChunkRecord(coord)
	assert.True(newRecord.LastUsed.After(oldRecord.LastUsed))
}

func TestRemoveChunk(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	// Removing non-existent chunk should not error
	err := csm.RemoveChunk(coord)
	assert.NoError(err)

	// Create chunk and try to remove while in wrong state
	err = csm.SetChunkState(coord, StateQueued, 1.0, 0)
	assert.NoError(err)

	err = csm.RemoveChunk(coord)
	assert.Error(err)

	// Transition to valid state for removal
	err = csm.SetChunkState(coord, StateUnloaded, 1.0, 0)
	assert.NoError(err)

	err = csm.RemoveChunk(coord)
	assert.NoError(err)

	// Verify removal
	assert.Equal(StateUnloaded, csm.GetChunkState(coord))
	assert.Equal(0, csm.Count())
}

func TestListChunksByState(t *testing.T) {
	assert := assert.New(t)
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
		assert.NoError(err)
	}

	// Transition one to Loading
	err := csm.SetChunkState(coords[1], StateLoading, 1.0, 0)
	assert.NoError(err)

	// Test listing
	queuedChunks := csm.ListChunksByState(StateQueued)
	assert.Len(queuedChunks, 2)

	loadingChunks := csm.ListChunksByState(StateLoading)
	assert.Len(loadingChunks, 1)

	readyChunks := csm.ListChunksByState(StateReady)
	assert.Len(readyChunks, 0)
}

func TestGetStats(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()

	// Initially empty
	stats := csm.GetStats()
	assert.Len(stats, 0)

	// Add chunks in different states
	coords := []spatial.GridCoord{{X: 1, Z: 1}, {X: 2, Z: 2}, {X: 3, Z: 3}}
	for _, coord := range coords {
		err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
		assert.NoError(err)
	}

	// Transition one
	err := csm.SetChunkState(coords[0], StateLoading, 1.0, 0)
	assert.NoError(err)

	stats = csm.GetStats()
	assert.Equal(2, stats[StateQueued])
	assert.Equal(1, stats[StateLoading])

	t.Logf("Stats: %v", stats)
}

func TestClear(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()

	// Add some chunks
	coords := []spatial.GridCoord{{X: 1, Z: 1}, {X: 2, Z: 2}}
	for _, coord := range coords {
		err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
		assert.NoError(err)
	}

	assert.Equal(2, csm.Count())

	csm.Clear()

	assert.Equal(0, csm.Count())
}

func TestConcurrentAccess(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup

	// Test concurrent reads and writes
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := range numOperations {
				coord := spatial.GridCoord{X: id, Z: j}

				// Create chunk
				err := csm.SetChunkState(coord, StateQueued, float32(j), id)
				if !assert.NoError(err) {
					continue
				}

				// Read state
				state := csm.GetChunkState(coord)
				assert.Equal(StateQueued, state)

				// Touch chunk
				csm.TouchChunk(coord)

				// Get record
				record := csm.GetChunkRecord(coord)
				assert.NotNil(record)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	expectedCount := numGoroutines * numOperations
	assert.Equal(expectedCount, csm.Count())

	stats := csm.GetStats()
	assert.Equal(expectedCount, stats[StateQueued])
}

func TestGetChunkRecordReturnsCopy(t *testing.T) {
	assert := assert.New(t)
	csm := NewChunkStateManager()
	coord := spatial.GridCoord{X: 1, Z: 2}

	err := csm.SetChunkState(coord, StateQueued, 1.0, 0)
	assert.NoError(err)

	record1 := csm.GetChunkRecord(coord)
	record2 := csm.GetChunkRecord(coord)

	// Should be equal but different pointers
	assert.NotSame(record1, record2)

	assert.Equal(record1.Priority, record2.Priority)

	// Modifying returned record should not affect internal state
	record1.Priority = 999.0
	record3 := csm.GetChunkRecord(coord)
	assert.NotEqual(float32(999.0), record3.Priority)
}
