package rendering

// import (
// 	"encoding/binary"
// 	"fmt"
// 	"log"
// 	"math"
// 	"time"

// 	"github.com/cs-wwu/srg-synvis/core/memory"
// 	"github.com/cs-wwu/srg-synvis/core/spatial"
// 	"github.com/cs-wwu/srg-synvis/internal/assets"
// 	"github.com/cs-wwu/srg-synvis/internal/coords"

// 	vk "github.com/vulkan-go/vulkan"
// 	lin "github.com/xlab/linmath"
// )

// // This file should be used for chunk management, chunk rendering, etc.

// const (
// 	MAP_FOLDER          = "assets/maps/"
// 	WATERMAP_EXTENSION  = ".swb.png"
// 	HEIGHTMAP_EXTENSION = ".hgt.png"
// 	TEXTURE_EXTENSION   = ".osm.png"

// 	MAX_IMAGE_MAPS = 1

// 	TICKER_SPEED = time.Second / 60 // x times per second

// 	MAP_LENGTH      = 3600
// 	CHUNKS_PER_SIDE = 20
// 	CHUNK_SIZE      = MAP_LENGTH / CHUNKS_PER_SIDE // 180 allows for perfect multiples of 1 through 4
// )

// var firstRun = true
// var LODBucketPercentages = []float32{0.3, 0.25, 0.25, 0.2}

// // A single TerrainManager to manage load/unloading images and chunks
// type TerrainManager struct {
// 	device   *vk.Device
// 	memProps *vk.PhysicalDeviceMemoryProperties

// 	lastPosition  lin.Vec2
// 	lastUpdatePos lin.Vec2

// 	chunks        map[string]*Chunk
// 	chunkRegistry map[string]*spatial.ChunkInfo // chunk IDs to spatial

// 	bucketAllocator *memory.BucketAllocator
// 	terrainGrid     *spatial.TerrainGrid
// 	assetManager    assets.AssetManager

// 	debugMode bool
// 	shutdown  chan struct{}
// }

// func (tm *TerrainManager) UpdateLastPosition(pos lin.Vec3) {
// 	tm.lastPosition = lin.Vec2{pos[0], pos[2]}
// }

// func calculateLODChunkSizes() []vk.DeviceSize {
// 	numLODLevels := len(LODBucketPercentages) // Should correspond to LOD_0 to LOD_3
// 	lodChunkSizes := make([]vk.DeviceSize, numLODLevels)

// 	for lod := range numLODLevels {
// 		stride := 1
// 		if lod > 0 {
// 			stride = 1 << uint(lod)
// 		}

// 		vps := CHUNK_SIZE / stride // verticesPerSide

// 		// Calculate number of vertices based on chunk.go's calculateChunkVertices
// 		numVertices := vps * vps
// 		currentVertexDataSize := vk.DeviceSize(numVertices) * vk.DeviceSize(VertexSize)

// 		// Calculate number of indices based on chunk.go's calculateChunkIndices logic
// 		numIndices := 0
// 		if vps > 1 { // Chunks with 1 vertex per side (or less) have no indices
// 			numStrips := vps - 1
// 			numIndices = numStrips * vps * 2 // Main strip indices
// 			if numStrips > 1 {               // If more than one strip, add degenerate triangles
// 				numIndices += (numStrips - 1) * 2
// 			}
// 		}
// 		currentIndexDataSize := vk.DeviceSize(numIndices) * vk.DeviceSize(IndexSize)

// 		lodChunkSizes[lod] = currentVertexDataSize + currentIndexDataSize
// 	}
// 	return lodChunkSizes
// }

// func NewTerrainManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties, debugMode bool) *TerrainManager {
// 	lodChunkSizes := calculateLODChunkSizes()
// 	fmt.Printf("LOD Chunk Sizes: ")
// 	for _, size := range lodChunkSizes {
// 		fmt.Printf("%.1fKB ", float32((size)/1024))
// 	}
// 	fmt.Println()
// 	fmt.Printf("LOD Bucket Percentages: %v\n", LODBucketPercentages)
// 	bucketAllocator := memory.NewBucketAllocator(device, memProps, LODBucketPercentages, lodChunkSizes)

// 	terrainGrid := spatial.NewTerrainGrid(CHUNK_SIZE, nil)

// 	assetManager := assets.NewAssetManager(MAP_FOLDER, 9)

// 	tm := &TerrainManager{
// 		device:          device,
// 		memProps:        memProps,
// 		bucketAllocator: bucketAllocator,
// 		terrainGrid:     terrainGrid,
// 		assetManager:    assetManager,
// 		lastPosition:    lin.Vec2{0.0, 0.0},
// 		chunks:          make(map[string]*Chunk),
// 		chunkRegistry:   make(map[string]*spatial.ChunkInfo),
// 		debugMode:       debugMode,
// 		shutdown:        make(chan struct{}),
// 	}

// 	log.Printf("TerrainManager intialized, starting update loop.")
// 	go tm.updateLoop()
// 	return tm
// }

// // Clears all chunks in the terrain manager (chunks in buffers are left)
// func (tm *TerrainManager) ClearChunks() {
// 	for key, chunk := range tm.chunks {
// 		if chunk.allocated {
// 			// Free bucket allocations
// 			tm.bucketAllocator.FreeChunk(chunk.vertexBufferId, int(chunk.LODLevel))
// 			tm.bucketAllocator.FreeChunk(chunk.indexBufferId, int(chunk.LODLevel))
// 		}
// 		delete(tm.chunks, key)

// 		// Remove from terrain grid if it exists in registry
// 		if spatialChunk, exists := tm.chunkRegistry[key]; exists {
// 			tm.terrainGrid.UnregisterChunk(spatialChunk.ID)
// 			delete(tm.chunkRegistry, key)
// 		}
// 	}

// 	// Clear the terrain grid
// 	tm.terrainGrid.Clear()
// }

// func (tm *TerrainManager) GenerateTestLine() {
// 	// Starting position based on the camera location (2D)
// 	startX := int(tm.lastPosition[0])
// 	startZ := int(tm.lastPosition[1])

// 	// Clear any existing chunks
// 	tm.ClearChunks()

// 	// Load heightmaps/watermaps first
// 	baseMap := coords.WorldToMap(startX, startZ)
// 	if _, err := tm.assetManager.LoadMapData(baseMap); err != nil {
// 		log.Printf("Failed to load map data for %s: %v", baseMap, err)
// 		return
// 	}

// 	// Step 1: Register chunks in spatial grid (no GPU data yet)
// 	log.Printf("Registering %d chunks in spatial grid...", 200)
// 	for i := range 200 {
// 		chunkX := startX
// 		chunkZ := startZ + (i * CHUNK_SIZE)
// 		worldPos := lin.Vec2{float32(chunkX), float32(chunkZ)}

// 		// Register with placeholder LOD - spatial system will calculate real LOD
// 		spatialChunk := tm.terrainGrid.RegisterChunk(worldPos, 0)
// 		spatialChunk.Status = spatial.ChunkUnloaded // Start as unloaded

// 		chunkKey := fmt.Sprintf("test_chunk_%d", i)
// 		tm.chunkRegistry[chunkKey] = spatialChunk
// 	}

// 	// Step 2: Update visibility - this calculates distances and LODs
// 	if tm.debugMode {
// 		log.Printf("Updating spatial grid visibility...")
// 	}
// 	tm.updateVisibility()

// 	// Step 3: Load GPU data for visible chunks using their calculated LODs
// 	if tm.debugMode {
// 		log.Printf("Loading GPU data for visible chunks...")
// 	}
// 	tm.loadVisibleChunks(baseMap)

// 	if tm.debugMode {
// 		log.Printf("Finished terrain loading process")
// 		log.Printf("Spatial grid: %d total, %d visible chunks",
// 			tm.terrainGrid.Stats().TotalChunks,
// 			len(tm.terrainGrid.GetVisibleChunks()))
// 	}
// }

// // New method: Load GPU data only for visible chunks
// func (tm *TerrainManager) loadVisibleChunks(baseMap string) {
// 	visibleChunks := tm.terrainGrid.GetVisibleChunks()

// 	for _, spatialChunk := range visibleChunks {
// 		// Find the corresponding chunk key
// 		var chunkKey string
// 		for key, registered := range tm.chunkRegistry {
// 			if registered == spatialChunk {
// 				chunkKey = key
// 				break
// 			}
// 		}

// 		if chunkKey == "" {
// 			continue // Skip if we can't find the key
// 		}

// 		// Extract world coordinates from spatial chunk
// 		worldPos := spatialChunk.WorldPos
// 		chunkX := int(worldPos[0])
// 		chunkZ := int(worldPos[1])

// 		// Use the LOD calculated by spatial system
// 		calculatedLOD := spatialChunk.LODLevel

// 		// Load GPU chunk using spatial system's LOD decision
// 		if tm.loadGPUChunk(chunkX, chunkZ, LODLevel(calculatedLOD), chunkKey, baseMap) {
// 			// Mark as loaded in spatial system
// 			spatialChunk.Status = spatial.ChunkLoaded
// 		}
// 	}
// }

// // Loads GPU chunk data directly
// func (tm *TerrainManager) loadGPUChunk(chunkX, chunkZ int, lodLevel LODLevel, chunkKey, mapCoords string) bool {
// 	mapData, err := tm.assetManager.LoadMapData(mapCoords)
// 	if err != nil {
// 		log.Printf("Failed to get map data for %s: %v", mapCoords, err)
// 		return false
// 	}

// 	// Create GPU chunk
// 	chunk := &Chunk{
// 		X:        chunkX,
// 		Z:        chunkZ,
// 		LODLevel: lodLevel,
// 	}

// 	// Calculate geometry
// 	calculateChunkVertices(mapData.Heightmap, mapData.Watermap, chunk)
// 	calculateChunkIndices(chunk)
// 	calculateChunkNormals(chunk)

// 	// Allocate buffers using spatial system's LOD
// 	vertexId := fmt.Sprintf("%s_vertices", chunkKey)
// 	vertexDataSize := vk.DeviceSize(len(chunk.Vertices) * VertexSize)
// 	success, vertexOffset := tm.bucketAllocator.AllocateChunk(vertexId, vertexDataSize, int(lodLevel))
// 	if !success {
// 		log.Printf("Failed to allocate vertex buffer for chunk %s", chunkKey)
// 		return false
// 	}

// 	indexId := fmt.Sprintf("%s_indices", chunkKey)
// 	indexDataSize := vk.DeviceSize(len(chunk.Indices) * IndexSize)
// 	success, indexOffset := tm.bucketAllocator.AllocateChunk(indexId, indexDataSize, int(lodLevel))
// 	if !success {
// 		tm.bucketAllocator.FreeChunk(vertexId, int(lodLevel))
// 		log.Printf("Failed to allocate index buffer for chunk %s", chunkKey)
// 		return false
// 	}

// 	// Upload data (same as before)
// 	vertexData := make([]byte, len(chunk.Vertices)*VertexSize)
// 	for i, vertex := range chunk.Vertices {
// 		copy(vertexData[i*VertexSize:], vertex.Data())
// 	}

// 	indexData := make([]byte, len(chunk.Indices)*IndexSize)
// 	for i, index := range chunk.Indices {
// 		binary.LittleEndian.PutUint32(indexData[i*IndexSize:], uint32(index))
// 	}

// 	if err := tm.bucketAllocator.Upload(vertexId, int(lodLevel), vertexData); err != nil {
// 		tm.bucketAllocator.FreeChunk(vertexId, int(lodLevel))
// 		tm.bucketAllocator.FreeChunk(indexId, int(lodLevel))
// 		log.Printf("Failed to upload vertex data: %v", err)
// 		return false
// 	}

// 	if err := tm.bucketAllocator.Upload(indexId, int(lodLevel), indexData); err != nil {
// 		tm.bucketAllocator.FreeChunk(vertexId, int(lodLevel))
// 		tm.bucketAllocator.FreeChunk(indexId, int(lodLevel))
// 		log.Printf("Failed to upload index data: %v", err)
// 		return false
// 	}

// 	// Store GPU chunk data
// 	chunk.allocated = true
// 	chunk.vertexOffset = vertexOffset
// 	chunk.indexOffset = indexOffset
// 	chunk.vertexBufferId = vertexId
// 	chunk.indexBufferId = indexId

// 	tm.chunks[chunkKey] = chunk

// 	// Link GPU chunk to spatial chunk
// 	if spatialChunk, exists := tm.chunkRegistry[chunkKey]; exists {
// 		spatialChunk.UserData = chunk
// 	}

// 	return true
// }

// func (tm *TerrainManager) checkForUpdates() {
// 	if firstRun {
// 		// Generate the test line of chunks
// 		tm.GenerateTestLine()

// 		// Capture last position for update tracking
// 		tm.lastUpdatePos = tm.lastPosition
// 		firstRun = false

// 		if tm.debugMode {
// 			// Log memory usage after loading all chunks
// 			log.Printf("Memory stats after loading all test chunks: %s",
// 				tm.bucketAllocator.GetDetailedStats())

// 			log.Printf("Terrain grid stats after loading all test chunks: %s",
// 				tm.terrainGrid.GetDetailedStats())

// 			log.Printf("Terrain grid visualization: %s", tm.terrainGrid.VisualizeGrid())

// 			log.Printf("Asset manager stats: %s", tm.assetManager.Stats())
// 		}
// 	}
// }

// func (tm *TerrainManager) updateVisibility() {
// 	// Create a simple frustum for testing (looking north with 90-degree FOV)
// 	direction := float32(math.Pi / 2) // Looking north (positive Z)
// 	fov := float32(math.Pi / 2)       // 90 degrees
// 	near := float32(10.0)
// 	far := float32(10000.0)

// 	frustum := spatial.NewFrustumFromCamera(tm.lastPosition, direction, fov, near, far)

// 	// Update visibility in terrain grid
// 	tm.terrainGrid.UpdateVisibility(tm.lastPosition, frustum)

// 	visibleChunks := tm.terrainGrid.GetVisibleChunks()

// 	if tm.debugMode {
// 		log.Printf("Terrain grid visibility update: %d/%d chunks visible",
// 			len(visibleChunks), tm.terrainGrid.Stats().TotalChunks)
// 	}
// }

// func (tm *TerrainManager) updateLoop() {
// 	log.Println("Starting update loop.")
// 	ticker := time.NewTicker(TICKER_SPEED)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-tm.shutdown:
// 			return
// 		case <-ticker.C:
// 			tm.checkForUpdates()
// 		}
// 	}
// }

// func (tm *TerrainManager) Destroy() {
// 	// Signal shutdown
// 	close(tm.shutdown)
// 	tm.bucketAllocator.Destroy()
// }

// // RenderAllocatedChunks draws chunks directly from bucket allocator
// func (tm *TerrainManager) RenderAllocatedChunks(cmdBuffer vk.CommandBuffer) {
// 	// Get visible chunks from terrain grid
// 	visibleChunks := tm.terrainGrid.GetVisibleChunksSorted()

// 	if len(visibleChunks) == 0 {
// 		return
// 	}

// 	for _, spatialChunk := range visibleChunks {
// 		// Get GPU chunk from UserData
// 		chunk, ok := spatialChunk.UserData.(*Chunk)
// 		if !ok || chunk == nil || !chunk.allocated {
// 			continue
// 		}

// 		// Get vertex buffer information
// 		vertexInfo, err := tm.bucketAllocator.GetAllocationInfo(
// 			chunk.vertexBufferId, int(chunk.lodLevel))
// 		if err != nil {
// 			log.Printf("Failed to get vertex allocation info: %v", err)
// 			continue
// 		}

// 		// Get index buffer information
// 		indexInfo, err := tm.bucketAllocator.GetAllocationInfo(
// 			chunk.indexBufferId, int(chunk.lodLevel))
// 		if err != nil {
// 			log.Printf("Failed to get index allocation info: %v", err)
// 			continue
// 		}

// 		// Bind vertex buffer
// 		vk.CmdBindVertexBuffers(cmdBuffer, 0, 1,
// 			[]vk.Buffer{vertexInfo.Buffer},
// 			[]vk.DeviceSize{vertexInfo.Offset})

// 		// Bind index buffer
// 		vk.CmdBindIndexBuffer(cmdBuffer, indexInfo.Buffer,
// 			indexInfo.Offset, vk.IndexTypeUint32)

// 		// Draw indexed
// 		vk.CmdDrawIndexed(cmdBuffer, uint32(len(chunk.indices)), 1, 0, 0, 0)
// 	}
// }
