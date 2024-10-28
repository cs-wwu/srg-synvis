package renderer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"unsafe"

	as "github.com/cs-wwu/srg-synvis/asche"
	mo "github.com/cs-wwu/srg-synvis/meshopt/meshoptimizer"

	// as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file should be used for chunk management, chunk rendering, etc.
// TODO maybe save raw arrays rather than image pointers?

const (
	WATERMAP_EXTENSION  = ".swb.png"
	HEIGHTMAP_EXTENSION = ".hgt.png"

	MAP_LENGTH = 3600
	NUM_MAPS   = 4 // TODO change to 9 in final version? (after LOD)

	WORLD_WIDTH  = 360 * int(ARC_DEGREES_TO_SECONDS)
	WORLD_HEIGHT = 180 * int(ARC_DEGREES_TO_SECONDS)

	VERTICES_PER_MAP = (MAP_LENGTH + 1) * (MAP_LENGTH + 1)
	INDICES_PER_MAP  = MAP_LENGTH * MAP_LENGTH * 6

	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds

	LOD_LEVELS             = 3
	LOD_DISTANCE_THRESHOLD = 500.0
)

// A single TerrainManager to manage load/unloading images and chunks
type TerrainManager struct {
	device   *vk.Device
	memProps *vk.PhysicalDeviceMemoryProperties

	vertices []Vertex
	indices  []uint32 // Identical to type Index

	lastPosition lin.Vec3

	vertexBuffer      vk.Buffer
	vertexMemory      vk.DeviceMemory
	indexBuffer       vk.Buffer
	indexMemory       vk.DeviceMemory
	drawCommandBuffer vk.Buffer
	drawCommandMemory vk.DeviceMemory

	heightmaps map[string]*image.Gray16
	watermaps  map[string]*image.Gray
	mapData    map[string]*MapGeometry

	drawCommand DrawCommand
}

type MapGeometry struct {
	vertexOffset uint32
	indexOffset  uint32
	indexCount   uint32
	mapX, mapZ   int
}

type DrawCommand struct {
	IndexCount    uint32
	InstanceCount uint32
	FirstIndex    uint32
	VertexOffset  int32
	FirstInstance uint32
}

func (tm *TerrainManager) UpdateLastPosition(pos lin.Vec3) {
	if tm.lastPosition[0] == 0.0 { // TODO Make dynamic based on location
		tm.LoadMap("n48w123")
		tm.LoadMap("n48w122")
		// tm.LoadMap("n47w123")
		// tm.LoadMap("n47w122")
		log.Printf("Updating camera position to %v\n", pos)
	}
	tm.lastPosition = pos
}

func NewTerrainManager(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties) *TerrainManager {
	tm := &TerrainManager{
		device:       device,
		memProps:     memProps,
		lastPosition: lin.Vec3{0.0, 0.0, 0.0},
		heightmaps:   make(map[string]*image.Gray16, NUM_MAPS),
		watermaps:    make(map[string]*image.Gray, NUM_MAPS),
		mapData:      make(map[string]*MapGeometry),
		drawCommand:  DrawCommand{},
	}

	tm.InitBuffers()

	return tm
}

// Generate vertices for a single heightmap
func (tm *TerrainManager) generateVertices(heightmap *image.Gray16, watermap *image.Gray,
	mapX, mapZ int) []Vertex {

	vertices := make([]Vertex, (MAP_LENGTH+1)*(MAP_LENGTH+1))

	for z := MAP_LENGTH; z >= 0; z-- {
		worldZ := mapZ + z
		for x := 0; x <= MAP_LENGTH; x++ {
			worldX := mapX + x

			pixelValue := heightmap.Gray16At(x, z).Y
			height := float32(int(pixelValue) - 32768)
			isWater := watermap.GrayAt(x, z).Y

			idx := z*(MAP_LENGTH+1) + x
			vertices[idx] = Vertex{
				Position: lin.Vec3{float32(worldX), height * HEIGHT_SCALE, float32(worldZ)},
				TexCoord: lin.Vec3{
					float32(x) / float32(MAP_LENGTH),
					float32(z) / float32(MAP_LENGTH),
					float32(isWater),
				},
			}
		}
	}

	return vertices
}

// Generate indices for a heightmap section
func (tm *TerrainManager) generateIndices(baseVertex uint32) []uint32 {
	indices := make([]uint32, MAP_LENGTH*MAP_LENGTH*6)
	idx := 0

	for z := 0; z < MAP_LENGTH; z++ {
		for x := 0; x < MAP_LENGTH; x++ {
			// Calculate the indices for the quad, offset by baseVertex
			topLeft := baseVertex + uint32(z*(MAP_LENGTH+1)+x)
			topRight := topLeft + 1
			bottomLeft := topLeft + uint32(MAP_LENGTH+1)
			bottomRight := bottomLeft + 1

			// First triangle
			indices[idx] = topLeft
			indices[idx+1] = bottomLeft
			indices[idx+2] = bottomRight

			// Second triangle
			indices[idx+3] = topLeft
			indices[idx+4] = bottomRight
			indices[idx+5] = topRight

			idx += 6
		}
	}

	return indices
}

func (tm *TerrainManager) calculateNormals(vertices []Vertex, startIdx int) {
	width := MAP_LENGTH + 1

	for z := 0; z < MAP_LENGTH; z++ {
		for x := 0; x < MAP_LENGTH; x++ {
			idx := startIdx + z*width + x

			// Get vertices for normal calculation
			current := vertices[idx].Position
			right := vertices[idx+1].Position
			down := vertices[idx+width].Position

			// Calculate normal using cross product
			var normal, a, b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			vertices[idx].Normal = normal
		}
	}
}

// LoadMap loads a heightmap and its associated watermap
func (tm *TerrainManager) LoadMap(coord string) error {
	fmt.Printf("Loading map: %s\n", coord)
	// Skip if already loaded
	if _, exists := tm.mapData[coord]; exists {
		return nil
	}

	// Load heightmap and watermap
	if err := tm.LoadHeightmap(coord); err != nil {
		return err
	}
	if err := tm.LoadWatermap(coord); err != nil {
		return err
	}

	mapX, mapZ := CoordinateMapToInt(coord)

	// Generate vertices for this map
	fmt.Println("Generating Geometry...")
	vertices := tm.generateVertices(tm.heightmaps[coord], tm.watermaps[coord], mapX, mapZ)
	vertexOffset := uint32(len(tm.vertices))
	tm.vertices = append(tm.vertices, vertices...)

	// Generate indices for this map
	indices := tm.generateIndices(vertexOffset)
	indexOffset := uint32(len(tm.indices))
	tm.indices = append(tm.indices, indices...)

	// Calculate normals for the new vertices
	tm.calculateNormals(tm.vertices, int(vertexOffset))

	// Store map data
	tm.mapData[coord] = &MapGeometry{
		vertexOffset: vertexOffset,
		indexOffset:  indexOffset,
		indexCount:   uint32(len(indices)),
		mapX:         mapX,
		mapZ:         mapZ,
	}

	// Optimize the combined geometry
	tm.optimizeGeometry()

	// Update GPU buffers
	tm.FillBuffers()

	return nil
}

func (tm *TerrainManager) optimizeGeometry() {
	if len(tm.vertices) == 0 {
		return
	}

	fmt.Println("Generating Vertex Remap...")
	// Step 1: Generate vertex remap
	remap := make([]uint32, len(tm.vertices))
	vertexCount := mo.GenerateVertexRemap(
		remap,
		tm.indices,
		uint64(len(tm.indices)),
		unsafe.Pointer(&tm.vertices[0]),
		uint64(len(tm.vertices)),
		uint64(VertexSize),
	)

	// Step 2: Create temporary buffers for optimized data
	optimizedVertices := make([]Vertex, vertexCount)
	optimizedIndices := make([]uint32, len(tm.indices))

	// Step 3: Remap vertices
	fmt.Println("Remapping Vertex Buffer...")
	mo.RemapVertexBuffer(
		unsafe.Pointer(&optimizedVertices[0]),
		unsafe.Pointer(&tm.vertices[0]),
		uint64(len(tm.vertices)),
		uint64(VertexSize),
		remap,
	)

	// Step 4: Remap indices
	fmt.Println("Remapping Index Buffer...")
	mo.RemapIndexBuffer(
		optimizedIndices,
		tm.indices,
		uint64(len(tm.indices)),
		remap,
	)

	// Step 5: Optimize vertex cache
	fmt.Println("Optimizing Vertex Cache...")
	mo.OptimizeVertexCache(
		optimizedIndices,
		optimizedIndices,
		uint64(len(optimizedIndices)),
		uint64(len(optimizedVertices)),
	)

	// Step 6: Optimize vertex fetch
	fmt.Println("Optimizing Vertex Fetch...")
	mo.OptimizeVertexFetch(
		unsafe.Pointer(&optimizedVertices[0]),
		optimizedIndices,
		uint64(len(optimizedIndices)),
		unsafe.Pointer(&optimizedVertices[0]),
		uint64(len(optimizedVertices)),
		uint64(VertexSize),
	)

	// Replace original data with optimized data
	tm.vertices = optimizedVertices
	tm.indices = optimizedIndices

	// Update draw command
	tm.drawCommand = DrawCommand{
		IndexCount:    uint32(len(tm.indices)),
		InstanceCount: 1,
		FirstIndex:    0,
		VertexOffset:  0,
		FirstInstance: 0,
	}
}

// FillBuffers updates GPU buffers with current geometry
func (tm *TerrainManager) FillBuffers() {
	fmt.Println("Filling Buffers...")
	if len(tm.vertices) == 0 || len(tm.indices) == 0 {
		return
	}

	// Fill vertex buffer
	vertexData := make([]byte, len(tm.vertices)*VertexSize)
	for i, vertex := range tm.vertices {
		copy(vertexData[i*VertexSize:], vertex.Data())
	}

	var pVertexData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.vertexMemory, 0, vk.DeviceSize(len(vertexData)), 0, &pVertexData)
	vk.Memcopy(pVertexData, vertexData)
	vk.UnmapMemory(*tm.device, tm.vertexMemory)

	// Fill index buffer
	indexData := make([]byte, len(tm.indices)*4)
	for i, index := range tm.indices {
		binary.LittleEndian.PutUint32(indexData[i*4:], index)
	}

	var pIndexData unsafe.Pointer
	vk.MapMemory(*tm.device, tm.indexMemory, 0, vk.DeviceSize(len(indexData)), 0, &pIndexData)
	vk.Memcopy(pIndexData, indexData)
	vk.UnmapMemory(*tm.device, tm.indexMemory)

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
	vertexData := make([]byte, VERTICES_PER_MAP*VertexSize*NUM_MAPS)
	vertexBuf := as.CreateBuffer(*tm.device, *tm.memProps, vertexData, BufferUsageVertexBufferBit)
	tm.vertexBuffer = vertexBuf.Buffer
	tm.vertexMemory = vertexBuf.Memory

	// Index buffer
	indexData := make([]byte, INDICES_PER_MAP*4*NUM_MAPS) // uint32 = 4 bytes
	indexBuf := as.CreateBuffer(*tm.device, *tm.memProps, indexData, BufferUsageIndexBufferBit)
	tm.indexBuffer = indexBuf.Buffer
	tm.indexMemory = indexBuf.Memory

	// Draw command buffer (single command)
	drawCommandData := make([]byte, unsafe.Sizeof(DrawCommand{}))
	drawCommandBuffer := as.CreateBuffer(*tm.device, *tm.memProps, drawCommandData, BufferUsageIndirectBufferBit)
	tm.drawCommandBuffer = drawCommandBuffer.Buffer
	tm.drawCommandMemory = drawCommandBuffer.Memory
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
