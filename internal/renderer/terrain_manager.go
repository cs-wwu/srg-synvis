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
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

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

	MAX_IMAGE_MAPS = 4

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

// A single TerrainManager to manage load/unloading images and chunks
type TerrainManager struct {
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	activeVertices  []Vertex
	activeIndices   []Index
	pendingVertices []Vertex
	pendingIndices  []Index

	updated    atomic.Bool
	chunkMutex sync.RWMutex

	activeVertexBuffer  vk.Buffer
	activeIndexBuffer   vk.Buffer
	pendingVertexBuffer vk.Buffer
	pendingIndexBuffer  vk.Buffer

	activeIndexMemory   vk.DeviceMemory
	activeVertexMemory  vk.DeviceMemory
	pendingIndexMemory  vk.DeviceMemory
	pendingVertexMemory vk.DeviceMemory

	lastPosition  lin.Vec3
	lastUpdatePos lin.Vec3

	heightmaps    map[string]*image.Gray16
	watermaps     map[string]*image.Gray
	textures      map[string]*Texture
	chunks        map[string]*Chunk
	visibleChunks map[string]bool
	orderedChunks []*Chunk

	frustum    *Frustum
	viewMatrix *lin.Mat4x4
	projMatrix *lin.Mat4x4

	shutdown chan struct{}
}

func (tm *TerrainManager) UpdateLastPosition(pos lin.Vec3) {
	tm.chunkMutex.Lock()
	tm.lastPosition = pos
	tm.chunkMutex.Unlock()
}

func NewTerrainManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties) *TerrainManager {
	tm := &TerrainManager{
		device:        device,
		memProps:      memProps,
		lastPosition:  lin.Vec3{0.0, 0.0, 0.0},
		heightmaps:    make(map[string]*image.Gray16, MAX_IMAGE_MAPS),
		watermaps:     make(map[string]*image.Gray, MAX_IMAGE_MAPS),
		textures:      make(map[string]*Texture, MAX_IMAGE_MAPS),
		chunks:        make(map[string]*Chunk),
		visibleChunks: make(map[string]bool),
		orderedChunks: make([]*Chunk, 0),
		frustum:       &Frustum{},
		viewMatrix:    &lin.Mat4x4{},
		projMatrix:    &lin.Mat4x4{},
		shutdown:      make(chan struct{}),
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

	// Clear the visible chunks map
	for k := range tm.visibleChunks {
		delete(tm.visibleChunks, k)
	}
}

func (tm *TerrainManager) LoadMultipleMaps(coords []string) error {
	if len(coords) == 0 {
		return nil
	}

	// Load the first map with clearing existing chunks
	if err := tm.LoadMap(coords[0], true); err != nil {
		log.Printf("Error loading map %s: %v", coords[0], err)
		// Continue with other maps even if one fails
	}

	// Load additional maps without clearing
	for i := 1; i < len(coords); i++ {
		if err := tm.LoadMap(coords[i], false); err != nil {
			log.Printf("Error loading map %s: %v", coords[i], err)
			// Continue with other maps even if one fails
		}
	}

	return nil
}

// LoadMap loads a heightmap and its associated watermap
func (tm *TerrainManager) LoadMap(coord string, clearExisting bool) error {
	tm.chunkMutex.Lock()
	defer tm.chunkMutex.Unlock()

	if clearExisting {
		tm.ClearChunks()
	}

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

func (tm *TerrainManager) HasCrossedUpdateThreshold(new lin.Vec3) bool {
	tm.chunkMutex.RLock()
	defer tm.chunkMutex.RUnlock()

	dx := math.Abs(float64(new[0] - tm.lastUpdatePos[0]))
	dy := math.Abs(float64(new[1] - tm.lastUpdatePos[1]))
	dz := math.Abs(float64(new[2] - tm.lastUpdatePos[2]))

	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	return dist >= LOD_DISTANCE_THRESHOLD
}

func (tm *TerrainManager) selectLODLevel(chunkX, chunkZ int, cameraPos lin.Vec3) LODLevel {
	// Calculate chunk center position
	chunkCenterX := float32(chunkX) + float32(CHUNK_SIZE)/2
	chunkCenterZ := float32(chunkZ) + float32(CHUNK_SIZE)/2

	// Calculate average height for chunk center (or use actual terrain height if available)
	// For now, using camera Y directly since terrain height might be complex to calculate
	chunkCenterY := float32(0.0) // This could be refined based on actual terrain height

	// Calculate 3D distance
	dx := chunkCenterX - cameraPos[0]
	dy := chunkCenterY - cameraPos[1]
	dz := chunkCenterZ - cameraPos[2]
	dist3DSq := dx*dx + dy*dy + dz*dz

	// Updated thresholds for 3D distance
	// Square root of previous values to maintain similar visual transitions
	switch {
	case dist3DSq < 240_000: // ~490 units away (including height)
		return LOD_0
	case dist3DSq < 960_000: // ~980 units away
		return LOD_1
	case dist3DSq < 3_840_000: // ~1960 units away
		return LOD_2
	default:
		return LOD_3
	}
}

func (tm *TerrainManager) FillPendingBuffers() {
	// Clear pending data
	tm.pendingVertices = tm.pendingVertices[:0]
	tm.pendingIndices = tm.pendingIndices[:0]

	// Collect vertices and indices
	vertexOffset := Index(0)
	for _, chunk := range tm.orderedChunks {
		// Add vertices
		tm.pendingVertices = append(tm.pendingVertices, chunk.vertices...)

		// Add indices with offset
		for _, idx := range chunk.indices {
			// Add the vertex offset to the current index
			newIndex := idx + Index(vertexOffset)
			tm.pendingIndices = append(tm.pendingIndices, newIndex)
		}
		vertexOffset += Index(len(chunk.vertices))
	}

	// Update pending buffers
	tm.updateBuffer(tm.pendingVertexMemory, tm.pendingVertices)
	tm.updateBuffer(tm.pendingIndexMemory, tm.pendingIndices)
}

func (tm *TerrainManager) checkForUpdates() {
	if tm.HasCrossedUpdateThreshold(tm.lastPosition) {
		// Get current map and adjacent maps
		mapsToLoad := GetAdjacentMaps(tm.lastPosition)

		// Determine which maps need to be loaded and which can be unloaded
		mapsToUnload := make([]string, 0)
		mapsNeedingLoad := make([]string, 0)

		// Check which maps we need to load
		for _, mapCoord := range mapsToLoad {
			if _, exists := tm.heightmaps[mapCoord]; !exists {
				mapsNeedingLoad = append(mapsNeedingLoad, mapCoord)
			}
		}

		// Check which maps we need to unload
		for loadedMap := range tm.heightmaps {
			stillNeeded := false
			for _, mapToKeep := range mapsToLoad {
				if loadedMap == mapToKeep {
					stillNeeded = true
					break
				}
			}
			if !stillNeeded {
				mapsToUnload = append(mapsToUnload, loadedMap)
			}
		}

		// Unload maps that are no longer needed
		for _, mapCoord := range mapsToUnload {
			log.Printf("Unloading map: %s", mapCoord)
			tm.UnloadMaps(mapCoord)
		}

		// Load new maps if needed
		if len(mapsNeedingLoad) > 0 {
			log.Printf("Loading %d new maps", len(mapsNeedingLoad))

			// Determine if we need to reload all maps or just add new ones
			if len(tm.heightmaps) == 0 {
				// No maps loaded, load everything
				tm.LoadMultipleMaps(mapsToLoad)
			} else {
				// Some maps already loaded, just add new ones
				for _, mapCoord := range mapsNeedingLoad {
					tm.LoadMap(mapCoord, false)
				}
			}
		}

		// Update buffers and mark as updated
		tm.FillPendingBuffers()
		tm.updated.Store(true)
		tm.lastUpdatePos = tm.lastPosition
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
	pendingVertexBuf := as.CreateBuffer(*tm.device, *tm.memProps, vertexData, BufferUsageVertexBufferBit)
	tm.pendingVertexBuffer = pendingVertexBuf.Buffer
	tm.pendingVertexMemory = pendingVertexBuf.Memory

	// Index buffers
	indexData := make([]byte, MAX_INDICES*IndexSize) // uint32 = 4 bytes
	activeIndexBuf := as.CreateBuffer(*tm.device, *tm.memProps, indexData, BufferUsageIndexBufferBit)
	tm.activeIndexBuffer = activeIndexBuf.Buffer
	tm.activeIndexMemory = activeIndexBuf.Memory
	pendingIndexBuf := as.CreateBuffer(*tm.device, *tm.memProps, indexData, BufferUsageIndexBufferBit)
	tm.pendingIndexBuffer = pendingIndexBuf.Buffer
	tm.pendingIndexMemory = pendingIndexBuf.Memory
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

func GetAdjacentMaps(pos lin.Vec3) []string {
	// Get current map coordinate
	currentMap := CoordinateIntToMap(int(pos[0]), int(pos[2]))

	// Get the base coordinates of the current map
	mapX, mapZ := CoordinateMapToInt(currentMap)

	// Create a slice to hold all map coordinates
	maps := []string{
		currentMap,
		CoordinateIntToMap(mapX+MAP_LENGTH, mapZ),            // East
		CoordinateIntToMap(mapX, mapZ+MAP_LENGTH),            // North
		CoordinateIntToMap(mapX+MAP_LENGTH, mapZ+MAP_LENGTH), // Northeast
	}

	return maps
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

// Loads a texture (coord.osm.png) into the TerrainManager
// func (tm *TerrainManager) LoadTexture(coord string) error {
// 	filepath := MAPS_PATH + coord + TEXTURE_EXTENSION
// 	_, err := loadMap(filepath) // TODO: replace "_" with rawImg
// 	if err != nil {
// 		return fmt.Errorf("failed to load file: %w", err)
// 	}
// 	// Load and store the texture
// 	// TODO load the texture into Vulkan with rawImg
// 	// placeholder
// 	texture := &Texture{}

// 	// Store that texture in a usable state in the texture map
// 	tm.textures[coord] = texture
// 	return nil
// }

// Unloads both the water and heightmaps
func (tm *TerrainManager) UnloadMaps(coord string) {
	tm.unloadWatermap(coord)
	tm.unloadHeightmap(coord)
	tm.unloadTexture(coord)
}

// Unloads a watermap (coord) from the TerrainManager
func (tm *TerrainManager) unloadWatermap(coord string) {
	delete(tm.watermaps, coord)
}

// Unloads a heightmap (coord) from the TerrainManager
func (tm *TerrainManager) unloadHeightmap(coord string) {
	delete(tm.heightmaps, coord)
}

// Unloads a texture (coord) from the TerrainManager
func (tm *TerrainManager) unloadTexture(coord string) {
	delete(tm.textures, coord) // TODO, problably will need to remove Vulkan stuff
}

// Loads a map and returns a png
func loadMap(filepath string) (image.Image, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load map %v: %w", filepath, err)
	}
	defer file.Close()

	rawImg, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode map %v: %w", filepath, err)
	}

	// Verify image size
	width, height := rawImg.Bounds().Dx(), rawImg.Bounds().Dy()
	if width != MAP_LENGTH+1 || height != MAP_LENGTH+1 {
		return nil, fmt.Errorf("bad image bounds: (%d, %d)", width, height)
	}

	return rawImg, nil
}

func (tm *TerrainManager) updateBuffer(memory vk.DeviceMemory, data interface{}) {
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

func (tm *TerrainManager) SwapBuffers() bool {
	if !tm.updated.Load() {
		return false
	}

	tm.chunkMutex.Lock()
	defer tm.chunkMutex.Unlock()

	// Swap buffer pointers
	tm.activeVertexBuffer, tm.pendingVertexBuffer = tm.pendingVertexBuffer, tm.activeVertexBuffer
	tm.activeIndexBuffer, tm.pendingIndexBuffer = tm.pendingIndexBuffer, tm.activeIndexBuffer
	tm.activeVertexMemory, tm.pendingVertexMemory = tm.pendingVertexMemory, tm.activeVertexMemory
	tm.activeIndexMemory, tm.pendingIndexMemory = tm.pendingIndexMemory, tm.activeIndexMemory

	// Store the active counts
	tm.activeVertices = tm.pendingVertices
	tm.activeIndices = tm.pendingIndices

	tm.updated.Store(false)
	return true
}

func (tm *TerrainManager) Destroy() {
	// Signal shutdown
	close(tm.shutdown)

	// Destroy buffers
	vk.DestroyBuffer(*tm.device, tm.activeVertexBuffer, nil)
	vk.DestroyBuffer(*tm.device, tm.activeIndexBuffer, nil)
	vk.DestroyBuffer(*tm.device, tm.pendingVertexBuffer, nil)
	vk.DestroyBuffer(*tm.device, tm.pendingIndexBuffer, nil)

	// Free memory
	vk.FreeMemory(*tm.device, tm.activeVertexMemory, nil)
	vk.FreeMemory(*tm.device, tm.activeIndexMemory, nil)
	vk.FreeMemory(*tm.device, tm.pendingVertexMemory, nil)
	vk.FreeMemory(*tm.device, tm.pendingIndexMemory, nil)
}

func (tm *TerrainManager) UpdateMatrices(view, proj *lin.Mat4x4) {
	tm.chunkMutex.Lock()
	defer tm.chunkMutex.Unlock()

	*tm.viewMatrix = *view
	*tm.projMatrix = *proj
}

// Returns active indicies with thread-safety
func (tm *TerrainManager) getActiveIndices() uint32 {
	tm.chunkMutex.RLock()
	defer tm.chunkMutex.RUnlock()
	return uint32(len(tm.activeIndices))
}

// GetActiveBuffers returns the currently active vertex and index buffers
func (tm *TerrainManager) GetActiveBuffers() (vk.Buffer, vk.Buffer) {
	tm.chunkMutex.RLock()
	defer tm.chunkMutex.RUnlock()
	return tm.activeVertexBuffer, tm.activeIndexBuffer
}

// Returns the visible chunks with thread-safety
func (tm *TerrainManager) GetVisibleChunks() int {
	tm.chunkMutex.RLock()
	defer tm.chunkMutex.RUnlock()
	return len(tm.visibleChunks)
}
