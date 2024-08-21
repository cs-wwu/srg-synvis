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

	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcminutes
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

	chunks map[string]*Chunk

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
	if cm.lastPosition[0] == 0.0 { // TODO Test if statement
		cm.LoadMaps("n48w123")
		cm.LoadMaps("n48w122")
		log.Printf("Updating camera position to %v\n", pos)
	}
	cm.lastPosition = pos
}

func NewChunkManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties) *ChunkManager {
	cm := &ChunkManager{
		device:       device,
		memProps:     memProps,
		lastPosition: lin.Vec3{0.0, 0.0, 0.0},
		drawCommands: make([]DrawCommand, 0),
		chunks:       make(map[string]*Chunk),
		heightmaps:   make(map[string]*image.Gray16, NUM_MAPS),
		watermaps:    make(map[string]*image.Gray, NUM_MAPS),
	}

	cm.InitBuffers()

	return cm
}

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
	for _, chunk := range cm.chunks {
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
	for _, chunk := range cm.chunks {
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
	for _, chunk := range cm.chunks {
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
	maxDrawComamnds := MAX_CHUNKS * int(unsafe.Sizeof(DrawCommand{}))
	drawCommandData := make([]byte, maxDrawComamnds)
	log.Printf("Allocating %d * %dB = %d Bytes for Draw commands\n", MAX_CHUNKS, int(unsafe.Sizeof(DrawCommand{})), maxDrawComamnds)
	drawCommandBuffer := as.CreateBuffer(*cm.device, *cm.memProps, drawCommandData, BufferUsageIndirectBufferBit)
	cm.drawCommandBuffer = drawCommandBuffer.Buffer
	cm.drawCommandMemory = drawCommandBuffer.Memory
}

func ChunkKey(chunkX, chunkZ int) string {
	return fmt.Sprintf("%d,%d", chunkX, chunkZ)
}

type Chunk struct {
	vertices []Vertex
	indices  []Index
	X        int // Global X Position of the top-left chunk corner
	Z        int // Global Z Position of the top-left chunk corner
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

// Loads a specific chunk into memory, x and z are the upper-left corner of
// the chunk (world is in arcsecond coordinates)
// Note that the world is Latitude by Longitude times 3600 (arcseconds):
//
//	<-    3600 * 360   ->
//	--------------------- ▲
//	|      World        | 3600 * 180
//	--------------------- ▼
func (cm *ChunkManager) LoadChunk(chunkX, chunkZ int) {
	// TODO: stitch surrounding chunks together
	// This should be done by asking if the chunkX, chunkZ exists in the chunk map
	mapCoords := CoordinateIntToMap(chunkX, chunkZ)
	heightmap := cm.heightmaps[mapCoords]
	watermap := cm.watermaps[mapCoords]

	// Load the chunk based on heightmap and watermap
	// TODO load water if missing heightmap and watermap
	if heightmap == nil || watermap == nil {
		log.Printf("Warning: Map(s) not loaded for coords %s (%d, %d)\n", mapCoords, chunkX, chunkZ)
		return
	}

	chunkKey := ChunkKey(chunkX, chunkZ)
	if _, exists := cm.chunks[chunkKey]; exists {
		return // Chunk already exists
	}

	chunk := &Chunk{
		X:        chunkX,
		Z:        chunkZ,
		vertices: make([]Vertex, CHUNK_SIZE*CHUNK_SIZE),
		indices:  make([]Index, 2*CHUNK_SIZE*(CHUNK_SIZE-1)+2*((CHUNK_SIZE-1)-1)),
	}

	calculateChunkVertices(heightmap, watermap, chunk)
	calculateChunkIndices(chunk)
	calculateChunkNormals(chunk)

	cm.chunks[chunkKey] = chunk
}

// Generate vertices and texcoords
func calculateChunkVertices(heightmap *image.Gray16, watermap *image.Gray, chunk *Chunk) {
	// Fill the vertices
	for z := CHUNK_SIZE - 1; z >= 0; z-- {
		worldZ := chunk.Z + z
		for x := 0; x < CHUNK_SIZE; x++ {
			worldX := chunk.X + x
			localX, localZ := CoordinateIntToPixel(worldX, worldZ)

			pixelValue := heightmap.Gray16At(localX, localZ).Y
			height := float32(int(pixelValue) - 32768)
			isWater := watermap.GrayAt(CoordinateIntToPixel(localX, localZ)).Y

			chunk.vertices[z*CHUNK_SIZE+x] = Vertex{
				Position: lin.Vec3{float32(worldX), height * HEIGHT_SCALE, float32(worldZ)},
				TexCoord: lin.Vec3{
					float32(localX) / float32(MAP_LENGTH),
					float32(localZ) / float32(MAP_LENGTH),
					float32(isWater),
				},
			}
		}
	}
}

// Generate indices (triangle strip)
func calculateChunkIndices(chunk *Chunk) {
	indexPos := 0
	for z := 0; z < CHUNK_SIZE-1; z++ {
		if z > 0 {
			// Degenerate triangle to connect strips
			chunk.indices[indexPos] = Index(z * CHUNK_SIZE)
			indexPos++
		}

		for x := 0; x < CHUNK_SIZE; x++ {
			chunk.indices[indexPos] = Index(z*CHUNK_SIZE + x)
			chunk.indices[indexPos+1] = Index((z+1)*CHUNK_SIZE + x)
			indexPos += 2
		}

		if z < CHUNK_SIZE-2 {
			// Degenerate triangle to connect strips
			chunk.indices[indexPos] = Index((z+1)*CHUNK_SIZE + (CHUNK_SIZE - 1))
			indexPos++
		}
	}
}

// Calculate Chunk Normals
func calculateChunkNormals(chunk *Chunk) {
	for z := 0; z < CHUNK_SIZE-1; z++ {
		for x := 0; x < CHUNK_SIZE-1; x++ {
			current := chunk.vertices[z*CHUNK_SIZE+x].Position
			right := chunk.vertices[z*CHUNK_SIZE+(x+1)].Position
			down := chunk.vertices[(z+1)*CHUNK_SIZE+x].Position

			var normal, a, b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			chunk.vertices[z*CHUNK_SIZE+x].Normal = normal
		}
	}
}
