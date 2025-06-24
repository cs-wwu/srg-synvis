package streaming

import (
	"fmt"
	"sync"
	"time"

	"github.com/cs-wwu/srg-synvis/core/spatial"
)

// ChunkState represents the lifecycle state of a chunk
type ChunkState int

const (
	StateUnloaded ChunkState = iota // Chunk doesn't exist in memory
	StateQueued                     // Chunk is queued for loading
	StateLoading                    // Chunk is currently being loaded
	StateReady                      // Chunk is loaded and ready for rendering
	StateEvicting                   // Chunk is marked for eviction
)

// String returns a human-readable representation of the chunk state
func (cs ChunkState) String() string {
	switch cs {
	case StateUnloaded:
		return "Unloaded"
	case StateQueued:
		return "Queued"
	case StateLoading:
		return "Loading"
	case StateReady:
		return "Ready"
	case StateEvicting:
		return "Evicting"
	default:
		return fmt.Sprintf("Unknown(%d)", int(cs))
	}
}

// ChunkRecord holds metadata about a chunk's current state and priority
type ChunkRecord struct {
	GridCoord    spatial.GridCoord // Grid coordinates of the chunk
	State        ChunkState        // Current lifecycle state
	Priority     float32           // Loading/eviction priority (higher = more important)
	LastUsed     time.Time         // Last time this chunk was accessed
	StateChanged time.Time         // When the chunk last changed state
	LODLevel     int               // Level of detail for this chunk
}

// TransitionTo attempts to transition the chunk to a new state
// Returns an error if the transition is invalid
func (cr *ChunkRecord) TransitionTo(newState ChunkState) error {
	if !isValidTransition(cr.State, newState) {
		return fmt.Errorf("invalid state transition from %s to %s", cr.State, newState)
	}

	cr.State = newState
	cr.StateChanged = time.Now()
	return nil
}

// Touch updates the LastUsed timestamp to the current time
func (cr *ChunkRecord) Touch() {
	cr.LastUsed = time.Now()
}

// Age returns how long the chunk has been in its current state
func (cr *ChunkRecord) Age() time.Duration {
	return time.Since(cr.StateChanged)
}

// isValidTransition checks if a state transition is allowed
func isValidTransition(from, to ChunkState) bool {
	// Allow any state to transition to Unloaded (for cleanup/errors)
	if to == StateUnloaded {
		return true
	}

	switch from {
	case StateUnloaded:
		return to == StateQueued
	case StateQueued:
		return to == StateLoading
	case StateLoading:
		return to == StateReady
	case StateReady:
		return to == StateEvicting
	case StateEvicting:
		return false // Can only go to Unloaded
	default:
		return false
	}
}

// ChunkStateManager provides thread-safe management of chunk states
type ChunkStateManager struct {
	chunks map[spatial.GridCoord]*ChunkRecord
	mutex  sync.RWMutex
}

// NewChunkStateManager creates a new chunk state manager
func NewChunkStateManager() *ChunkStateManager {
	return &ChunkStateManager{
		chunks: make(map[spatial.GridCoord]*ChunkRecord),
	}
}

// GetChunkState returns the current state of a chunk
// Returns StateUnloaded if the chunk doesn't exist
func (csm *ChunkStateManager) GetChunkState(coord spatial.GridCoord) ChunkState {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()

	if record, exists := csm.chunks[coord]; exists {
		return record.State
	}
	return StateUnloaded
}

// GetChunkRecord returns a copy of the chunk record
// Returns nil if the chunk doesn't exist
func (csm *ChunkStateManager) GetChunkRecord(coord spatial.GridCoord) *ChunkRecord {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()

	if record, exists := csm.chunks[coord]; exists {
		// Return a copy to prevent external modification
		recordCopy := *record
		return &recordCopy
	}
	return nil
}

// SetChunkState transitions a chunk to a new state
// Creates the chunk record if it doesn't exist
func (csm *ChunkStateManager) SetChunkState(coord spatial.GridCoord, newState ChunkState, priority float32, lodLevel int) error {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()

	record, exists := csm.chunks[coord]
	if !exists {
		// Create new record
		if newState != StateQueued {
			return fmt.Errorf("new chunks must start in Queued state, got %s", newState)
		}

		now := time.Now()
		record = &ChunkRecord{
			GridCoord:    coord,
			State:        StateUnloaded, // Start as unloaded for proper transition
			Priority:     priority,
			LastUsed:     now,
			StateChanged: now,
			LODLevel:     lodLevel,
		}
		csm.chunks[coord] = record
	}

	// Update priority and LOD (they can change over time)
	record.Priority = priority
	record.LODLevel = lodLevel

	// Attempt state transition
	return record.TransitionTo(newState)
}

// TouchChunk updates the last used time for a chunk
func (csm *ChunkStateManager) TouchChunk(coord spatial.GridCoord) {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()

	if record, exists := csm.chunks[coord]; exists {
		record.Touch()
	}
}

// RemoveChunk removes a chunk from tracking
// The chunk must be in StateUnloaded or StateEvicting
func (csm *ChunkStateManager) RemoveChunk(coord spatial.GridCoord) error {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()

	record, exists := csm.chunks[coord]
	if !exists {
		return nil // Already removed
	}

	if record.State != StateUnloaded && record.State != StateEvicting {
		return fmt.Errorf("cannot remove chunk in state %s", record.State)
	}

	delete(csm.chunks, coord)
	return nil
}

// ListChunksByState returns all chunks currently in the specified state
// Returns copies of the records to prevent external modification
func (csm *ChunkStateManager) ListChunksByState(state ChunkState) []*ChunkRecord {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()

	var results []*ChunkRecord
	for _, record := range csm.chunks {
		if record.State == state {
			recordCopy := *record
			results = append(results, &recordCopy)
		}
	}
	return results
}

// GetAllChunks returns copies of all chunk records
func (csm *ChunkStateManager) GetAllChunks() []*ChunkRecord {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()

	results := make([]*ChunkRecord, 0, len(csm.chunks))
	for _, record := range csm.chunks {
		recordCopy := *record
		results = append(results, &recordCopy)
	}
	return results
}

// GetStats returns statistics about chunks in each state
func (csm *ChunkStateManager) GetStats() map[ChunkState]int {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()

	stats := make(map[ChunkState]int)
	for _, record := range csm.chunks {
		stats[record.State]++
	}
	return stats
}

// Count returns the total number of tracked chunks
func (csm *ChunkStateManager) Count() int {
	csm.mutex.RLock()
	defer csm.mutex.RUnlock()

	return len(csm.chunks)
}

// Clear removes all chunks from tracking
// This should only be used during shutdown or testing
func (csm *ChunkStateManager) Clear() {
	csm.mutex.Lock()
	defer csm.mutex.Unlock()

	csm.chunks = make(map[spatial.GridCoord]*ChunkRecord)
}
