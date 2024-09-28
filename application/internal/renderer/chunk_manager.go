package renderer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"unsafe"

	as "github.com/vulkan-go/asche"
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
	NUM_MAPS        = 4 // TODO change to 9 in final version
	MAX_CHUNKS      = NUM_MAPS * CHUNKS_PER_MAP

	WORLD_WIDTH  = 360 * int(ARC_DEGREES_TO_SECONDS)
	WORLD_HEIGHT = 180 * int(ARC_DEGREES_TO_SECONDS)
	CHUNKS_X     = WORLD_WIDTH / CHUNK_SIZE
	CHUNKS_Z     = WORLD_HEIGHT / CHUNK_SIZE

	VERTICES_PER_CHUNK = (CHUNK_SIZE + 1) * (CHUNK_SIZE + 1)
	INDICES_PER_CHUNK  = 2*CHUNK_SIZE*(CHUNK_SIZE-1) + 2*((CHUNK_SIZE-1)-1)

	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds

	LOD_LEVELS             = 3
	LOD_DISTANCE_THRESHOLD = 500.0
)

// A single ChunkManager to manage load/unloading images and chunks
type ChunkManager struct {
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	lastPosition lin.Vec3

	vertexBuffer      vk.Buffer
	vertexMemory      vk.DeviceMemory
	indexBuffer       vk.Buffer
	indexMemory       vk.DeviceMemory
	drawCommandBuffer vk.Buffer
	drawCommandMemory vk.DeviceMemory
	drawCommands      []DrawCommand

	chunks        map[string]*Chunk
	visibleChunks map[string]bool
	orderedChunks []*Chunk

	heightmaps map[string]*image.Gray16
	watermaps  map[string]*image.Gray
}

type DrawCommand struct {
	IndexCount    uint32
	InstanceCount uint32
	FirstIndex    uint32
	VertexOffset  int32
	FirstInstance uint32
}

func (cm *ChunkManager) UpdateLastPosition(pos lin.Vec3) {
	if cm.lastPosition[0] == 0.0 { // TODO Test if statement, make dynamic
		cm.LoadMaps("s55w070")
		log.Printf("Updating camera position to %v\n", pos)
	}
	cm.lastPosition = pos
}

func NewChunkManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties) *ChunkManager {
	cm := &ChunkManager{
		device:        device,
		memProps:      memProps,
		lastPosition:  lin.Vec3{0.0, 0.0, 0.0},
		drawCommands:  make([]DrawCommand, 0),
		chunks:        make(map[string]*Chunk),
		visibleChunks: make(map[string]bool),
		orderedChunks: make([]*Chunk, 0),
		heightmaps:    make(map[string]*image.Gray16, NUM_MAPS),
		watermaps:     make(map[string]*image.Gray, NUM_MAPS),
	}

	cm.InitBuffers()

	return cm
}

// updates the draw commands based on the current visible chunks
func (cm *ChunkManager) updateDrawCommands() {
	cm.drawCommands = make([]DrawCommand, 0, len(cm.visibleChunks))
	var baseVertex int32 = 0
	var baseIndex uint32 = 0

	for _, chunk := range cm.orderedChunks {
		chunkKey := ChunkKey(chunk.X, chunk.Z)
		if cm.visibleChunks[chunkKey] {
			cmd := ChunkToDrawCommand(chunk, baseVertex, baseIndex)
			cm.drawCommands = append(cm.drawCommands, cmd)
		}

		baseVertex += int32(len(chunk.vertices))
		baseIndex += uint32(len(chunk.indices))
	}
}

// To be called after updating the drawcommand array
func (cm *ChunkManager) updateDrawCommandBuffer() {
	drawCommandData := make([]byte, len(cm.drawCommands)*int(unsafe.Sizeof(DrawCommand{})))
	for i, cmd := range cm.drawCommands {
		binary.LittleEndian.PutUint32(drawCommandData[i*20+0:], cmd.IndexCount)
		binary.LittleEndian.PutUint32(drawCommandData[i*20+4:], cmd.InstanceCount)
		binary.LittleEndian.PutUint32(drawCommandData[i*20+8:], cmd.FirstIndex)
		binary.LittleEndian.PutUint32(drawCommandData[i*20+12:], uint32(cmd.VertexOffset))
		binary.LittleEndian.PutUint32(drawCommandData[i*20+16:], cmd.FirstInstance)
	}

	var pDrawCommandData unsafe.Pointer
	ret := vk.MapMemory(*cm.device, cm.drawCommandMemory, 0, vk.DeviceSize(len(drawCommandData)), 0, &pDrawCommandData)
	if ret != vk.Success {
		log.Printf("Failed to map draw command memory: %v", ret)
		return
	}
	n := vk.Memcopy(pDrawCommandData, drawCommandData)
	if n != len(drawCommandData) {
		log.Printf("Failed to copy draw command data: %d != %d", n, len(drawCommandData))
	}
	vk.UnmapMemory(*cm.device, cm.drawCommandMemory)
}

// Loads all chunks referenced by the current map(s)
func (cm *ChunkManager) LoadChunks() {
	if len(cm.heightmaps) == 0 || len(cm.watermaps) == 0 {
		log.Println("No maps loaded. Load maps before calling LoadChunks.")
		return
	}

	for mapCoord := range cm.heightmaps {
		log.Printf("Loading map %s\n", mapCoord)

		// Convert map coordinate to world coordinates
		mapX, mapZ := CoordinateMapToInt(mapCoord)
		log.Printf("Map coordinate: %d, %d\n", mapX, mapZ)

		counter := 0
		for x := mapX; x < mapX+MAP_LENGTH; x += CHUNK_SIZE {
			// Iterate from top right
			for z := mapZ; z < mapZ+MAP_LENGTH; z += CHUNK_SIZE {
				cm.LoadChunk(x, z)
				counter++
			}
			log.Printf("Loading chunks... (%d / %d)\n", counter, CHUNKS_PER_MAP)
		}
	}

	log.Printf("Loaded %d chunks from %d maps", len(cm.chunks), len(cm.heightmaps))
}

// Test function to fill the buffers with all current chunks and render them all
func (cm *ChunkManager) FillBuffers() {
	if cm.vertexBuffer == vk.NullBuffer || cm.indexBuffer == vk.NullBuffer || cm.drawCommandBuffer == vk.NullBuffer {
		log.Println("Buffers not initialized. Call InitBuffers first.")
		return
	}

	if len(cm.chunks) == 0 {
		log.Println("No data loaded, failed to add chunks to buffers.")
		return
	}

	var totalVertices, totalIndices uint32
	drawCommands := make([]DrawCommand, 0, len(cm.chunks))

	// First pass: calculate total sizes and prepare draw commands
	for _, chunk := range cm.orderedChunks {
		vertexCount := uint32(len(chunk.vertices))
		indexCount := uint32(len(chunk.indices))

		drawCommands = append(drawCommands, DrawCommand{
			IndexCount:    indexCount,
			InstanceCount: 1,
			FirstIndex:    totalIndices,
			VertexOffset:  int32(totalVertices),
			FirstInstance: 0,
		})

		totalVertices += vertexCount
		totalIndices += indexCount
	}
	cm.drawCommands = drawCommands

	// Prepare vertex data
	vertexData := make([]byte, totalVertices*uint32(VertexSize))
	log.Printf("Vertex Data size: %d\n", len(vertexData))
	var vertexOffset uint32
	for _, chunk := range cm.orderedChunks {
		for _, vertex := range chunk.vertices {
			copy(vertexData[vertexOffset:], vertex.Data())
			vertexOffset += uint32(VertexSize)
		}
	}

	// Update vertex buffer
	var pVertexData unsafe.Pointer
	ret := vk.MapMemory(*cm.device, cm.vertexMemory, 0, vk.DeviceSize(len(vertexData)), 0, &pVertexData)
	if ret != vk.Success {
		log.Printf("Failed to map vertex memory: %v", ret)
		panic(as.NewError(ret))
	}
	n := vk.Memcopy(pVertexData, vertexData)
	if n != len(vertexData) {
		log.Printf("Failed to copy vertex data: %d != %d", n, len(vertexData))
	}
	vk.UnmapMemory(*cm.device, cm.vertexMemory)

	// Prepare index data
	indexData := make([]byte, totalIndices*uint32(IndexSize))
	var indexOffset uint32
	for _, chunk := range cm.orderedChunks {
		for _, index := range chunk.indices {
			copy(indexData[indexOffset:], index.Data())
			indexOffset += uint32(IndexSize)
		}
	}

	// Update index buffer
	log.Printf("Index Data size: %d\n", len(indexData))
	var pIndexData unsafe.Pointer
	ret = vk.MapMemory(*cm.device, cm.indexMemory, 0, vk.DeviceSize(len(indexData)), 0, &pIndexData)
	if ret != vk.Success {
		log.Printf("Failed to map index memory: %v", ret)
		return
	}
	n = vk.Memcopy(pIndexData, indexData)
	if n != len(indexData) {
		log.Printf("Failed to copy index data: %d != %d", n, len(indexData))
	}
	vk.UnmapMemory(*cm.device, cm.indexMemory)

	// Prepare draw command data
	maxDrawCommands := MAX_CHUNKS
	drawCommandData := make([]byte, maxDrawCommands*int(unsafe.Sizeof(DrawCommand{})))
	drawCommandBuffer := as.CreateBuffer(*cm.device, *cm.memProps, drawCommandData, BufferUsageIndirectBufferBit)
	cm.drawCommandBuffer = drawCommandBuffer.Buffer
	cm.drawCommandMemory = drawCommandBuffer.Memory

	// Update draw command buffer
	log.Printf("DrawCommand Data size: %d\n", len(drawCommandData))
	var pDrawCommandData unsafe.Pointer
	ret = vk.MapMemory(*cm.device, cm.drawCommandMemory, 0, vk.DeviceSize(len(drawCommandData)), 0, &pDrawCommandData)
	if ret != vk.Success {
		log.Printf("Failed to map draw command memory: %v", ret)
		return
	}
	n = vk.Memcopy(pDrawCommandData, drawCommandData)
	if n != len(drawCommandData) {
		log.Printf("Failed to copy draw command data: %d != %d", n, len(drawCommandData))
	}
	vk.UnmapMemory(*cm.device, cm.drawCommandMemory)

	cm.updateDrawCommandBuffer()
}

func ChunkToDrawCommand(chunk *Chunk, baseVertex int32, baseIndex uint32) DrawCommand {
	return DrawCommand{
		IndexCount:    uint32(len(chunk.indices)),
		InstanceCount: 1,
		FirstIndex:    baseIndex,
		VertexOffset:  baseVertex,
		FirstInstance: 0,
	}
}

// Intializes the ChunkManager buffers
func (cm *ChunkManager) InitBuffers() {
	// Create vertex buffer
	maxVertices := MAX_CHUNKS * VERTICES_PER_CHUNK
	vertexData := make([]byte, maxVertices*VertexSize)
	log.Printf("Allocating (%d * %d) * %dB = %d Bytes for vertices\n", MAX_CHUNKS, VERTICES_PER_CHUNK, VertexSize, len(vertexData))

	vertexBuf := as.CreateBuffer(*cm.device, *cm.memProps, vertexData, BufferUsageVertexBufferBit)
	cm.vertexBuffer = vertexBuf.Buffer
	cm.vertexMemory = vertexBuf.Memory

	// Create index buffer
	maxIndices := MAX_CHUNKS * INDICES_PER_CHUNK
	indexData := make([]byte, maxIndices*IndexSize)
	log.Printf("Allocating (%d * %d) * %dB = %d Bytes for indices\n", MAX_CHUNKS, INDICES_PER_CHUNK, IndexSize, len(indexData))

	indexBuf := as.CreateBuffer(*cm.device, *cm.memProps, indexData, BufferUsageIndexBufferBit)
	cm.indexBuffer = indexBuf.Buffer
	cm.indexMemory = indexBuf.Memory

	// Create the draw command buffer
	maxDrawCommands := MAX_CHUNKS * int(unsafe.Sizeof(DrawCommand{}))
	drawCommandData := make([]byte, maxDrawCommands)
	log.Printf("Allocating %d * %dB = %d Bytes for Draw commands\n", MAX_CHUNKS, int(unsafe.Sizeof(DrawCommand{})), maxDrawCommands)
	drawCommandBuffer := as.CreateBuffer(*cm.device, *cm.memProps, drawCommandData, BufferUsageIndirectBufferBit)
	cm.drawCommandBuffer = drawCommandBuffer.Buffer
	cm.drawCommandMemory = drawCommandBuffer.Memory
}

// Loads both maps associated with the coordinate
func (cm *ChunkManager) LoadMaps(coord string) error {
	err := cm.LoadHeightmap(coord)
	if err != nil {
		return err
	}

	err = cm.LoadWatermap(coord)
	if err != nil {
		return err
	}

	return nil
}

// Loads a watermap (coord.swb.png) into the ChunkManager
func (cm *ChunkManager) LoadWatermap(coord string) error {
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

	cm.watermaps[coord] = grayImg

	return nil
}

// Loads a heightmap (coord.hgt.png) into the ChunkManager
func (cm *ChunkManager) LoadHeightmap(coord string) error {
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

	cm.heightmaps[coord] = grayImg

	return nil
}

// Unloads both the water and heightmaps
func (cm *ChunkManager) UnloadMaps(coord string) {
	cm.unloadWatermap(coord)
	cm.unloadHeightmap(coord)
}

// Unloads a watermap (coord) from the ChunkManager
func (cm *ChunkManager) unloadWatermap(coord string) {
	delete(cm.watermaps, coord)
}

// Unloads a heightmap (coord) from the ChunkManager
func (cm *ChunkManager) unloadHeightmap(coord string) {
	delete(cm.heightmaps, coord)
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

func (cm *ChunkManager) Destroy() {
	vk.DestroyBuffer(*cm.device, cm.vertexBuffer, nil)
	vk.FreeMemory(*cm.device, cm.vertexMemory, nil)

	vk.DestroyBuffer(*cm.device, cm.indexBuffer, nil)
	vk.FreeMemory(*cm.device, cm.indexMemory, nil)
}
