package renderer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"time"

	"github.com/cs-wwu/srg-synvis/core/memory"
	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/coords"

	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file should be used for chunk management, chunk rendering, etc.

const (
	WATERMAP_EXTENSION  = ".swb.png"
	HEIGHTMAP_EXTENSION = ".hgt.png"
	TEXTURE_EXTENSION   = ".osm.png"

	MAX_IMAGE_MAPS = 1

	TICKER_SPEED = time.Second / 60 // x times per second

	MAP_LENGTH      = 3600
	CHUNKS_PER_SIDE = 20
	CHUNK_SIZE      = MAP_LENGTH / CHUNKS_PER_SIDE // 180 allows for perfect multiples of 1 through 4
)

var firstRun = true
var LODBucketPercentages = []float32{0.3, 0.25, 0.25, 0.2}

// A single TerrainManager to manage load/unloading images and chunks
type TerrainManager struct {
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	lastPosition  lin.Vec2
	lastUpdatePos lin.Vec2

	heightmaps    map[string]*image.Gray16
	watermaps     map[string]*image.Gray
	chunks        map[string]*Chunk
	chunkRegistry map[string]*spatial.ChunkInfo // chunk IDs to spatial

	bucketAllocator *memory.BucketAllocator
	terrainGrid     *spatial.TerrainGrid

	debugMode bool
	shutdown  chan struct{}
}

func (tm *TerrainManager) UpdateLastPosition(pos lin.Vec3) {
	tm.lastPosition = lin.Vec2{pos[0], pos[2]}
}

func calculateLODChunkSizes() []vk.DeviceSize {
	numLODLevels := len(LODBucketPercentages) // Should correspond to LOD_0 to LOD_3
	lodChunkSizes := make([]vk.DeviceSize, numLODLevels)

	for lod := range numLODLevels {
		stride := 1
		if lod > 0 {
			stride = 1 << uint(lod)
		}

		vps := CHUNK_SIZE / stride // verticesPerSide

		// Calculate number of vertices based on chunk.go's calculateChunkVertices
		numVertices := vps * vps
		currentVertexDataSize := vk.DeviceSize(numVertices) * vk.DeviceSize(VertexSize)

		// Calculate number of indices based on chunk.go's calculateChunkIndices logic
		numIndices := 0
		if vps > 1 { // Chunks with 1 vertex per side (or less) have no indices
			numStrips := vps - 1
			numIndices = numStrips * vps * 2 // Main strip indices
			if numStrips > 1 {               // If more than one strip, add degenerate triangles
				numIndices += (numStrips - 1) * 2
			}
		}
		currentIndexDataSize := vk.DeviceSize(numIndices) * vk.DeviceSize(IndexSize)

		lodChunkSizes[lod] = currentVertexDataSize + currentIndexDataSize
	}
	return lodChunkSizes
}

func NewTerrainManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties, debugMode bool) *TerrainManager {
	lodChunkSizes := calculateLODChunkSizes()
	fmt.Printf("LOD Chunk Sizes: ")
	for _, size := range lodChunkSizes {
		fmt.Printf("%.1fKB ", float32((size)/1024))
	}
	fmt.Println()
	fmt.Printf("LOD Bucket Percentages: %v\n", LODBucketPercentages)
	bucketAllocator := memory.NewBucketAllocator(device, memProps, LODBucketPercentages, lodChunkSizes)

	terrainGrid := spatial.NewTerrainGrid(CHUNK_SIZE, nil)

	tm := &TerrainManager{
		device:          device,
		memProps:        memProps,
		bucketAllocator: bucketAllocator,
		terrainGrid:     terrainGrid,
		lastPosition:    lin.Vec2{0.0, 0.0},
		heightmaps:      make(map[string]*image.Gray16, MAX_IMAGE_MAPS),
		watermaps:       make(map[string]*image.Gray, MAX_IMAGE_MAPS),
		chunks:          make(map[string]*Chunk),
		chunkRegistry:   make(map[string]*spatial.ChunkInfo),
		debugMode:       debugMode,
		shutdown:        make(chan struct{}),
	}

	log.Printf("TerrainManager intialized, starting update loop.")
	go tm.updateLoop()
	return tm
}

// Clears all chunks in the terrain manager (chunks in buffers are left)
func (tm *TerrainManager) ClearChunks() {
	for key, chunk := range tm.chunks {
		if chunk.allocated {
			// Free bucket allocations
			tm.bucketAllocator.FreeChunk(chunk.vertexBufferId, int(chunk.lodLevel))
			tm.bucketAllocator.FreeChunk(chunk.indexBufferId, int(chunk.lodLevel))
		}
		delete(tm.chunks, key)

		// Remove from terrain grid if it exists in registry
		if spatialChunk, exists := tm.chunkRegistry[key]; exists {
			tm.terrainGrid.UnregisterChunk(spatialChunk.ID)
			delete(tm.chunkRegistry, key)
		}
	}

	// Clear the terrain grid
	tm.terrainGrid.Clear()
}

// LoadMap loads a heightmap and its associated watermap
func (tm *TerrainManager) LoadMap(coord string) error {
	// Skip heightmaps if already loaded
	if _, exists := tm.heightmaps[coord]; !exists {
		if err := tm.LoadHeightmap(coord); err != nil {
			return err
		}
		if err := tm.LoadWatermap(coord); err != nil {
			return err
		}
	}

	log.Printf("Loaded map: %s", coord)
	return nil
}

func (tm *TerrainManager) GenerateTestLine() {
	// Starting position based on the camera location (2D)
	startX := int(tm.lastPosition[0])
	startZ := int(tm.lastPosition[1])

	// Clear any existing chunks
	tm.ClearChunks()

	// Load heightmaps/watermaps first
	baseMap := coords.WorldToMap(startX, startZ)
	if _, exists := tm.heightmaps[baseMap]; !exists {
		log.Printf("Loading base map: %s", baseMap)
		if err := tm.LoadHeightmap(baseMap); err != nil {
			log.Printf("Error loading heightmap: %v", err)
			return
		}
		if err := tm.LoadWatermap(baseMap); err != nil {
			log.Printf("Error loading watermap: %v", err)
			return
		}
	}

	// Step 1: Register chunks in spatial grid (no GPU data yet)
	log.Printf("Registering %d chunks in spatial grid...", 200)
	for i := range 200 {
		chunkX := startX
		chunkZ := startZ + (i * CHUNK_SIZE)
		worldPos := lin.Vec2{float32(chunkX), float32(chunkZ)}

		// Register with placeholder LOD - spatial system will calculate real LOD
		spatialChunk := tm.terrainGrid.RegisterChunk(worldPos, 0)
		spatialChunk.Status = spatial.ChunkUnloaded // Start as unloaded

		chunkKey := fmt.Sprintf("test_chunk_%d", i)
		tm.chunkRegistry[chunkKey] = spatialChunk
	}

	// Step 2: Update visibility - this calculates distances and LODs
	if tm.debugMode {
		log.Printf("Updating spatial grid visibility...")
	}
	tm.updateVisibility()

	// Step 3: Load GPU data for visible chunks using their calculated LODs
	if tm.debugMode {
		log.Printf("Loading GPU data for visible chunks...")
	}
	tm.loadVisibleChunks(baseMap)

	if tm.debugMode {
		log.Printf("Finished terrain loading process")
		log.Printf("Spatial grid: %d total, %d visible chunks",
			tm.terrainGrid.Stats().TotalChunks,
			len(tm.terrainGrid.GetVisibleChunks()))
	}
}

// New method: Load GPU data only for visible chunks
func (tm *TerrainManager) loadVisibleChunks(baseMap string) {
	visibleChunks := tm.terrainGrid.GetVisibleChunks()

	for _, spatialChunk := range visibleChunks {
		// Find the corresponding chunk key
		var chunkKey string
		for key, registered := range tm.chunkRegistry {
			if registered == spatialChunk {
				chunkKey = key
				break
			}
		}

		if chunkKey == "" {
			continue // Skip if we can't find the key
		}

		// Extract world coordinates from spatial chunk
		worldPos := spatialChunk.WorldPos
		chunkX := int(worldPos[0])
		chunkZ := int(worldPos[1])

		// Use the LOD calculated by spatial system
		calculatedLOD := spatialChunk.LODLevel

		// Load GPU chunk using spatial system's LOD decision
		if tm.loadGPUChunk(chunkX, chunkZ, LODLevel(calculatedLOD), chunkKey, baseMap) {
			// Mark as loaded in spatial system
			spatialChunk.Status = spatial.ChunkLoaded
		}
	}
}

// Renamed and simplified: just loads GPU data
func (tm *TerrainManager) loadGPUChunk(chunkX, chunkZ int, lodLevel LODLevel, chunkKey, mapCoords string) bool {
	heightmap := tm.heightmaps[mapCoords]
	watermap := tm.watermaps[mapCoords]

	if heightmap == nil || watermap == nil {
		log.Printf("Warning: Map(s) not loaded for coords %s", mapCoords)
		return false
	}

	// Create GPU chunk
	chunk := &Chunk{
		X:        chunkX,
		Z:        chunkZ,
		lodLevel: lodLevel,
	}

	// Calculate geometry
	calculateChunkVertices(heightmap, watermap, chunk)
	calculateChunkIndices(chunk)
	calculateChunkNormals(chunk)

	// Allocate buffers using spatial system's LOD
	vertexId := fmt.Sprintf("%s_vertices", chunkKey)
	vertexDataSize := vk.DeviceSize(len(chunk.vertices) * VertexSize)
	success, vertexOffset := tm.bucketAllocator.AllocateChunk(vertexId, vertexDataSize, int(lodLevel))
	if !success {
		log.Printf("Failed to allocate vertex buffer for chunk %s", chunkKey)
		return false
	}

	indexId := fmt.Sprintf("%s_indices", chunkKey)
	indexDataSize := vk.DeviceSize(len(chunk.indices) * IndexSize)
	success, indexOffset := tm.bucketAllocator.AllocateChunk(indexId, indexDataSize, int(lodLevel))
	if !success {
		tm.bucketAllocator.FreeChunk(vertexId, int(lodLevel))
		log.Printf("Failed to allocate index buffer for chunk %s", chunkKey)
		return false
	}

	// Upload data (same as before)
	vertexData := make([]byte, len(chunk.vertices)*VertexSize)
	for i, vertex := range chunk.vertices {
		copy(vertexData[i*VertexSize:], vertex.Data())
	}

	indexData := make([]byte, len(chunk.indices)*IndexSize)
	for i, index := range chunk.indices {
		binary.LittleEndian.PutUint32(indexData[i*IndexSize:], uint32(index))
	}

	if err := tm.bucketAllocator.Upload(vertexId, int(lodLevel), vertexData); err != nil {
		tm.bucketAllocator.FreeChunk(vertexId, int(lodLevel))
		tm.bucketAllocator.FreeChunk(indexId, int(lodLevel))
		log.Printf("Failed to upload vertex data: %v", err)
		return false
	}

	if err := tm.bucketAllocator.Upload(indexId, int(lodLevel), indexData); err != nil {
		tm.bucketAllocator.FreeChunk(vertexId, int(lodLevel))
		tm.bucketAllocator.FreeChunk(indexId, int(lodLevel))
		log.Printf("Failed to upload index data: %v", err)
		return false
	}

	// Store GPU chunk data
	chunk.allocated = true
	chunk.vertexOffset = vertexOffset
	chunk.indexOffset = indexOffset
	chunk.vertexBufferId = vertexId
	chunk.indexBufferId = indexId

	tm.chunks[chunkKey] = chunk

	// Link GPU chunk to spatial chunk
	if spatialChunk, exists := tm.chunkRegistry[chunkKey]; exists {
		spatialChunk.UserData = chunk
	}

	return true
}

func (tm *TerrainManager) checkForUpdates() {
	if firstRun {
		// Generate the test line of chunks
		tm.GenerateTestLine()

		// Capture last position for update tracking
		tm.lastUpdatePos = tm.lastPosition
		firstRun = false

		if tm.debugMode {
			// Log memory usage after loading all chunks
			log.Printf("Memory stats after loading all test chunks: %s",
				tm.bucketAllocator.GetDetailedStats())

			log.Printf("Terrain grid stats after loading all test chunks: %s",
				tm.terrainGrid.GetDetailedStats())

			log.Printf("Terrain grid visualization: %s", tm.terrainGrid.VisualizeGrid())
		}
	}
}

func (tm *TerrainManager) updateVisibility() {
	// Create a simple frustum for testing (looking north with 90-degree FOV)
	direction := float32(math.Pi / 2) // Looking north (positive Z)
	fov := float32(math.Pi / 2)       // 90 degrees
	near := float32(10.0)
	far := float32(10000.0)

	frustum := spatial.NewFrustumFromCamera(tm.lastPosition, direction, fov, near, far)

	// Update visibility in terrain grid
	tm.terrainGrid.UpdateVisibility(tm.lastPosition, frustum)

	visibleChunks := tm.terrainGrid.GetVisibleChunks()

	if tm.debugMode {
		log.Printf("Terrain grid visibility update: %d/%d chunks visible",
			len(visibleChunks), tm.terrainGrid.Stats().TotalChunks)
	}
}

func (tm *TerrainManager) updateLoop() {
	log.Println("Starting update loop.")
	ticker := time.NewTicker(TICKER_SPEED)
	defer ticker.Stop()

	for {
		select {
		case <-tm.shutdown:
			return
		case <-ticker.C:
			tm.checkForUpdates()
		}
	}
}

// generateOceanHeightmap creates a flat heightmap (all water)
func (tm *TerrainManager) generateOceanHeightmap() *image.Gray16 {
	// Create a new Gray16 image with the correct dimensions
	img := image.NewGray16(image.Rect(0, 0, MAP_LENGTH+1, MAP_LENGTH+1))

	// Fill with a constant elevation value slightly below sea level
	// 32768 is the zero point (sea level) in our 16-bit heightmap
	oceanLevel := uint16(32760) // Slightly below sea level

	for y := 0; y <= MAP_LENGTH; y++ {
		for x := 0; x <= MAP_LENGTH; x++ {
			img.SetGray16(x, y, color.Gray16{Y: oceanLevel})
		}
	}

	return img
}

// generateOceanWatermap creates a watermap with all water
func (tm *TerrainManager) generateOceanWatermap() *image.Gray {
	// Create a new Gray image with the correct dimensions
	img := image.NewGray(image.Rect(0, 0, MAP_LENGTH+1, MAP_LENGTH+1))

	// Fill with water value (255 represents water in our implementation)
	waterValue := uint8(255)

	for y := 0; y <= MAP_LENGTH; y++ {
		for x := 0; x <= MAP_LENGTH; x++ {
			img.SetGray(x, y, color.Gray{Y: waterValue})
		}
	}

	return img
}

// Loads a watermap (coord.swb.png) into the TerrainManager
func (tm *TerrainManager) LoadWatermap(coord string) error {
	filepath := MAPS_PATH + coord + WATERMAP_EXTENSION

	// Try to open the file first
	file, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create an ocean watermap instead
			log.Printf("Watermap file not found for %s, generating ocean tile", coord)
			tm.watermaps[coord] = tm.generateOceanWatermap()
			return nil
		}
		return fmt.Errorf("failed to open watermap file: %w", err)
	}
	defer file.Close()

	// If file exists, proceed with normal loading
	rawImg, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode watermap: %w", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray)
	if !ok {
		return fmt.Errorf("watermap is not in 8-bit grayscale format")
	}

	tm.watermaps[coord] = grayImg
	return nil
}

// Loads a heightmap (coord.hgt.png) into the TerrainManager
func (tm *TerrainManager) LoadHeightmap(coord string) error {
	filepath := MAPS_PATH + coord + HEIGHTMAP_EXTENSION

	// Try to open the file first
	file, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create an ocean heightmap instead
			log.Printf("Heightmap file not found for %s, generating ocean tile", coord)
			tm.heightmaps[coord] = tm.generateOceanHeightmap()
			return nil
		}
		return fmt.Errorf("failed to open heightmap file: %w", err)
	}
	defer file.Close()

	// If file exists, proceed with normal loading
	rawImg, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode heightmap: %w", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray16)
	if !ok {
		return fmt.Errorf("heightmap is not in 16-bit grayscale format")
	}

	tm.heightmaps[coord] = grayImg
	return nil
}

// Unloads both the water and heightmaps
func (tm *TerrainManager) UnloadMaps(coord string) {
	tm.unloadWatermap(coord)
	tm.unloadHeightmap(coord)
}

// Unloads a watermap (coord) from the TerrainManager
func (tm *TerrainManager) unloadWatermap(coord string) {
	delete(tm.watermaps, coord)
}

// Unloads a heightmap (coord) from the TerrainManager
func (tm *TerrainManager) unloadHeightmap(coord string) {
	delete(tm.heightmaps, coord)
}

func (tm *TerrainManager) Destroy() {
	// Signal shutdown
	close(tm.shutdown)
	tm.bucketAllocator.Destroy()
}

// RenderAllocatedChunks draws chunks directly from bucket allocator
func (tm *TerrainManager) RenderAllocatedChunks(cmdBuffer vk.CommandBuffer) {
	// Get visible chunks from terrain grid
	visibleChunks := tm.terrainGrid.GetVisibleChunksSorted()

	if len(visibleChunks) == 0 {
		return
	}

	for _, spatialChunk := range visibleChunks {
		// Get GPU chunk from UserData
		chunk, ok := spatialChunk.UserData.(*Chunk)
		if !ok || chunk == nil || !chunk.allocated {
			continue
		}

		// Get vertex buffer information
		vertexInfo, err := tm.bucketAllocator.GetAllocationInfo(
			chunk.vertexBufferId, int(chunk.lodLevel))
		if err != nil {
			log.Printf("Failed to get vertex allocation info: %v", err)
			continue
		}

		// Get index buffer information
		indexInfo, err := tm.bucketAllocator.GetAllocationInfo(
			chunk.indexBufferId, int(chunk.lodLevel))
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

		// Draw indexed
		vk.CmdDrawIndexed(cmdBuffer, uint32(len(chunk.indices)), 1, 0, 0, 0)
	}
}
