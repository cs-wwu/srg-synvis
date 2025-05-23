package rendering

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cs-wwu/srg-synvis/core/memory"
	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/assets"
	"github.com/cs-wwu/srg-synvis/internal/coords"
	"github.com/cs-wwu/srg-synvis/internal/models"

	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

const (
	CHUNK_SIZE = 120 // Same as before for compatibility
	MAP_FOLDER = "assets/maps/"
)

// RenderableChunk represents a chunk ready for GPU
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

	// Vulkan resources
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	// State tracking
	activeChunks  map[spatial.GridCoord]*RenderableChunk
	lastCameraPos lin.Vec2

	cfg models.AppConfig
}

// NewStreamManager creates a new stream manager with all subsystems
func NewStreamManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties, cfg models.AppConfig) *StreamManager {
	// Initialize ChunkSelector (replaces old TerrainGrid)
	chunkSelector := spatial.NewChunkSelector(CHUNK_SIZE, nil)

	// Initialize BucketAllocator (same as before)
	lodChunkSizes := calculateLODChunkSizes()
	bucketAllocator := memory.NewBucketAllocator(device, memProps,
		[]float32{0.3, 0.25, 0.25, 0.2}, lodChunkSizes)

	// Initialize AssetManager (same as before)
	assetManager := assets.NewAssetManager(MAP_FOLDER, 27)

	sm := &StreamManager{
		chunkSelector:   chunkSelector,
		bucketAllocator: bucketAllocator,
		assetManager:    assetManager,
		device:          device,
		memProps:        memProps,
		cfg:             cfg,
		activeChunks:    make(map[spatial.GridCoord]*RenderableChunk),
	}

	log.Printf("StreamManager initialized with ChunkSelector")
	return sm
}

// UpdateCameraPosition updates the camera position for spatial queries
func (sm *StreamManager) UpdateCameraPosition(pos lin.Vec3) {
	sm.lastCameraPos = lin.Vec2{pos[0], pos[2]}
}

// LoadSomeTestChunks loads a few chunks for testing (replaces GenerateTestLine)
func (sm *StreamManager) LoadSomeTestChunks() error {
	// Create a simple frustum for testing
	near := float32(10.0)
	frustum := spatial.NewFrustumFromCamera(sm.lastCameraPos, sm.cfg.CameraStartDir[1], sm.cfg.Fov, near, sm.cfg.RenderDistance)

	// Update ChunkSelector with current camera state
	sm.chunkSelector.UpdateSelection(sm.lastCameraPos, frustum, sm.cfg.RenderDistance)

	// Get desired chunks
	desiredChunks := sm.chunkSelector.GetDesiredChunks()
	log.Printf("ChunkSelector found %d desired chunks", len(desiredChunks))

	// Load first few chunks for testing (limit to avoid overwhelming)
	maxChunks := 4000
	if len(desiredChunks) > maxChunks {
		desiredChunks = desiredChunks[:maxChunks]
	}

	if len(desiredChunks) == 0 {
		return fmt.Errorf("no chunks to load")
	}

	// Track which maps we've already loaded to avoid reloading
	loadedMaps := make(map[string]bool)

	// Process each desired chunk
	for _, spec := range desiredChunks {
		// Determine the correct map coordinate for this specific chunk
		chunkMapCoord := coords.GetMapFromPosition(lin.Vec3{spec.WorldPos[0], 0, spec.WorldPos[1]})

		// Load map data for this coordinate if we haven't already
		if !loadedMaps[chunkMapCoord] {
			log.Printf("Loading map data for: %s", chunkMapCoord)
			_, err := sm.assetManager.LoadMapData(chunkMapCoord)
			if err != nil {
				log.Printf("Failed to load map data for %s: %v", chunkMapCoord, err)
				continue // Skip this chunk if we can't load its map data
			}
			loadedMaps[chunkMapCoord] = true
		}

		// Load the chunk with its correct map coordinate
		err := sm.loadChunk(spec, chunkMapCoord)
		if err != nil {
			log.Printf("Failed to load chunk %v: %v", spec.GridCoord, err)
			continue
		}

		// Mark as available in ChunkSelector
		sm.chunkSelector.MarkChunkAvailable(spec.GridCoord)
	}

	log.Printf("Loaded map data for %d different map coordinates", len(loadedMaps))
	return nil
}

// loadChunk handles the loading of a single chunk
func (sm *StreamManager) loadChunk(spec spatial.ChunkSpec, mapCoord string) error {
	// Get map data
	mapData, err := sm.assetManager.LoadMapData(mapCoord)
	if err != nil {
		return fmt.Errorf("failed to get map data: %w", err)
	}

	// Generate chunk using existing logic from  package
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

	sm.activeChunks[spec.GridCoord] = renderableChunk
	return nil
}

// GetRenderableChunks returns all currently loaded chunks ready for
func (sm *StreamManager) GetRenderableChunks() []*RenderableChunk {
	chunks := make([]*RenderableChunk, 0, len(sm.activeChunks))
	for _, chunk := range sm.activeChunks {
		chunks = append(chunks, chunk)
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

// Stats returns information about the StreamManager state
func (sm *StreamManager) Stats() string {
	chunkStats := sm.chunkSelector.Stats()
	return fmt.Sprintf("StreamManager: %d active chunks, ChunkSelector: %d desired, %d missing",
		len(sm.activeChunks), chunkStats.DesiredChunks, chunkStats.MissingChunks)
}

// Destroy cleans up resources
func (sm *StreamManager) Destroy() {
	// Print some stats before destruction
	if sm.cfg.DebugMode {
		log.Printf("StreamManager stats: %s", sm.Stats())
		log.Printf("Memory usage: %s", sm.bucketAllocator.GetDetailedStats())
		log.Printf("Chunk Selector: %s", sm.chunkSelector.GetDetailedStats())
		log.Printf("Visual: %s", sm.chunkSelector.VisualizeSelection())
		log.Printf("AssetManager: %s", sm.assetManager.Stats())
	}

	sm.bucketAllocator.Destroy()
}

// calculateLODChunkSizes calculates memory requirements for each LOD level
func calculateLODChunkSizes() []vk.DeviceSize {
	numLODLevels := 4 // LOD 0-3
	lodChunkSizes := make([]vk.DeviceSize, numLODLevels)

	for lod := range numLODLevels {
		stride := 1
		if lod > 0 {
			stride = 1 << uint(lod)
		}

		vps := CHUNK_SIZE / stride
		numVertices := vps * vps
		vertexDataSize := vk.DeviceSize(numVertices) * vk.DeviceSize(VertexSize)

		// Calculate indices (same logic as existing chunk.go)
		numIndices := 0
		if vps > 1 {
			numStrips := vps - 1
			numIndices = numStrips * vps * 2
			if numStrips > 1 {
				numIndices += (numStrips - 1) * 2
			}
		}
		indexDataSize := vk.DeviceSize(numIndices) * vk.DeviceSize(IndexSize)

		lodChunkSizes[lod] = vertexDataSize + indexDataSize
	}
	return lodChunkSizes
}
