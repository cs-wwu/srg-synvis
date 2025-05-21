package renderer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"time"

	"github.com/cs-wwu/srg-synvis/core/memory"

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
	CHUNK_SIZE      = MAP_LENGTH / CHUNKS_PER_SIDE
)

var firstRun = true
var LODBucketPercentages = []float32{0.25, 0.25, 0.25, 0.25}

// A single TerrainManager to manage load/unloading images and chunks
type TerrainManager struct {
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	lastPosition  lin.Vec3
	lastUpdatePos lin.Vec3

	heightmaps    map[string]*image.Gray16
	watermaps     map[string]*image.Gray
	chunks        map[string]*Chunk
	orderedChunks []*Chunk

	bucketAllocator *memory.BucketAllocator

	shutdown chan struct{}
}

func (tm *TerrainManager) UpdateLastPosition(pos lin.Vec3) {
	tm.lastPosition = pos
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

func NewTerrainManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties) *TerrainManager {
	lodChunkSizes := calculateLODChunkSizes()
	fmt.Printf("LOD Chunk Sizes: ")
	for _, size := range lodChunkSizes {
		fmt.Printf("%.1fKB ", float32((size)/1024))
	}
	fmt.Println()
	fmt.Printf("LOD Bucket Percentages: %v\n", LODBucketPercentages)
	bucketAllocator := memory.NewBucketAllocator(device, memProps, LODBucketPercentages, lodChunkSizes)

	tm := &TerrainManager{
		device:          device,
		memProps:        memProps,
		bucketAllocator: bucketAllocator, // Assign the initialized allocator
		lastPosition:    lin.Vec3{0.0, 0.0, 0.0},
		heightmaps:      make(map[string]*image.Gray16, MAX_IMAGE_MAPS),
		watermaps:       make(map[string]*image.Gray, MAX_IMAGE_MAPS),
		chunks:          make(map[string]*Chunk),
		orderedChunks:   make([]*Chunk, 0),
		shutdown:        make(chan struct{}),
	}

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
	}
	tm.orderedChunks = tm.orderedChunks[:0]
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
	// Starting position based on the camera location
	startX := int(tm.lastPosition[0])
	startZ := int(tm.lastPosition[2])

	// Clear any existing chunks
	tm.ClearChunks()

	// Get current map for heightmap/watermap access
	baseMap := CoordinateIntToMap(startX, startZ)
	if _, exists := tm.heightmaps[baseMap]; !exists {
		// Load the base map if not already loaded
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

	// Generate a line of 20 chunks extending along Z axis
	numChunks := 20
	chunkSpacing := CHUNK_SIZE // Space chunks out to see them clearly

	log.Printf("Generating test line of %d chunks starting at (%d, %d)", numChunks, startX, startZ)

	// Create chunks at increasing distances
	for i := range numChunks {
		// Calculate position - create a line going away from camera
		chunkX := startX
		chunkZ := startZ + (i * chunkSpacing)

		// Force different LOD levels for testing
		forcedLODLevel := (i / 5) % 4 // 0,0,0,0,0, 1,1,1,1,1, 2,2,2,2,2, 3,3,3,3,3

		// Generate the chunk
		mapCoords := CoordinateIntToMap(chunkX, chunkZ)
		if _, exists := tm.heightmaps[mapCoords]; !exists {
			// If we need a different map, load it
			if err := tm.LoadHeightmap(mapCoords); err != nil {
				log.Printf("Using base map for chunk %d: original map %s not found", i, mapCoords)
				// Use base map instead
				mapCoords = baseMap
			} else {
				// Also load the watermap
				if err := tm.LoadWatermap(mapCoords); err != nil {
					log.Printf("Error loading watermap: %v", err)
				}
			}
		}

		// Create a unique key for this test chunk
		chunkKey := fmt.Sprintf("test_chunk_%d", i)

		// Load the chunk with the forced LOD level
		tm.LoadTestChunk(chunkX, chunkZ, LODLevel(forcedLODLevel), chunkKey, mapCoords)

		log.Printf("Created test chunk %d at (%d, %d) with LOD %d",
			i, chunkX, chunkZ, forcedLODLevel)
	}

	log.Printf("Finished generating test line with %d chunks", len(tm.chunks))
}

// Add this specialized version of LoadChunk for testing specific LOD levels
func (tm *TerrainManager) LoadTestChunk(chunkX, chunkZ int, forcedLOD LODLevel, chunkKey, mapCoords string) {
	heightmap := tm.heightmaps[mapCoords]
	watermap := tm.watermaps[mapCoords]

	if heightmap == nil || watermap == nil {
		log.Printf("Warning: Map(s) not loaded for coords %s", mapCoords)
		return
	}

	// Create new chunk with forced LOD level
	chunk := &Chunk{
		X:        chunkX,
		Z:        chunkZ,
		lodLevel: forcedLOD,
	}

	// Calculate geometry
	calculateChunkVertices(heightmap, watermap, chunk)
	calculateChunkIndices(chunk)
	calculateChunkNormals(chunk)

	// Allocate vertex buffer
	vertexId := fmt.Sprintf("%s_vertices", chunkKey)
	vertexDataSize := vk.DeviceSize(len(chunk.vertices) * VertexSize)
	success, vertexOffset := tm.bucketAllocator.AllocateChunk(vertexId, vertexDataSize, int(forcedLOD))
	if !success {
		log.Printf("Failed to allocate vertex buffer for chunk %s", chunkKey)
		return
	}

	// Allocate index buffer
	indexId := fmt.Sprintf("%s_indices", chunkKey)
	indexDataSize := vk.DeviceSize(len(chunk.indices) * IndexSize)
	success, indexOffset := tm.bucketAllocator.AllocateChunk(indexId, indexDataSize, int(forcedLOD))
	if !success {
		// Free the previously allocated vertex buffer
		tm.bucketAllocator.FreeChunk(vertexId, int(forcedLOD))
		log.Printf("Failed to allocate index buffer for chunk %s", chunkKey)
		return
	}

	// Prepare vertex data for upload
	vertexData := make([]byte, len(chunk.vertices)*VertexSize)
	for i, vertex := range chunk.vertices {
		copy(vertexData[i*VertexSize:], vertex.Data())
	}

	// Prepare index data for upload
	indexData := make([]byte, len(chunk.indices)*IndexSize)
	for i, index := range chunk.indices {
		binary.LittleEndian.PutUint32(indexData[i*IndexSize:], uint32(index))
	}

	// Upload vertex data
	err := tm.bucketAllocator.Upload(vertexId, int(forcedLOD), vertexData)
	if err != nil {
		tm.bucketAllocator.FreeChunk(vertexId, int(forcedLOD))
		tm.bucketAllocator.FreeChunk(indexId, int(forcedLOD))
		log.Printf("Failed to upload vertex data: %v", err)
		return
	}

	// Upload index data
	err = tm.bucketAllocator.Upload(indexId, int(forcedLOD), indexData)
	if err != nil {
		tm.bucketAllocator.FreeChunk(vertexId, int(forcedLOD))
		tm.bucketAllocator.FreeChunk(indexId, int(forcedLOD))
		log.Printf("Failed to upload index data: %v", err)
		return
	}

	// Update chunk with allocation information
	chunk.allocated = true
	chunk.vertexOffset = vertexOffset
	chunk.indexOffset = indexOffset
	chunk.vertexBufferId = vertexId
	chunk.indexBufferId = indexId

	// Store in terrain manager
	tm.chunks[chunkKey] = chunk
	tm.orderedChunks = append(tm.orderedChunks, chunk)
}

func (tm *TerrainManager) checkForUpdates() {
	if firstRun {
		// Generate the test line of chunks
		tm.GenerateTestLine()

		// Capture last position for update tracking
		tm.lastUpdatePos = tm.lastPosition
		firstRun = false

		// Log memory usage after loading all chunks
		log.Printf("Memory stats after loading all test chunks: %s",
			tm.bucketAllocator.GetDetailedStats())
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
	if len(tm.orderedChunks) == 0 {
		return
	}

	for _, chunk := range tm.orderedChunks {
		// Skip chunks that weren't allocated with the bucket allocator
		if !chunk.allocated {
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
