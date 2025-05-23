package rendering

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cs-wwu/srg-synvis/core/containers"
	"github.com/cs-wwu/srg-synvis/core/memory"
	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/assets"
	"github.com/cs-wwu/srg-synvis/internal/coords"
	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/streaming"

	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

const (
	CHUNK_SIZE = 120
	MAP_FOLDER = "assets/maps/"

	// Worker configuration
	DEFAULT_WORKERS        = 3
	COORDINATOR_TICK       = 50 * time.Millisecond
	CAMERA_UPDATE_DEBOUNCE = 25 * time.Millisecond
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

// RenderableChunk represents a chunk ready for GPU rendering
type RenderableChunk struct {
	GridCoord      spatial.GridCoord
	LODLevel       int
	VertexBufferId string
	IndexBufferId  string
	VertexOffset   vk.DeviceSize
	IndexOffset    vk.DeviceSize
	IndexCount     int
}

// StreamManager coordinates chunk loading and manages the spatial/memory systems
type StreamManager struct {
	// Core systems
	chunkSelector   *spatial.ChunkSelector
	bucketAllocator *memory.BucketAllocator
	assetManager    assets.AssetManager
	stateManager    *streaming.ChunkStateManager

	// Vulkan resources
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	// Renderable chunks (only chunks in Ready state)
	renderableChunks map[spatial.GridCoord]*RenderableChunk
	renderMutex      sync.RWMutex

	// Background processing
	loadQueue  *containers.PriorityQueue[LoadJob]
	evictQueue *containers.PriorityQueue[EvictJob]

	// Communication channels
	cameraUpdates chan lin.Vec2
	shutdown      chan struct{}

	// Worker management
	ctx        context.Context
	cancel     context.CancelFunc
	workerWg   sync.WaitGroup
	numWorkers int

	// State
	lastCameraPos    lin.Vec2
	lastCameraUpdate time.Time
	isRunning        bool
	runningMutex     sync.RWMutex

	cfg models.AppConfig
}

// NewStreamManager creates a new stream manager with all subsystems
func NewStreamManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties, cfg models.AppConfig) *StreamManager {
	// Initialize ChunkSelector (replaces old TerrainGrid)
	chunkSelector := spatial.NewChunkSelector(CHUNK_SIZE, nil)

	// Initialize BucketAllocator (same as before)
	lodChunkSizes := CalculateLODChunkSizes()
	bucketAllocator := memory.NewBucketAllocator(device, memProps,
		[]float32{0.3, 0.25, 0.25, 0.2}, lodChunkSizes)

	// Initialize AssetManager (same as before)
	assetManager := assets.NewAssetManager(MAP_FOLDER, 27)

	// Initialize ChunkStateManager
	stateManager := streaming.NewChunkStateManager()

	// Create priority queues
	loadQueue := containers.NewPriorityQueue(func(job LoadJob) float32 {
		return job.Priority
	})

	evictQueue := containers.NewPriorityQueue(func(job EvictJob) float32 {
		// Lower priority = evict first, so negate
		return -job.Priority
	})

	sm := &StreamManager{
		chunkSelector:    chunkSelector,
		bucketAllocator:  bucketAllocator,
		assetManager:     assetManager,
		stateManager:     stateManager,
		device:           device,
		memProps:         memProps,
		cfg:              cfg,
		renderableChunks: make(map[spatial.GridCoord]*RenderableChunk),
		loadQueue:        loadQueue,
		evictQueue:       evictQueue,
		cameraUpdates:    make(chan lin.Vec2, 10), // Buffered channel
		shutdown:         make(chan struct{}),
		numWorkers:       DEFAULT_WORKERS,
	}

	log.Printf("StreamManager initialized with background workers")
	return sm
}

// Start begins background processing
func (sm *StreamManager) Start() {
	sm.runningMutex.Lock()
	defer sm.runningMutex.Unlock()

	if sm.isRunning {
		return
	}

	sm.ctx, sm.cancel = context.WithCancel(context.Background())
	sm.isRunning = true

	// Start coordinator
	go sm.coordinatorLoop()

	// Start worker pool
	for i := range sm.numWorkers {
		sm.workerWg.Add(1)
		go sm.workerLoop(i)
	}

	log.Printf("StreamManager started with %d workers", sm.numWorkers)
}

// Stop shuts down background processing
func (sm *StreamManager) Stop() {
	sm.runningMutex.Lock()
	defer sm.runningMutex.Unlock()

	if !sm.isRunning {
		return
	}

	sm.isRunning = false
	close(sm.shutdown)
	sm.cancel()

	// Wait for workers to finish
	sm.workerWg.Wait()

	// Clear queues
	sm.loadQueue.Clear()
	sm.evictQueue.Clear()

	log.Printf("StreamManager stopped")
}

// IsRunning returns whether background processing is active
func (sm *StreamManager) IsRunning() bool {
	sm.runningMutex.RLock()
	defer sm.runningMutex.RUnlock()
	return sm.isRunning
}

// UpdateCameraPosition updates the camera position (non-blocking)
func (sm *StreamManager) UpdateCameraPosition(pos lin.Vec3) {
	newCameraPos := lin.Vec2{pos[0], pos[2]}

	// Non-blocking send to channel
	select {
	case sm.cameraUpdates <- newCameraPos:
	default:
		// Channel full, skip this update (debouncing effect)
	}
}

// coordinatorLoop is the main coordination loop
func (sm *StreamManager) coordinatorLoop() {
	ticker := time.NewTicker(COORDINATOR_TICK)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-sm.shutdown:
			return
		case newPos := <-sm.cameraUpdates:
			sm.handleCameraUpdate(newPos)
		case <-ticker.C:
			sm.periodicUpdate()
		}
	}
}

// handleCameraUpdate processes camera position changes
func (sm *StreamManager) handleCameraUpdate(newPos lin.Vec2) {
	now := time.Now()

	// Debounce rapid camera updates
	if now.Sub(sm.lastCameraUpdate) < CAMERA_UPDATE_DEBOUNCE {
		return
	}

	sm.lastCameraPos = newPos
	sm.lastCameraUpdate = now

	// Update chunk selection
	sm.updateChunkSelection()
}

// periodicUpdate runs maintenance tasks
func (sm *StreamManager) periodicUpdate() {
	// Queue missing chunks for loading
	sm.queueMissingChunks()

	// Check for chunks that need eviction (when memory is getting full)
	sm.checkMemoryPressure()
}

// updateChunkSelection updates which chunks should be loaded
func (sm *StreamManager) updateChunkSelection() {
	// Create frustum for current camera position
	near := float32(10.0)
	frustum := spatial.NewFrustumFromCamera(
		sm.lastCameraPos,
		sm.cfg.CameraStartDir[1],
		sm.cfg.Fov,
		near,
		sm.cfg.RenderDistance,
	)

	// Update ChunkSelector
	sm.chunkSelector.UpdateSelection(sm.lastCameraPos, frustum, sm.cfg.RenderDistance)
}

// queueMissingChunks adds missing chunks to the load queue
func (sm *StreamManager) queueMissingChunks() {
	missingChunks := sm.chunkSelector.GetMissingChunks()

	for _, spec := range missingChunks {
		// Check if already queued or loading
		currentState := sm.stateManager.GetChunkState(spec.GridCoord)
		if currentState != streaming.StateUnloaded {
			continue
		}

		// Queue the chunk
		err := sm.stateManager.SetChunkState(spec.GridCoord, streaming.StateQueued, spec.Priority, spec.LODLevel)
		if err != nil {
			log.Printf("Failed to queue chunk %v: %v", spec.GridCoord, err)
			continue
		}

		// Determine map coordinate
		chunkMapCoord := coords.GetMapFromPosition(lin.Vec3{spec.WorldPos[0], 0, spec.WorldPos[1]})

		// Add to load queue
		job := LoadJob{
			Spec:     spec,
			MapCoord: chunkMapCoord,
			Priority: spec.Priority,
			QueuedAt: time.Now(),
		}

		sm.loadQueue.Enqueue(job)
	}
}

// checkMemoryPressure evicts chunks when memory usage is high
func (sm *StreamManager) checkMemoryPressure() {
	stats := sm.bucketAllocator.Stats()

	// If memory usage > 85%, start evicting distant chunks
	usagePercent := float32(stats.UsedSize) / float32(stats.TotalSize)
	if usagePercent > 0.85 {
		sm.queueDistantChunksForEviction()
	}
}

// queueDistantChunksForEviction adds distant ready chunks to eviction queue
func (sm *StreamManager) queueDistantChunksForEviction() {
	readyChunks := sm.stateManager.ListChunksByState(streaming.StateReady)

	// Find chunks to evict (furthest from camera first)
	for _, record := range readyChunks {
		// Calculate distance from current camera position
		chunkWorldPos := sm.chunkSelector.GridToWorld(record.GridCoord)
		dx := chunkWorldPos[0] - sm.lastCameraPos[0]
		dz := chunkWorldPos[1] - sm.lastCameraPos[1]
		distance := dx*dx + dz*dz // Use squared distance for efficiency

		// If chunk is far and not in frustum, queue for eviction
		if distance > sm.cfg.RenderDistance*sm.cfg.RenderDistance*0.5 {
			job := EvictJob{
				Coord:    record.GridCoord,
				Priority: record.Priority,
				QueuedAt: time.Now(),
			}
			sm.evictQueue.Enqueue(job)
		}
	}
}

// workerLoop is the main worker thread loop
func (sm *StreamManager) workerLoop(workerID int) {
	defer sm.workerWg.Done()

	log.Printf("Worker %d started", workerID)

	for {
		select {
		case <-sm.ctx.Done():
			log.Printf("Worker %d shutting down", workerID)
			return
		default:
			// Try to get work
			if sm.processNextJob(workerID) {
				continue // Got work, immediately check for more
			}

			// No work available, wait a bit
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// processNextJob processes the next available job
func (sm *StreamManager) processNextJob(workerID int) bool {
	// Prioritize load jobs over evict jobs
	if loadJob, hasLoad := sm.loadQueue.Dequeue(); hasLoad {
		sm.processLoadJob(workerID, loadJob)
		return true
	}

	if evictJob, hasEvict := sm.evictQueue.Dequeue(); hasEvict {
		sm.processEvictJob(workerID, evictJob)
		return true
	}

	return false
}

// processLoadJob handles loading a chunk
func (sm *StreamManager) processLoadJob(workerID int, job LoadJob) {
	// Verify chunk is still in queued state
	currentState := sm.stateManager.GetChunkState(job.Spec.GridCoord)
	if currentState != streaming.StateQueued {
		return // Chunk was cancelled or already processed
	}

	// Transition to loading
	err := sm.stateManager.SetChunkState(job.Spec.GridCoord, streaming.StateLoading, job.Priority, job.Spec.LODLevel)
	if err != nil {
		log.Printf("Worker %d: Failed to transition chunk %v to loading: %v", workerID, job.Spec.GridCoord, err)
		return
	}

	// Load the chunk
	err = sm.loadChunk(job.Spec, job.MapCoord)
	if err != nil {
		log.Printf("Worker %d: Failed to load chunk %v: %v", workerID, job.Spec.GridCoord, err)
		// Mark as failed
		sm.stateManager.SetChunkState(job.Spec.GridCoord, streaming.StateUnloaded, job.Priority, job.Spec.LODLevel)
		return
	}

	// Mark as available in ChunkSelector
	sm.chunkSelector.MarkChunkAvailable(job.Spec.GridCoord)
}

// processEvictJob handles evicting a chunk
func (sm *StreamManager) processEvictJob(workerID int, job EvictJob) {
	err := sm.EvictChunk(job.Coord)
	if err != nil {
		log.Printf("Worker %d: Failed to evict chunk %v: %v", workerID, job.Coord, err)
	}
}

// LoadSomeTestChunks is now deprecated - use Start() instead
func (sm *StreamManager) LoadSomeTestChunks() error {
	if !sm.IsRunning() {
		return fmt.Errorf("StreamManager not running - call Start() first")
	}

	// Initial camera position to trigger loading
	sm.UpdateCameraPosition(lin.Vec3{sm.lastCameraPos[0], 0, sm.lastCameraPos[1]})

	log.Printf("Background chunk loading initiated")
	return nil
}

// loadChunk handles the loading of a single chunk with state management
func (sm *StreamManager) loadChunk(spec spatial.ChunkSpec, mapCoord string) error {
	// Get map data
	mapData, err := sm.assetManager.LoadMapData(mapCoord)
	if err != nil {
		return fmt.Errorf("failed to get map data: %w", err)
	}

	// Generate chunk using existing logic from package
	chunk := &Chunk{
		X:        int(spec.WorldPos[0] - CHUNK_SIZE/2), // Convert center pos to corner
		Z:        int(spec.WorldPos[1] - CHUNK_SIZE/2),
		LODLevel: LODLevel(spec.LODLevel),
	}

	// Calculate geometry using existing functions
	CalculateChunkVertices(mapData.Heightmap, mapData.Watermap, chunk)
	CalculateChunkIndices(chunk)
	CalculateChunkNormals(chunk)

	// Allocate vertex buffer
	vertexId := fmt.Sprintf("chunk_%v_vertices", spec.GridCoord)
	vertexDataSize := vk.DeviceSize(len(chunk.Vertices) * VertexSize)
	success, vertexOffset := sm.bucketAllocator.AllocateChunk(vertexId, vertexDataSize, spec.LODLevel)
	if !success {
		return fmt.Errorf("failed to allocate vertex buffer")
	}

	// Allocate index buffer
	indexId := fmt.Sprintf("chunk_%v_indices", spec.GridCoord)
	indexDataSize := vk.DeviceSize(len(chunk.Indices) * IndexSize)
	success, indexOffset := sm.bucketAllocator.AllocateChunk(indexId, indexDataSize, spec.LODLevel)
	if !success {
		sm.bucketAllocator.FreeChunk(vertexId, spec.LODLevel)
		return fmt.Errorf("failed to allocate index buffer")
	}

	// Upload vertex data
	vertexData := make([]byte, len(chunk.Vertices)*VertexSize)
	for i, vertex := range chunk.Vertices {
		copy(vertexData[i*VertexSize:], vertex.Data())
	}

	if err := sm.bucketAllocator.Upload(vertexId, spec.LODLevel, vertexData); err != nil {
		sm.bucketAllocator.FreeChunk(vertexId, spec.LODLevel)
		sm.bucketAllocator.FreeChunk(indexId, spec.LODLevel)
		return fmt.Errorf("failed to upload vertex data: %w", err)
	}

	// Upload index data
	indexData := make([]byte, len(chunk.Indices)*IndexSize)
	for i, index := range chunk.Indices {
		binary.LittleEndian.PutUint32(indexData[i*IndexSize:], uint32(index))
	}

	if err := sm.bucketAllocator.Upload(indexId, spec.LODLevel, indexData); err != nil {
		sm.bucketAllocator.FreeChunk(vertexId, spec.LODLevel)
		sm.bucketAllocator.FreeChunk(indexId, spec.LODLevel)
		return fmt.Errorf("failed to upload index data: %w", err)
	}

	// Create renderable chunk
	renderableChunk := &RenderableChunk{
		GridCoord:      spec.GridCoord,
		LODLevel:       spec.LODLevel,
		VertexBufferId: vertexId,
		IndexBufferId:  indexId,
		VertexOffset:   vertexOffset,
		IndexOffset:    indexOffset,
		IndexCount:     len(chunk.Indices),
	}

	// Store renderable chunk with thread safety
	sm.renderMutex.Lock()
	sm.renderableChunks[spec.GridCoord] = renderableChunk
	sm.renderMutex.Unlock()

	// Transition to ready state
	err = sm.stateManager.SetChunkState(spec.GridCoord, streaming.StateReady, spec.Priority, spec.LODLevel)
	if err != nil {
		// Clean up on state transition failure
		sm.renderMutex.Lock()
		delete(sm.renderableChunks, spec.GridCoord)
		sm.renderMutex.Unlock()
		sm.bucketAllocator.FreeChunk(vertexId, spec.LODLevel)
		sm.bucketAllocator.FreeChunk(indexId, spec.LODLevel)
		return fmt.Errorf("failed to transition to ready state: %w", err)
	}

	// Touch the chunk to update its last used time
	sm.stateManager.TouchChunk(spec.GridCoord)

	return nil
}

// GetRenderableChunks returns all currently loaded chunks ready for rendering (thread-safe)
func (sm *StreamManager) GetRenderableChunks() []*RenderableChunk {
	sm.renderMutex.RLock()
	defer sm.renderMutex.RUnlock()

	// Only return chunks that are in Ready state
	readyChunks := sm.stateManager.ListChunksByState(streaming.StateReady)

	chunks := make([]*RenderableChunk, 0, len(readyChunks))
	for _, record := range readyChunks {
		if renderableChunk, exists := sm.renderableChunks[record.GridCoord]; exists {
			// Touch the chunk since it's being rendered
			sm.stateManager.TouchChunk(record.GridCoord)
			chunks = append(chunks, renderableChunk)
		}
	}

	return chunks
}

// RenderChunks renders all loaded chunks (temporary - will be moved to Terrain)
func (sm *StreamManager) RenderChunks(cmdBuffer vk.CommandBuffer) {
	chunks := sm.GetRenderableChunks()
	if len(chunks) == 0 {
		return
	}

	for _, chunk := range chunks {
		// Get buffer information from BucketAllocator
		vertexInfo, err := sm.bucketAllocator.GetAllocationInfo(chunk.VertexBufferId, chunk.LODLevel)
		if err != nil {
			log.Printf("Failed to get vertex allocation info: %v", err)
			continue
		}

		indexInfo, err := sm.bucketAllocator.GetAllocationInfo(chunk.IndexBufferId, chunk.LODLevel)
		if err != nil {
			log.Printf("Failed to get index allocation info: %v", err)
			continue
		}

		// Bind vertex buffer
		vk.CmdBindVertexBuffers(cmdBuffer, 0, 1,
			[]vk.Buffer{vertexInfo.Buffer},
			[]vk.DeviceSize{vertexInfo.Offset})

		// Bind index buffer
		vk.CmdBindIndexBuffer(cmdBuffer, indexInfo.Buffer,
			indexInfo.Offset, vk.IndexTypeUint32)

		// Draw
		vk.CmdDrawIndexed(cmdBuffer, uint32(chunk.IndexCount), 1, 0, 0, 0)
	}
}

// EvictChunk removes a chunk from memory and transitions it to unloaded
func (sm *StreamManager) EvictChunk(coord spatial.GridCoord) error {
	// Get current state
	currentState := sm.stateManager.GetChunkState(coord)
	if currentState != streaming.StateReady {
		return fmt.Errorf("can only evict ready chunks, chunk %v is in state %s", coord, currentState)
	}

	// Transition to evicting state
	record := sm.stateManager.GetChunkRecord(coord)
	if record == nil {
		return fmt.Errorf("chunk record not found for %v", coord)
	}

	err := sm.stateManager.SetChunkState(coord, streaming.StateEvicting, record.Priority, record.LODLevel)
	if err != nil {
		return fmt.Errorf("failed to transition to evicting state: %w", err)
	}

	// Clean up renderable chunk with thread safety
	sm.renderMutex.Lock()
	renderableChunk, exists := sm.renderableChunks[coord]
	if exists {
		delete(sm.renderableChunks, coord)
	}
	sm.renderMutex.Unlock()

	if exists {
		// Free memory allocations
		sm.bucketAllocator.FreeChunk(renderableChunk.VertexBufferId, renderableChunk.LODLevel)
		sm.bucketAllocator.FreeChunk(renderableChunk.IndexBufferId, renderableChunk.LODLevel)
	}

	// Mark as unavailable in ChunkSelector
	sm.chunkSelector.MarkChunkUnavailable(coord)

	// Transition to unloaded and remove from tracking
	err = sm.stateManager.SetChunkState(coord, streaming.StateUnloaded, record.Priority, record.LODLevel)
	if err != nil {
		return fmt.Errorf("failed to transition to unloaded state: %w", err)
	}

	err = sm.stateManager.RemoveChunk(coord)
	if err != nil {
		return fmt.Errorf("failed to remove chunk from tracking: %w", err)
	}

	return nil
}

// Stats returns information about the StreamManager state
func (sm *StreamManager) Stats() string {
	chunkStats := sm.chunkSelector.Stats()
	stateStats := sm.stateManager.GetStats()

	stateStatsStr := ""
	for state, count := range stateStats {
		if stateStatsStr != "" {
			stateStatsStr += ", "
		}
		stateStatsStr += fmt.Sprintf("%s:%d", state, count)
	}

	loadQueueSize := sm.loadQueue.Size()
	evictQueueSize := sm.evictQueue.Size()

	sm.renderMutex.RLock()
	renderableCount := len(sm.renderableChunks)
	sm.renderMutex.RUnlock()

	return fmt.Sprintf("StreamManager: %d renderable, %d load jobs, %d evict jobs, ChunkSelector: %d desired, %d missing, States: [%s]",
		renderableCount, loadQueueSize, evictQueueSize, chunkStats.DesiredChunks, chunkStats.MissingChunks, stateStatsStr)
}

// GetDetailedStateStats returns detailed statistics about chunk states
func (sm *StreamManager) GetDetailedStateStats() string {
	stats := sm.stateManager.GetStats()
	allChunks := sm.stateManager.GetAllChunks()

	result := fmt.Sprintf("Chunk State Statistics (Total: %d chunks):\n", sm.stateManager.Count())
	result += "=========================================\n"

	for state := streaming.StateUnloaded; state <= streaming.StateEvicting; state++ {
		count := stats[state]
		result += fmt.Sprintf("  %s: %d chunks\n", state, count)
	}

	// Add worker information
	result += fmt.Sprintf("\nWorker Status:\n")
	result += fmt.Sprintf("  Running: %v\n", sm.IsRunning())
	result += fmt.Sprintf("  Workers: %d\n", sm.numWorkers)
	result += fmt.Sprintf("  Load Queue: %d jobs\n", sm.loadQueue.Size())
	result += fmt.Sprintf("  Evict Queue: %d jobs\n", sm.evictQueue.Size())

	// Show some example chunks in each state
	stateExamples := make(map[streaming.ChunkState][]*streaming.ChunkRecord)
	for _, chunk := range allChunks {
		stateExamples[chunk.State] = append(stateExamples[chunk.State], chunk)
	}

	result += "\nSample chunks by state:\n"
	for state, chunks := range stateExamples {
		if len(chunks) > 0 {
			result += fmt.Sprintf("  %s: ", state)
			maxShow := 3
			for i, chunk := range chunks {
				if i >= maxShow {
					result += fmt.Sprintf("... (%d more)", len(chunks)-maxShow)
					break
				}
				if i > 0 {
					result += ", "
				}
				result += fmt.Sprintf("%v(LOD:%d,P:%.1f)", chunk.GridCoord, chunk.LODLevel, chunk.Priority)
			}
			result += "\n"
		}
	}

	return result
}

// Destroy cleans up resources
func (sm *StreamManager) Destroy() {
	// Stop background processing first
	sm.Stop()

	if sm.cfg.DebugMode {
		log.Printf("StreamManager stats: %s", sm.Stats())
		log.Printf("Memory usage: %s", sm.bucketAllocator.GetDetailedStats())
		log.Printf("Chunk Selector: %s", sm.chunkSelector.GetDetailedStats())
		log.Printf("Visual: %s", sm.chunkSelector.VisualizeSelection())
		log.Printf("AssetManager: %s", sm.assetManager.Stats())
		log.Printf("Detailed State Stats:\n%s", sm.GetDetailedStateStats())
	}

	// Clean up all ready chunks before destroying allocator
	readyChunks := sm.stateManager.ListChunksByState(streaming.StateReady)
	for _, record := range readyChunks {
		err := sm.EvictChunk(record.GridCoord)
		if err != nil {
			log.Printf("Failed to evict chunk %v during cleanup: %v", record.GridCoord, err)
		}
	}

	// Clear state manager
	sm.stateManager.Clear()

	sm.bucketAllocator.Destroy()
}
