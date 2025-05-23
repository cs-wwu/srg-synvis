package rendering

import (
	"fmt"
	"image"

	"github.com/cs-wwu/srg-synvis/internal/coords"
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
	LOD_0 LODLevel = iota // Full detail (180x180)
	LOD_1                 // Half detail (90x90)
	LOD_2                 // Quarter detail (45x45)
	LOD_3                 // Eighth detail (23x23)
)

type Chunk struct {
	Vertices []Vertex
	Indices  []Index
	X        int // Global X Position of the top-left chunk corner
	Z        int // Global Z Position of the top-left chunk corner
	LODLevel LODLevel
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
func CalculateChunkVertices(heightmap *image.Gray16, watermap *image.Gray, chunk *Chunk) {
	// Calculate vertex stride based on LOD
	stride := 1 << chunk.LODLevel                // 1, 2, 4, or 8
	verticesPerSide := (CHUNK_SIZE / stride) + 1 // +1 for overlap on north/west edges

	// Allocate vertex array based on LOD level
	chunk.Vertices = make([]Vertex, verticesPerSide*verticesPerSide)

	for z := range verticesPerSide {
		worldZ := chunk.Z + (z * stride) - stride // Start one stride earlier (north)
		for x := range verticesPerSide {
			worldX := chunk.X + (x * stride) - stride // Start one stride earlier (west)
			localX, localZ := coords.WorldToPixel(worldX, worldZ)

			pixelValue := heightmap.Gray16At(localX, localZ).Y
			height := float32(int(pixelValue) - 32768)
			isWater := watermap.GrayAt(localX, localZ).Y

			idx := z*verticesPerSide + x
			chunk.Vertices[idx] = Vertex{
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
