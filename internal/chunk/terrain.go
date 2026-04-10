package chunk

import (
	"encoding/binary"
	"fmt"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/coords"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

type LODLevel int

// Nodes larger than this are not meshed.
const MAX_CHUNK_SIZE = 8192

// Leaves are not split to be smaller than this. Also, the number of vertices
// in every chunk.
const MIN_CHUNK_SIZE = 64

const (
	LOD_0 LODLevel = iota // Full detail
	LOD_1                 // Half detail
	LOD_2                 // Quarter detail
	LOD_3                 // Eighth detail
	LOD_4                 // Sixteenth detail
	LOD_COUNT
)

type Chunk struct {
	Vertices []Vertex
	Indices  []Index
	X        int  // Global X Position of the top-left chunk corner
	Z        int  // Global Z Position of the top-left chunk corner
	size     uint // width/height in grid cells. always a power of 2
}

func (c *ChunkManager) buildChunk(pos spatial.GridCoord, size uint) renderChunk {
	if size > MAX_CHUNK_SIZE {
		return renderChunk{}
	}

	chunk := &Chunk{
		X:    pos.X,
		Z:    pos.Z,
		size: size,
	}

	mapData, err := c.AssetManager.LoadMapData(
		coords.WorldToMap(int(chunk.X), int(chunk.Z)),
	)
	if err != nil {
		fmt.Printf("Failed to load map data: %v\n", err)
		return renderChunk{}
	}

	chunk.calculateVertices(NewHeightInfo(mapData.HeightData), mapData.WaterData)
	chunk.calculateIndices()
	chunk.calculateNormals()

	out, err := chunk.upload(c.device, c.memProps)
	if err != nil {
		fmt.Printf("Failed to upload chunk: %v\n", err)
		return renderChunk{}
	}

	return out
}

// Loads a specific chunk into memory, x and z are the upper-left corner of
// the chunk (world is in arcsecond coordinates)
// Note that the world is Latitude by Longitude times 3600 (arcseconds):
//
//	<-    3600 * 360   ->
//	--------------------- ▲
//	|      World        | 3600 * 180
//	--------------------- ▼

func ChunkKey(chunkX, chunkZ int) string {
	return fmt.Sprintf("%d,%d", chunkX, chunkZ)
}

// Calculate vertices and texcoords for a chunk
func (chunk *Chunk) calculateVertices(heightInfo *HeightInfo, waterData []uint8) {
	// Calculate vertex stride based on LOD
	stride := int(chunk.size / MIN_CHUNK_SIZE)
	const VERTICES_PER_STRIDE = (MIN_CHUNK_SIZE) + 1 // +1 for overlap on right/bottom edges

	// Allocate vertex array based on LOD level
	chunk.Vertices = make([]Vertex, VERTICES_PER_STRIDE*VERTICES_PER_STRIDE)
	firstZ := true

	for z := range VERTICES_PER_STRIDE {
		worldZ := chunk.Z + (z * stride) - 1 // Start one vertex north (subtract a point)
		firstX := true

		for x := range VERTICES_PER_STRIDE {
			worldX := chunk.X + (x * stride) - 1 // Start one vertex west (subtract a point)
			localX, localZ := coords.WorldToPixel(worldX, worldZ)

			if firstZ && (worldZ%3600 == -1 || worldZ%3600 == 1) {
				localZ -= 3599
			}
			if firstX && (worldX%3600 == -1 || worldX%3600 == 1) {
				localX -= 3599
			}

			// Direct array access - much faster than method image
			height := heightInfo.getLocal(localX, localZ)
			isWater := waterData[localZ*MAP_LENGTH+localX]

			idx := z*VERTICES_PER_STRIDE + x
			chunk.Vertices[idx] = Vertex{
				Position: lin.Vec3{float32(worldX), height, float32(worldZ)},
				TexCoord: lin.Vec3{
					float32(localX) / float32(MAP_LENGTH),
					float32(localZ) / float32(MAP_LENGTH),
					float32(isWater),
				},
			}

			firstX = false
		}
		firstZ = false
	}
}

// Generate indices (triangle strip)c
func (chunk *Chunk) calculateIndices() {
	const VERTICES_PER_STRIDE = (MIN_CHUNK_SIZE) + 1 // +1 for overlap on right/bottom edges
	numStrips := VERTICES_PER_STRIDE - 1
	verticesPerStrip := VERTICES_PER_STRIDE * 2
	degenerateVertices := (numStrips - 1) * 2

	chunk.Indices = make([]Index, (numStrips*verticesPerStrip)+degenerateVertices)
	indexPos := 0

	for z := range VERTICES_PER_STRIDE - 1 {
		if z > 0 {
			// Degenerate triangle to connect strips
			chunk.Indices[indexPos] = Index(z * VERTICES_PER_STRIDE)
			indexPos++
		}

		for x := range VERTICES_PER_STRIDE {
			chunk.Indices[indexPos] = Index(z*VERTICES_PER_STRIDE + x)
			chunk.Indices[indexPos+1] = Index((z+1)*VERTICES_PER_STRIDE + x)
			indexPos += 2
		}

		if z < VERTICES_PER_STRIDE-2 {
			// Degenerate triangle for next strip
			chunk.Indices[indexPos] = Index((z+1)*VERTICES_PER_STRIDE + (VERTICES_PER_STRIDE - 1))
			indexPos++
		}
	}
}

// Calculate normals for the chunk
func (chunk *Chunk) calculateNormals() {
	const VERTICES_PER_STRIDE = (MIN_CHUNK_SIZE) + 1 // +1 for overlap on right/bottom edges

	// For each vertex except the last row and column
	for z := range VERTICES_PER_STRIDE - 1 {
		for x := range VERTICES_PER_STRIDE - 1 {
			idx := z*VERTICES_PER_STRIDE + x
			current := chunk.Vertices[idx].Position
			right := chunk.Vertices[idx+1].Position
			down := chunk.Vertices[idx+VERTICES_PER_STRIDE].Position

			var normal, a, b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			chunk.Vertices[idx].Normal = normal
		}
	}

	// Handle right edge - use the normal from the vertex to the left
	for z := range VERTICES_PER_STRIDE - 1 {
		idx := z*VERTICES_PER_STRIDE + (VERTICES_PER_STRIDE - 1)
		chunk.Vertices[idx].Normal = chunk.Vertices[idx-1].Normal
	}

	// Handle bottom edge - use the normal from the vertex above
	for x := range VERTICES_PER_STRIDE {
		idx := (VERTICES_PER_STRIDE-1)*VERTICES_PER_STRIDE + x
		chunk.Vertices[idx].Normal = chunk.Vertices[idx-VERTICES_PER_STRIDE].Normal
	}

	// Handle bottom-right corner - use normal from vertex to the left
	lastIdx := (VERTICES_PER_STRIDE * VERTICES_PER_STRIDE) - 1
	chunk.Vertices[lastIdx].Normal = chunk.Vertices[lastIdx-1].Normal
}

func (chunk *Chunk) upload(device vk.Device, memProps vk.PhysicalDeviceMemoryProperties) (out renderChunk, err error) {
	// Vertex buffer
	vertexData := make([]byte, len(chunk.Vertices)*VertexSize)
	for i, vertex := range chunk.Vertices {
		copy(vertexData[i*VertexSize:], vertex.Data())
	}
	vb := as.CreateBuffer(device, memProps, vertexData, vk.BufferUsageVertexBufferBit)
	out.vertexBuffer = vb.Buffer

	// Index buffer
	indexData := make([]byte, len(chunk.Indices)*IndexSize)
	for i, index := range chunk.Indices {
		binary.LittleEndian.PutUint32(indexData[i*IndexSize:], uint32(index))
	}
	ib := as.CreateBuffer(device, memProps, indexData, vk.BufferUsageIndexBufferBit)
	out.indexBuffer = ib.Buffer

	out.indexCount = uint32(len(chunk.Indices))
	return
}
