package rendering

import (
	"fmt"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

const (
	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds
	MAP_LENGTH   = 3601
)

func ChunkKey(chunkX, chunkZ int) string {
	return fmt.Sprintf("%d,%d", chunkX, chunkZ)
}

type LODLevel int

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
	X        int // Global X Position of the top-left chunk corner
	Z        int // Global Z Position of the top-left chunk corner
	LODLevel LODLevel
}

type HeightInfo struct {
	heightData *[MAP_LENGTH * MAP_LENGTH]uint16
}

func newHeightInfo(data []uint16) *HeightInfo {
	return &HeightInfo{
		heightData: (*[MAP_LENGTH * MAP_LENGTH]uint16)(data),
	}
}

func (h *HeightInfo) getLocal(x, z int) float32 {
	if (x < 0 || x >= MAP_LENGTH) || (z < 0 || z >= MAP_LENGTH) {
		return 0.0
	}

	value := h.heightData[z*MAP_LENGTH+x]
	// normalize from 0-65535 to -32768 to 32767
	return float32(int(value)-32768) * HEIGHT_SCALE
}

// Loads a specific chunk into memory, x and z are the upper-left corner of
// the chunk (world is in arcsecond coordinates)
// Note that the world is Latitude by Longitude times 3600 (arcseconds):
//
//	<-    3600 * 360   ->
//	--------------------- ▲
//	|      World        | 3600 * 180
//	--------------------- ▼

// Calculate vertices and texcoords for a chunk
func CalculateChunkVertices(heightInfo *HeightInfo, waterData []uint8, chunk *Chunk) {
	// Calculate vertex stride based on LOD
	stride := 1 << chunk.LODLevel                // 1, 2, 4, or 8
	verticesPerSide := (CHUNK_SIZE / stride) + 1 // +1 for overlap on right/bottom edges

	// Allocate vertex array based on LOD level
	chunk.Vertices = make([]Vertex, verticesPerSide*verticesPerSide)
	firstZ := true

	for z := range verticesPerSide {
		worldZ := chunk.Z + (z * stride) - 1 // Start one vertex north (subtract a point)
		firstX := true

		for x := range verticesPerSide {
			worldX := chunk.X + (x * stride) - 1 // Start one vertex west (subtract a point)
			localX, localZ := coords.WorldToPixel(worldX, worldZ)

			if firstZ && worldZ%3600 == -1 || worldZ%3600 == 1 {
				localZ -= 3599
			}
			if firstX && worldX%3600 == -1 || worldX%3600 == 1 {
				localX -= 3599
			}

			// Direct array access - much faster than method image
			height := heightInfo.getLocal(localX, localZ)
			isWater := waterData[localZ*MAP_LENGTH+localX]

			idx := z*verticesPerSide + x
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
func CalculateChunkIndices(chunk *Chunk) {
	stride := 1 << chunk.LODLevel
	verticesPerSide := (CHUNK_SIZE / stride) + 1 // +1 for overlap
	numStrips := verticesPerSide - 1
	verticesPerStrip := verticesPerSide * 2
	degenerateVertices := (numStrips - 1) * 2

	chunk.Indices = make([]Index, (numStrips*verticesPerStrip)+degenerateVertices)
	indexPos := 0

	for z := range verticesPerSide - 1 {
		if z > 0 {
			// Degenerate triangle to connect strips
			chunk.Indices[indexPos] = Index(z * verticesPerSide)
			indexPos++
		}

		for x := range verticesPerSide {
			chunk.Indices[indexPos] = Index(z*verticesPerSide + x)
			chunk.Indices[indexPos+1] = Index((z+1)*verticesPerSide + x)
			indexPos += 2
		}

		if z < verticesPerSide-2 {
			// Degenerate triangle for next strip
			chunk.Indices[indexPos] = Index((z+1)*verticesPerSide + (verticesPerSide - 1))
			indexPos++
		}
	}
}

// Calculate normals for the chunk
func CalculateChunkNormals(chunk *Chunk) {
	stride := 1 << chunk.LODLevel
	verticesPerSide := (CHUNK_SIZE / stride) + 1 // +1 for overlap

	// For each vertex except the last row and column
	for z := range verticesPerSide - 1 {
		for x := range verticesPerSide - 1 {
			idx := z*verticesPerSide + x
			current := chunk.Vertices[idx].Position
			right := chunk.Vertices[idx+1].Position
			down := chunk.Vertices[idx+verticesPerSide].Position

			var normal, a, b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			chunk.Vertices[idx].Normal = normal
		}
	}

	// Handle right edge - use the normal from the vertex to the left
	for z := range verticesPerSide - 1 {
		idx := z*verticesPerSide + (verticesPerSide - 1)
		chunk.Vertices[idx].Normal = chunk.Vertices[idx-1].Normal
	}

	// Handle bottom edge - use the normal from the vertex above
	for x := range verticesPerSide {
		idx := (verticesPerSide-1)*verticesPerSide + x
		chunk.Vertices[idx].Normal = chunk.Vertices[idx-verticesPerSide].Normal
	}

	// Handle bottom-right corner - use normal from vertex to the left
	lastIdx := (verticesPerSide * verticesPerSide) - 1
	chunk.Vertices[lastIdx].Normal = chunk.Vertices[lastIdx-1].Normal
}

// CalculateLODChunkSizes calculates memory requirements for each LOD level
func CalculateLODChunkSizes() []vk.DeviceSize {
	numLODLevels := LOD_COUNT
	lodChunkSizes := make([]vk.DeviceSize, numLODLevels)

	for lod := range numLODLevels {
		stride := 1
		if lod > 0 {
			stride = 1 << uint(lod)
		}

		vps := (CHUNK_SIZE / stride) + 1 // +1 for overlap
		numVertices := vps * vps
		vertexDataSize := vk.DeviceSize(numVertices) * vk.DeviceSize(VertexSize)

		// Calculate indices (same logic as existing chunk.go)
		numIndices := 0
		if vps > 1 {
			numStrips := vps - 1
			numIndices = numStrips * vps * 2
			if numStrips > 1 {
				numIndices += (numStrips - 1) * 2
			}
		}
		indexDataSize := vk.DeviceSize(numIndices) * vk.DeviceSize(IndexSize)

		lodChunkSizes[lod] = vertexDataSize + indexDataSize
	}
	return lodChunkSizes
}
