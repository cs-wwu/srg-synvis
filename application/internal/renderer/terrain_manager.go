package renderer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"unsafe"

	as "github.com/cs-wwu/srg-synvis/asche"

	// as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file should be used for chunk management, chunk rendering, etc.
// TODO maybe save raw arrays rather than image pointers?

const (
	WATERMAP_EXTENSION  = ".swb.png"
	HEIGHTMAP_EXTENSION = ".hgt.png"

	MAP_LENGTH      = 3600
	CHUNKS_PER_SIDE = 20
	CHUNKS_PER_MAP  = CHUNKS_PER_SIDE * CHUNKS_PER_SIDE
	CHUNK_SIZE      = MAP_LENGTH / CHUNKS_PER_SIDE
	NUM_MAPS        = 2 // TODO: Figure it out better
	MAX_CHUNKS      = NUM_MAPS * CHUNKS_PER_MAP

	WORLD_WIDTH  = 360 * int(ARC_DEGREES_TO_SECONDS)
	WORLD_HEIGHT = 180 * int(ARC_DEGREES_TO_SECONDS)

	VERTICES_PER_CHUNK = (CHUNK_SIZE + 1) * (CHUNK_SIZE + 1)
	INDICES_PER_CHUNK  = 2*CHUNK_SIZE*(CHUNK_SIZE-1) + 2*((CHUNK_SIZE-1)-1)

	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds

	LOD_LEVELS             = 3
	LOD_DISTANCE_THRESHOLD = CHUNK_SIZE * 2
)

// A single TerrainManager to manage load/unloading images and chunks
type TerrainManager struct {
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	vertices []Vertex
	indices  []uint32

	lastPosition  lin.Vec3
	lastUpdatePos lin.Vec3

	vertexBuffer      vk.Buffer
	vertexMemory      vk.DeviceMemory
	indexBuffer       vk.Buffer
	indexMemory       vk.DeviceMemory
	drawCommandBuffer vk.Buffer
	drawCommandMemory vk.DeviceMemory

	heightmaps    map[string]*image.Gray16
	watermaps     map[string]*image.Gray
	chunks        map[string]*Chunk
	visibleChunks map[string]bool
	orderedChunks []*Chunk

	drawCommand DrawCommand
}

type DrawCommand struct {
	IndexCount    uint32
	InstanceCount uint32
	FirstIndex    uint32
	VertexOffset  int32
	FirstInstance uint32
}

func (tm *TerrainManager) UpdateLastPosition(pos lin.Vec3) {
	if tm.HasCrossedThreshold(pos) { // TODO Make dynamic based on location
		tm.lastPosition = pos
		tm.LoadMap("n48w123")
		// tm.LoadMap("n48w122")
		// tm.LoadMap("n47w123")
		// tm.LoadMap("n47w122")
		log.Printf("Updating camera position to %v\n", pos)
	}
}

func NewTerrainManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties) *TerrainManager {
	tm := &TerrainManager{
		device:        device,
		memProps:      memProps,
		lastPosition:  lin.Vec3{0.0, 0.0, 0.0},
		heightmaps:    make(map[string]*image.Gray16, NUM_MAPS),
		watermaps:     make(map[string]*image.Gray, NUM_MAPS),
		chunks:        make(map[string]*Chunk),
		visibleChunks: make(map[string]bool),
		orderedChunks: make([]*Chunk, 0),
		drawCommand:   DrawCommand{},
	}

	tm.InitBuffers()
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

// LoadMap loads a heightmap and its associated watermap
func (tm *TerrainManager) LoadMap(coord string) error {
	log.Printf("Loading map: %s\n", coord)
	tm.ClearChunks()

	// Skip heightmaps if already loaded
	if _, exists := tm.heightmaps[coord]; !exists {
		// Load heightmap and watermap
		if err := tm.LoadHeightmap(coord); err != nil {
			return err
		}
		if err := tm.LoadWatermap(coord); err != nil {
			return err
		}
	}

	// Generate chunks for this map
	mapX, mapZ := CoordinateMapToInt(coord)
	for x := 0; x < CHUNKS_PER_SIDE; x++ {
		for z := 0; z < CHUNKS_PER_SIDE; z++ {
			chunkX := mapX + x*CHUNK_SIZE
			chunkZ := mapZ + z*CHUNK_SIZE
			tm.LoadChunk(chunkX, chunkZ)
		}
	}

	// TODO remove lod calculation
	sizes := make([]int, 4)
	for _, chunk := range tm.chunks {
		sizes[chunk.lodLevel]++
	}

	log.Printf("\nLOD sizes: %v\n\n", sizes)

	// Update GPU buffers
	tm.FillBuffers()

	return nil
}

func (tm *TerrainManager) HasCrossedThreshold(new lin.Vec3) bool {
	dx := math.Abs(float64(new[0] - tm.lastPosition[0]))
	dy := math.Abs(float64(new[1] - tm.lastPosition[1]))
	dz := math.Abs(float64(new[2] - tm.lastPosition[2]))

	// Calculate 3D distance
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

// FillBuffers updates GPU buffers with current geometry
func (tm *TerrainManager) FillBuffers() {
	if len(tm.chunks) == 0 {
		return
	}

	// Collect all vertices and indices
	var totalVertices []Vertex
	var totalIndices []uint32

	for _, chunk := range tm.orderedChunks {
		totalVertices = append(totalVertices, chunk.vertices...)
		for _, idx := range chunk.indices {
			totalIndices = append(totalIndices, uint32(idx))
		}
	}

	tm.vertices = totalVertices
	tm.indices = totalIndices

	// Update vertex buffer
	vertexData := make([]byte, len(totalVertices)*VertexSize)
	for i, vertex := range totalVertices {
		copy(vertexData[i*VertexSize:], vertex.Data())
	}

	var pVertexData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.vertexMemory, 0, vk.DeviceSize(len(vertexData)), 0, &pVertexData)
	vk.Memcopy(pVertexData, vertexData)
	vk.UnmapMemory(*tm.device, tm.vertexMemory)

	// Update index buffer
	indexData := make([]byte, len(totalIndices)*4)
	for i, index := range totalIndices {
		binary.LittleEndian.PutUint32(indexData[i*4:], index)
	}

	var pIndexData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.indexMemory, 0, vk.DeviceSize(len(indexData)), 0, &pIndexData)
	vk.Memcopy(pIndexData, indexData)
	vk.UnmapMemory(*tm.device, tm.indexMemory)

	// Update draw command
	tm.drawCommand = DrawCommand{
		IndexCount:    uint32(len(totalIndices)),
		InstanceCount: 1,
		FirstIndex:    0,
		VertexOffset:  0,
		FirstInstance: 0,
	}

	// Update draw command buffer
	commandData := make([]byte, unsafe.Sizeof(DrawCommand{}))
	binary.LittleEndian.PutUint32(commandData[0:], tm.drawCommand.IndexCount)
	binary.LittleEndian.PutUint32(commandData[4:], tm.drawCommand.InstanceCount)
	binary.LittleEndian.PutUint32(commandData[8:], tm.drawCommand.FirstIndex)
	binary.LittleEndian.PutUint32(commandData[12:], uint32(tm.drawCommand.VertexOffset))
	binary.LittleEndian.PutUint32(commandData[16:], tm.drawCommand.FirstInstance)

	var pCommandData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.drawCommandMemory, 0, vk.DeviceSize(len(commandData)), 0, &pCommandData)
	vk.Memcopy(pCommandData, commandData)
	vk.UnmapMemory(*tm.device, tm.drawCommandMemory)
}

// Intializes the ChunkManager buffers
func (tm *TerrainManager) InitBuffers() {
	// Vertex buffer
	maxVertices := MAX_CHUNKS * VERTICES_PER_CHUNK
	vertexData := make([]byte, maxVertices*VertexSize)
	vertexBuf := as.CreateBuffer(*tm.device, *tm.memProps, vertexData, BufferUsageVertexBufferBit)
	tm.vertexBuffer = vertexBuf.Buffer
	tm.vertexMemory = vertexBuf.Memory

	// Index buffer
	maxIndices := MAX_CHUNKS * INDICES_PER_CHUNK
	indexData := make([]byte, maxIndices*4) // uint32 = 4 bytes
	indexBuf := as.CreateBuffer(*tm.device, *tm.memProps, indexData, BufferUsageIndexBufferBit)
	tm.indexBuffer = indexBuf.Buffer
	tm.indexMemory = indexBuf.Memory

	// Draw command buffer
	drawCommandData := make([]byte, unsafe.Sizeof(DrawCommand{}))
	drawCommandBuffer := as.CreateBuffer(*tm.device, *tm.memProps, drawCommandData, BufferUsageIndirectBufferBit)
	tm.drawCommandBuffer = drawCommandBuffer.Buffer
	tm.drawCommandMemory = drawCommandBuffer.Memory
}

// updateDrawCommands updates the draw commands for visible chunks
func (tm *TerrainManager) updateDrawCommand() {
	// Pre-calculate total sizes to avoid reallocation
	totalVertices := 0
	totalIndices := 0
	for _, chunk := range tm.orderedChunks {
		chunkKey := ChunkKey(chunk.X, chunk.Z)
		if tm.visibleChunks[chunkKey] {
			totalVertices += len(chunk.vertices)
			totalIndices += len(chunk.indices)
		}
	}

	// Create single draw command
	tm.drawCommand = DrawCommand{
		IndexCount:    uint32(totalIndices),
		InstanceCount: 1,
		FirstIndex:    0,
		VertexOffset:  0,
		FirstInstance: 0,
	}

	// Update draw command buffer (can be static since it's small)
	commandData := [20]byte{} // sizeof(DrawCommand) = 20
	binary.LittleEndian.PutUint32(commandData[0:], tm.drawCommand.IndexCount)
	binary.LittleEndian.PutUint32(commandData[4:], tm.drawCommand.InstanceCount)
	binary.LittleEndian.PutUint32(commandData[8:], tm.drawCommand.FirstIndex)
	binary.LittleEndian.PutUint32(commandData[12:], uint32(tm.drawCommand.VertexOffset))
	binary.LittleEndian.PutUint32(commandData[16:], tm.drawCommand.FirstInstance)

	var pCommandData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.drawCommandMemory, 0, vk.DeviceSize(20), 0, &pCommandData)
	vk.Memcopy(pCommandData, commandData[:])
	vk.UnmapMemory(*tm.device, tm.drawCommandMemory)

	// Allocate exact buffer sizes once
	vertexData := make([]byte, totalVertices*VertexSize)
	indexData := make([]byte, totalIndices*4)

	// Fill buffers directly without intermediate slices
	vertexOffset := 0
	indexOffset := 0
	vertexCount := 0

	for _, chunk := range tm.orderedChunks {
		if tm.visibleChunks[ChunkKey(chunk.X, chunk.Z)] {
			// Copy vertices directly to vertexData
			for i, vertex := range chunk.vertices {
				copy(vertexData[(vertexOffset+i)*VertexSize:], vertex.Data())
			}

			// Copy indices directly to indexData with offset
			for _, idx := range chunk.indices {
				binary.LittleEndian.PutUint32(indexData[indexOffset:], uint32(idx)+uint32(vertexCount))
				indexOffset += 4
			}

			vertexOffset += len(chunk.vertices)
			vertexCount += len(chunk.vertices)
		}
	}

	// Update vertex buffer
	var pVertexData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.vertexMemory, 0, vk.DeviceSize(len(vertexData)), 0, &pVertexData)
	vk.Memcopy(pVertexData, vertexData)
	vk.UnmapMemory(*tm.device, tm.vertexMemory)

	// Update index buffer
	var pIndexData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.indexMemory, 0, vk.DeviceSize(len(indexData)), 0, &pIndexData)
	vk.Memcopy(pIndexData, indexData)
	vk.UnmapMemory(*tm.device, tm.indexMemory)
}

// Loads a watermap (coord.swb.png) into the ChunkManager
func (tm *TerrainManager) LoadWatermap(coord string) error {
	filepath := MAPS_PATH + coord + WATERMAP_EXTENSION
	rawImg, err := loadMap(filepath)
	if err != nil {
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray)
	if !ok {
		return fmt.Errorf("watermap is not in 8-bit grayscale format")
	}

	tm.watermaps[coord] = grayImg

	return nil
}

// Loads a heightmap (coord.hgt.png) into the ChunkManager
func (tm *TerrainManager) LoadHeightmap(coord string) error {
	filepath := MAPS_PATH + coord + HEIGHTMAP_EXTENSION
	rawImg, err := loadMap(filepath)
	if err != nil {
		return fmt.Errorf("failed to load file: %w", err)
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

// Unloads a watermap (coord) from the ChunkManager
func (tm *TerrainManager) unloadWatermap(coord string) {
	delete(tm.watermaps, coord)
}

// Unloads a heightmap (coord) from the ChunkManager
func (tm *TerrainManager) unloadHeightmap(coord string) {
	delete(tm.heightmaps, coord)
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

func (tm *TerrainManager) Destroy() {
	vk.DestroyBuffer(*tm.device, tm.vertexBuffer, nil)
	vk.FreeMemory(*tm.device, tm.vertexMemory, nil)

	vk.DestroyBuffer(*tm.device, tm.indexBuffer, nil)
	vk.FreeMemory(*tm.device, tm.indexMemory, nil)

	// Cleanup draw command buffer - this was missing
	vk.DestroyBuffer(*tm.device, tm.drawCommandBuffer, nil)
	vk.FreeMemory(*tm.device, tm.drawCommandMemory, nil)
}
