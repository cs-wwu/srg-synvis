package chunk

import (
	"encoding/binary"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/coords"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// Nodes larger than this are not meshed.
const MAX_CHUNK_SIZE = 8192

// Leaves are not split to be smaller than this. Also, the number of vertices
// in every chunk.
const MIN_CHUNK_SIZE = 64

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

	mapCoord := coords.WorldToMap(int(chunk.X), int(chunk.Z))
	heightInfo := NewHeightInfo(c.AssetManager, mapCoord)
	chunk.calculateVertices(heightInfo)
	chunk.calculateIndices()
	chunk.calculateNormals()

	return chunk.upload(c.device, c.memProps)
}

// Calculate vertices and texcoords for a chunk
func (chunk *Chunk) calculateVertices(heightInfo *HeightInfo) {
	stride := int(chunk.size / MIN_CHUNK_SIZE)
	const VERTICES_PER_STRIDE = MIN_CHUNK_SIZE + 1

	chunk.Vertices = make([]Vertex, VERTICES_PER_STRIDE*VERTICES_PER_STRIDE)

	for z := range VERTICES_PER_STRIDE {
		worldZ := chunk.Z + (z * stride) - 1

		for x := range VERTICES_PER_STRIDE {
			worldX := chunk.X + (x * stride) - 1

			height := heightInfo.Height(float32(worldX), float32(worldZ))

			var waterVal float32
			if heightInfo.IsWater(worldX, worldZ) {
				waterVal = 1.0
			}

			localX, localZ := coords.WorldToPixel(worldX, worldZ)

			idx := z*VERTICES_PER_STRIDE + x
			chunk.Vertices[idx] = Vertex{
				Position: lin.Vec3{float32(worldX), height, float32(worldZ)},
				TexCoord: lin.Vec3{
					float32(localX) / float32(MAP_LENGTH),
					float32(localZ) / float32(MAP_LENGTH),
					waterVal,
				},
			}
		}
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

func (chunk *Chunk) upload(device vk.Device, memProps vk.PhysicalDeviceMemoryProperties) (out renderChunk) {
	// Vertex buffer
	vertexData := make([]byte, len(chunk.Vertices)*VertexSize)
	for i, vertex := range chunk.Vertices {
		copy(vertexData[i*VertexSize:], vertex.Data())
	}
	vb := as.CreateBuffer(device, memProps, vertexData, vk.BufferUsageVertexBufferBit)
	out.vertexBuffer = vb

	// Index buffer
	indexData := make([]byte, len(chunk.Indices)*IndexSize)
	for i, index := range chunk.Indices {
		binary.LittleEndian.PutUint32(indexData[i*IndexSize:], uint32(index))
	}
	ib := as.CreateBuffer(device, memProps, indexData, vk.BufferUsageIndexBufferBit)
	out.indexBuffer = ib

	out.indexCount = uint32(len(chunk.Indices))
	return
}
