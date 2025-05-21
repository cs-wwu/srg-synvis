package renderer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"sort"
	"time"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/core/memory"
	as "github.com/cs-wwu/srg-synvis/pkg/asche"

	// as "github.com/vulkan-go/asche"

	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file should be used for chunk management, chunk rendering, etc.

const (
	WATERMAP_EXTENSION  = ".swb.png"
	HEIGHTMAP_EXTENSION = ".hgt.png"
	TEXTURE_EXTENSION   = ".osm.png"

	MAX_IMAGE_MAPS = 1

	TICKER_SPEED = time.Second / 30 // x times per second

	MAP_LENGTH      = 3600
	CHUNKS_PER_SIDE = 20
	CHUNKS_PER_MAP  = CHUNKS_PER_SIDE * CHUNKS_PER_SIDE
	CHUNK_SIZE      = MAP_LENGTH / CHUNKS_PER_SIDE

	WORLD_WIDTH  = 360 * int(ARC_DEGREES_TO_SECONDS)
	WORLD_HEIGHT = 180 * int(ARC_DEGREES_TO_SECONDS)

	VERTICES_PER_CHUNK = (CHUNK_SIZE + 1) * (CHUNK_SIZE + 1)
	INDICES_PER_CHUNK  = 2*CHUNK_SIZE*(CHUNK_SIZE-1) + 2*((CHUNK_SIZE-1)-1)

	MAX_VERTICES = VERTICES_PER_CHUNK * CHUNKS_PER_MAP
	MAX_INDICES  = INDICES_PER_CHUNK * CHUNKS_PER_MAP

	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds

	LOD_LEVELS             = 3
	LOD_DISTANCE_THRESHOLD = CHUNK_SIZE * 2
)

var firstRun = true
var LODBucketPercentages = []float32{0.25, 0.25, 0.25, 0.25}

// A single TerrainManager to manage load/unloading images and chunks
type TerrainManager struct {
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	activeVertices []Vertex
	activeIndices  []Index

	activeVertexBuffer vk.Buffer
	activeIndexBuffer  vk.Buffer

	activeIndexMemory  vk.DeviceMemory
	activeVertexMemory vk.DeviceMemory

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

	tm.InitBuffers()
	go tm.updateLoop()
	return tm
}

// Clears all chunks in the terrain manager (chunks in buffers are left)
func (tm *TerrainManager) ClearChunks() {
	// Clear the chunks map
	for k := range tm.chunks {
		delete(tm.chunks, k)
	}

	// Clear the ordered chunks slice
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

	// Generate chunks for this map
	mapX, mapZ := CoordinateMapToInt(coord)
	for x := range CHUNKS_PER_SIDE {
		for z := range CHUNKS_PER_SIDE {
			chunkX := mapX + x*CHUNK_SIZE
			chunkZ := mapZ + z*CHUNK_SIZE
			tm.LoadChunk(chunkX, chunkZ)
		}
	}

	log.Printf("Loaded map: %s", coord)
	return nil
}

func (tm *TerrainManager) FillActiveBuffers() {
	// Clear pending data
	tm.activeVertices = tm.activeVertices[:0]
	tm.activeIndices = tm.activeIndices[:0]

	sort.Slice(tm.orderedChunks, func(i, j int) bool {
		if tm.orderedChunks[i].Z != tm.orderedChunks[j].Z {
			return tm.orderedChunks[i].Z < tm.orderedChunks[j].Z
		}
		return tm.orderedChunks[i].X < tm.orderedChunks[j].X
	})

	// Collect vertices and indices
	vertexOffset := Index(0)
	for i, chunk := range tm.orderedChunks {
		// Add vertices
		tm.activeVertices = append(tm.activeVertices, chunk.vertices...)

		// If this isn't the first chunk, add a primitive restart index
		if i > 0 {
			tm.activeIndices = append(tm.activeIndices, 0xFFFFFFFF) // Primitive restart index
		}

		// Add indices with offset
		for _, idx := range chunk.indices {
			newIndex := idx + Index(vertexOffset)
			tm.activeIndices = append(tm.activeIndices, newIndex)
		}

		vertexOffset += Index(len(chunk.vertices))
	}

	// Update active buffers
	tm.updateBuffer(tm.activeVertexMemory, tm.activeVertices)
	tm.updateBuffer(tm.activeIndexMemory, tm.activeIndices)
}

func (tm *TerrainManager) checkForUpdates() {
	if firstRun {
		// Get current map and adjacent maps
		thisMap := CoordinateIntToMap(int(tm.lastPosition[0]), int(tm.lastPosition[2]))

		// Load new maps if needed
		log.Printf("Loading %s new maps", thisMap)
		tm.LoadMap(thisMap)

		for _, chunk := range tm.chunks {
			tm.LoadChunk(chunk.X, chunk.Z)
		}

		// Update buffers and mark as updated
		tm.FillActiveBuffers()
		tm.lastUpdatePos = tm.lastPosition
		firstRun = false
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

// Intializes the ChunkManager buffers
func (tm *TerrainManager) InitBuffers() {
	// Vertex buffers
	vertexData := make([]byte, MAX_VERTICES*VertexSize)
	activeVertexBuf := as.CreateBuffer(*tm.device, *tm.memProps, vertexData, BufferUsageVertexBufferBit)
	tm.activeVertexBuffer = activeVertexBuf.Buffer
	tm.activeVertexMemory = activeVertexBuf.Memory

	// Index buffers
	indexData := make([]byte, MAX_INDICES*IndexSize) // uint32 = 4 bytes
	activeIndexBuf := as.CreateBuffer(*tm.device, *tm.memProps, indexData, BufferUsageIndexBufferBit)
	tm.activeIndexBuffer = activeIndexBuf.Buffer
	tm.activeIndexMemory = activeIndexBuf.Memory
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

func (tm *TerrainManager) updateBuffer(memory vk.DeviceMemory, data any) {
	var bufferData []byte
	switch v := data.(type) {
	case []Vertex:
		bufferData = make([]byte, len(v)*VertexSize)
		for i, vertex := range v {
			copy(bufferData[i*VertexSize:], vertex.Data())
		}
	case []Index:
		bufferData = make([]byte, len(v)*4)
		for i, index := range v {
			binary.LittleEndian.PutUint32(bufferData[i*4:], uint32(index))
		}
	default:
		log.Printf("Warning: Unknown buffer data type")
		return
	}

	var pData unsafe.Pointer
	vk.MapMemory(*tm.device, memory, 0, vk.DeviceSize(len(bufferData)), 0, &pData)
	vk.Memcopy(pData, bufferData)
	vk.UnmapMemory(*tm.device, memory)
}

func (tm *TerrainManager) Destroy() {
	// Signal shutdown
	close(tm.shutdown)

	// Destroy buffers
	vk.DestroyBuffer(*tm.device, tm.activeVertexBuffer, nil)
	vk.DestroyBuffer(*tm.device, tm.activeIndexBuffer, nil)
	vk.FreeMemory(*tm.device, tm.activeVertexMemory, nil)
	vk.FreeMemory(*tm.device, tm.activeIndexMemory, nil)
}

// Returns active indicies with thread-safety
func (tm *TerrainManager) getActiveIndices() uint32 {
	return uint32(len(tm.activeIndices))
}

// GetActiveBuffers returns the currently active vertex and index buffers
func (tm *TerrainManager) GetActiveBuffers() (vk.Buffer, vk.Buffer) {
	return tm.activeVertexBuffer, tm.activeIndexBuffer
}
