package renderer

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"

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
	CHUNKS_PER_SIDE = 60
	CHUNKS_PER_MAP  = CHUNKS_PER_SIDE * CHUNKS_PER_SIDE
	CHUNK_SIZE      = MAP_LENGTH / CHUNKS_PER_SIDE
	NUM_MAPS        = 4 // TODO change to 9 in final version
	MAX_CHUNKS      = NUM_MAPS * CHUNKS_PER_MAP

	WORLD_WIDTH  = 360 * int(ARC_DEGREES_TO_SECONDS)
	WORLD_HEIGHT = 180 * int(ARC_DEGREES_TO_SECONDS)
	CHUNKS_X     = WORLD_WIDTH / CHUNK_SIZE
	CHUNKS_Z     = WORLD_HEIGHT / CHUNK_SIZE

	VERTICES_PER_CHUNK = (CHUNK_SIZE + 1) * (CHUNK_SIZE + 1)
	INDICES_PER_CHUNK  = 6 * CHUNK_SIZE * CHUNK_SIZE

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

	chunks        map[string]*Chunk
	visibleChunks []*Chunk

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
	if cm.lastPosition == (lin.Vec3{0.0, 0.0, 0.0}) { // TODO Test if statement
		cm.LoadMaps("n48w123")
		log.Printf("Updating camera position to %v\n", pos)
	}
	cm.lastPosition = pos
}

func NewChunkManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties) *ChunkManager {
	cm := &ChunkManager{
		device:        device,
		memProps:      memProps,
		lastPosition:  lin.Vec3{0.0, 0.0, 0.0},
		visibleChunks: make([]*Chunk, 0),
		chunks:        make(map[string]*Chunk),
		heightmaps:    make(map[string]*image.Gray16, NUM_MAPS),
		watermaps:     make(map[string]*image.Gray, NUM_MAPS),
	}

	cm.InitBuffers()

	return cm
}

// Test function to fill the buffers with all current chunks and render them all
func (cm *ChunkManager) FillBuffers() {

}

// Intializes the ChunkManager buffers
func (cm *ChunkManager) InitBuffers() {
	// Create vertex buffer
	maxVertices := MAX_CHUNKS * VERTICES_PER_CHUNK
	vertexData := make([]byte, maxVertices*VertexSize)

	vertexBuf := as.CreateBuffer(*cm.device, *cm.memProps, vertexData, BufferUsageVertexBufferBit)
	cm.vertexBuffer = vertexBuf.Buffer
	cm.vertexMemory = vertexBuf.Memory

	// Create index buffer
	maxIndices := MAX_CHUNKS * INDICES_PER_CHUNK
	indexData := make([]byte, maxIndices*IndexSize)

	indexBuf := as.CreateBuffer(*cm.device, *cm.memProps, indexData, BufferUsageIndexBufferBit)
	cm.indexBuffer = indexBuf.Buffer
	cm.indexMemory = indexBuf.Memory

	// Create the draw command buffer
	drawCommandData := make([]byte, MAX_CHUNKS)
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
	if width != length || height != length {
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
	mapCoords := CoordinateFloatToMap(chunkX, chunkZ)
	heightmap := cm.heightmaps[mapCoords]
	watermap := cm.watermaps[mapCoords]

	// Load the chunk based on heightmap and watermap
	// TODO load water if missing heightmap and watermap
	if heightmap == nil || watermap == nil {
		log.Printf("Warning: Heightmap or Watermap not loaded for coordinates %s\n", mapCoords)
		return
	}

	chunkKey := ChunkKey(chunkX, chunkZ)
	if _, exists := cm.chunks[chunkKey]; exists {
		return // Chunk already exists
	}

	chunk := &Chunk{
		X:        chunkX * ChunkSize,
		Z:        chunkZ * ChunkSize,
		vertices: make([]Vertex, ChunkSize*ChunkSize),
		indices:  make([]Index, 2*ChunkSize*(ChunkSize-1)+2*((ChunkSize-1)-1)),
	}

	calculateChunkVertices(heightmap, watermap, chunk)
	calculateChunkIndices(chunk)
	calculateChunkNormals(chunk)

	cm.chunks[chunkKey] = chunk
}

// Generate vertices and texcoords
func calculateChunkVertices(heightmap *image.Gray16, watermap *image.Gray, chunk *Chunk) {
	// Fill the vertices
	for z := 0; z < ChunkSize; z++ {
		for x := 0; x < ChunkSize; x++ {
			worldX := chunk.X + x
			worldZ := chunk.Z + z
			if (x == 0) && (z == 0) { // TODO remove
				log.Printf("Loading chunk at coord: (%d, %d)\n", worldX, worldZ)
			}
			
			pixelValue := heightmap.Gray16At(worldX, worldZ).Y
			height := float32(int(pixelValue) - 32768)
			isWater := watermap.GrayAt(worldX, worldZ).Y

			chunk.vertices[z*ChunkSize+x] = Vertex{
				Position: lin.Vec3{float32(worldX), height * HEIGHT_SCALE, float32(worldZ)},
				TexCoord: lin.Vec3{
					float32(worldX) / float32(length-1),
					float32(worldZ) / float32(length-1),
					float32(isWater),
				},
			}
		}
	}
}

// Generate indices (triangle strip)
func calculateChunkIndices(chunk *Chunk) {
	indexPos := 0
	for z := 0; z < ChunkSize-1; z++ {
		if z > 0 {
			// Degenerate triangle to connect strips
			chunk.indices[indexPos] = Index(z * ChunkSize)
			indexPos++
		}

		for x := 0; x < ChunkSize; x++ {
			chunk.indices[indexPos] = Index(z*ChunkSize + x)
			chunk.indices[indexPos+1] = Index((z+1)*ChunkSize + x)
			indexPos += 2
		}

		if z < ChunkSize-2 {
			// Degenerate triangle to connect strips
			chunk.indices[indexPos] = Index((z+1)*ChunkSize + (ChunkSize - 1))
			indexPos++
		}
	}
}

// Calculate Chunk Normals
func calculateChunkNormals(chunk *Chunk) {
	for z := 0; z < ChunkSize-1; z++ {
		for x := 0; x < ChunkSize-1; x++ {
			current := chunk.vertices[z*ChunkSize+x].Position
			right := chunk.vertices[z*ChunkSize+(x+1)].Position
			down := chunk.vertices[(z+1)*ChunkSize+x].Position

			var normal, a, b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			chunk.vertices[z*ChunkSize+x].Normal = normal
		}
	}
}
