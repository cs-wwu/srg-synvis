package streaming

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/cs-wwu/srg-synvis/core/containers"
	"github.com/cs-wwu/srg-synvis/core/spatial"
)

// JobType represents the type of background job
type JobType int

const (
	JobTypeLoad JobType = iota
	JobTypeEvict
)

// LoadJob represents a chunk loading job
type LoadJob struct {
	Spec     spatial.ChunkSpec
	MapCoord string
	Priority float32
	QueuedAt time.Time
}

// EvictJob represents a chunk eviction job
type EvictJob struct {
	Coord    spatial.GridCoord
	Priority float32 // Lower priority chunks evicted first
	QueuedAt time.Time
}

// ChunkLoader interface defines what the worker pool needs to load/evict chunks
type ChunkLoader interface {
	LoadChunk(spec spatial.ChunkSpec, mapCoord string) error
	EvictChunk(coord spatial.GridCoord) error
	SetChunkState(coord spatial.GridCoord, state ChunkState, priority float32, lodLevel int) error
	GetChunkState(coord spatial.GridCoord) ChunkState
	MarkChunkAvailable(coord spatial.ChunkSpec)
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	NumWorkers  int
	WorkerSleep time.Duration
}

// DefaultWorkerPoolConfig returns sensible defaults
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		NumWorkers:  5,
		WorkerSleep: 5 * time.Millisecond,
	}
}

// WorkerPool manages background workers for chunk loading and eviction
type WorkerPool struct {
	// Configuration
	config WorkerPoolConfig
	loader ChunkLoader

	// Job queues
	loadQueue  *containers.PriorityQueue[LoadJob]
	evictQueue *containers.PriorityQueue[EvictJob]

	// Worker management
	ctx      context.Context
	cancel   context.CancelFunc
	workerWg sync.WaitGroup

	// State
	isRunning    bool
	runningMutex sync.RWMutex
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(loader ChunkLoader, config WorkerPoolConfig) *WorkerPool {
	// Create priority queues
	loadQueue := containers.NewPriorityQueue(func(job LoadJob) float32 {
		return job.Priority
	})

	evictQueue := containers.NewPriorityQueue(func(job EvictJob) float32 {
		// Lower priority = evict first, so negate
		return -job.Priority
	})

	return &WorkerPool{
		config:     config,
		loader:     loader,
		loadQueue:  loadQueue,
		evictQueue: evictQueue,
	}
}

// Start begins background processing
func (wp *WorkerPool) Start() {
	wp.runningMutex.Lock()
	defer wp.runningMutex.Unlock()

	if wp.isRunning {
		return
	}

	wp.ctx, wp.cancel = context.WithCancel(context.Background())
	wp.isRunning = true

	// Start worker pool
	for i := range wp.config.NumWorkers {
		wp.workerWg.Add(1)
		go wp.workerLoop(i)
	}

	log.Printf("WorkerPool started with %d workers", wp.config.NumWorkers)
}

// Stop shuts down background processing
func (wp *WorkerPool) Stop() {
	wp.runningMutex.Lock()
	defer wp.runningMutex.Unlock()

	if !wp.isRunning {
		return
	}

	wp.isRunning = false
	wp.cancel()

	// Wait for workers to finish
	wp.workerWg.Wait()

	// Clear queues
	wp.loadQueue.Clear()
	wp.evictQueue.Clear()

	log.Printf("WorkerPool stopped")
}

// IsRunning returns whether background processing is active
func (wp *WorkerPool) IsRunning() bool {
	wp.runningMutex.RLock()
	defer wp.runningMutex.RUnlock()
	return wp.isRunning
}

// QueueLoadJob adds a load job to the queue
func (wp *WorkerPool) QueueLoadJob(spec spatial.ChunkSpec, mapCoord string) {
	// Check if already queued or loading
	currentState := wp.loader.GetChunkState(spec.GridCoord)
	if currentState != StateUnloaded && currentState != StateReady {
		return
	}

	// Queue the chunk in state manager
	err := wp.loader.SetChunkState(spec.GridCoord, StateQueued, spec.Priority, spec.LODLevel)
	if err != nil {
		log.Printf("Failed to queue chunk %v: %v", spec.GridCoord, err)
		return
	}

	// Add to load queue
	job := LoadJob{
		Spec:     spec,
		MapCoord: mapCoord,
		Priority: spec.Priority,
		QueuedAt: time.Now(),
	}

	wp.loadQueue.Enqueue(job)
}

// QueueEvictJob adds an evict job to the queue
func (wp *WorkerPool) QueueEvictJob(coord spatial.GridCoord, priority float32) {
	job := EvictJob{
		Coord:    coord,
		Priority: priority,
		QueuedAt: time.Now(),
	}
	wp.evictQueue.Enqueue(job)
}

// GetQueueSizes returns the current size of load and evict queues
func (wp *WorkerPool) GetQueueSizes() (loadSize, evictSize int) {
	return wp.loadQueue.Size(), wp.evictQueue.Size()
}

// workerLoop is the main worker thread loop
func (wp *WorkerPool) workerLoop(workerID int) {
	defer wp.workerWg.Done()

	log.Printf("Worker %d started", workerID)

	for {
		select {
		case <-wp.ctx.Done():
			log.Printf("Worker %d shutting down", workerID)
			return
		default:
			// Try to get work
			if wp.processNextJob(workerID) {
				continue // Got work, immediately check for more
			}

			// No work available, wait a bit
			time.Sleep(wp.config.WorkerSleep)
		}
	}
}

// processNextJob processes the next available job
func (wp *WorkerPool) processNextJob(workerID int) bool {
	// Prioritize load jobs over evict jobs
	if loadJob, hasLoad := wp.loadQueue.Dequeue(); hasLoad {
		wp.processLoadJob(workerID, loadJob)
		return true
	}

	if evictJob, hasEvict := wp.evictQueue.Dequeue(); hasEvict {
		wp.processEvictJob(workerID, evictJob)
		return true
	}

	return false
}

// processLoadJob handles loading a chunk
func (wp *WorkerPool) processLoadJob(workerID int, job LoadJob) {
	// Verify chunk is still in queued state
	currentState := wp.loader.GetChunkState(job.Spec.GridCoord)
	if currentState != StateQueued {
		return // Chunk was cancelled or already processed
	}

	// Transition to loading
	err := wp.loader.SetChunkState(job.Spec.GridCoord, StateLoading, job.Priority, job.Spec.LODLevel)
	if err != nil {
		log.Printf("Worker %d: Failed to transition chunk %v to loading: %v", workerID, job.Spec.GridCoord, err)
		return
	}

	// Load the chunk
	err = wp.loader.LoadChunk(job.Spec, job.MapCoord)
	if err != nil {
		log.Printf("Worker %d: Failed to load chunk %v: %v", workerID, job.Spec.GridCoord, err)
		// Mark as failed
		wp.loader.SetChunkState(job.Spec.GridCoord, StateUnloaded, job.Priority, job.Spec.LODLevel)
		return
	}

	// Mark as available in ChunkSelector
	wp.loader.MarkChunkAvailable(job.Spec)
}

// processEvictJob handles evicting a chunk
func (wp *WorkerPool) processEvictJob(workerID int, job EvictJob) {
	err := wp.loader.EvictChunk(job.Coord)
	if err != nil {
		log.Printf("Worker %d: Failed to evict chunk %v: %v", workerID, job.Coord, err)
	}
}

// Stats returns worker pool statistics
func (wp *WorkerPool) Stats() WorkerPoolStats {
	loadSize, evictSize := wp.GetQueueSizes()

	return WorkerPoolStats{
		IsRunning:      wp.IsRunning(),
		NumWorkers:     wp.config.NumWorkers,
		LoadQueueSize:  loadSize,
		EvictQueueSize: evictSize,
	}
}

// WorkerPoolStats holds statistics about the worker pool
type WorkerPoolStats struct {
	IsRunning      bool
	NumWorkers     int
	LoadQueueSize  int
	EvictQueueSize int
}
