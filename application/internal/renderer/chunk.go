package renderer

import (
	"fmt"
	"image"
	"log"

	lin "github.com/xlab/linmath"
)

func ChunkKey(chunkX, chunkZ int) string {
	return fmt.Sprintf("%d,%d", chunkX, chunkZ)
}

type Chunk struct {
	vertices []Vertex
	indices  []Index
	X        int // Global X Position of the top-left chunk corner
	Z        int // Global Z Position of the top-left chunk corner
}

// Loads a specific chunk into memory, x and z are the upper-left corner of
// the chunk (world is in arcsecond coordinates)
// Note that the world is Latitude by Longitude times 3600 (arcseconds):
//
//	<-    3600 * 360   ->
//	--------------------- ▲
//	|      World        | 3600 * 180
//	--------------------- ▼
func (tm *TerrainManager) LoadChunk(chunkX, chunkZ int) {
	mapCoords := CoordinateIntToMap(chunkX, chunkZ)
	heightmap := tm.heightmaps[mapCoords]
	watermap := tm.watermaps[mapCoords]

	if heightmap == nil || watermap == nil {
		log.Printf("Warning: Map(s) not loaded for coords %s (%d, %d)\n", mapCoords, chunkX, chunkZ)
		return
	}

	chunkKey := ChunkKey(chunkX, chunkZ)
	if _, exists := tm.chunks[chunkKey]; exists {
		return
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

	tm.chunks[chunkKey] = chunk
	tm.orderedChunks = append(tm.orderedChunks, chunk)
}

// Calculate vertices and texcoords for a chunk
func calculateChunkVertices(heightmap *image.Gray16, watermap *image.Gray, chunk *Chunk) {
	chunk.vertices = make([]Vertex, (CHUNK_SIZE+1)*(CHUNK_SIZE+1))

	for z := 0; z <= CHUNK_SIZE; z++ {
		worldZ := chunk.Z + z
		for x := 0; x <= CHUNK_SIZE; x++ {
			worldX := chunk.X + x
			localX, localZ := CoordinateIntToPixel(worldX, worldZ)

			pixelValue := heightmap.Gray16At(localX, localZ).Y
			height := float32(int(pixelValue) - 32768)
			isWater := watermap.GrayAt(localX, localZ).Y

			idx := z*(CHUNK_SIZE+1) + x
			chunk.vertices[idx] = Vertex{
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
	chunk.indices = make([]Index, 2*CHUNK_SIZE*(CHUNK_SIZE+1)+2*(CHUNK_SIZE-1))
	indexPos := 0

	for z := 0; z < CHUNK_SIZE; z++ {
		if z > 0 {
			// Degenerate triangle to connect strips
			chunk.indices[indexPos] = Index(z * (CHUNK_SIZE + 1))
			indexPos++
		}

		for x := 0; x <= CHUNK_SIZE; x++ {
			chunk.indices[indexPos] = Index(z*(CHUNK_SIZE+1) + x)
			chunk.indices[indexPos+1] = Index((z+1)*(CHUNK_SIZE+1) + x)
			indexPos += 2
		}

		if z < CHUNK_SIZE-1 {
			// Degenerate triangle to connect strips
			chunk.indices[indexPos] = Index((z+1)*(CHUNK_SIZE+1) + CHUNK_SIZE)
			indexPos++
		}
	}
}

// Calculate normals for the chunk
func calculateChunkNormals(chunk *Chunk) {
	width := CHUNK_SIZE + 1
	for z := 0; z < CHUNK_SIZE; z++ {
		for x := 0; x < CHUNK_SIZE; x++ {
			idx := z*width + x
			current := chunk.vertices[idx].Position
			right := chunk.vertices[idx+1].Position
			down := chunk.vertices[idx+width].Position

			var normal, a, b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			chunk.vertices[idx].Normal = normal
		}
	}

	// Handle right edge
	for z := 0; z < CHUNK_SIZE; z++ {
		idx := z*width + CHUNK_SIZE
		chunk.vertices[idx].Normal = chunk.vertices[idx-1].Normal
	}

	// Handle bottom edge
	for x := 0; x <= CHUNK_SIZE; x++ {
		idx := CHUNK_SIZE*width + x
		chunk.vertices[idx].Normal = chunk.vertices[idx-width].Normal
	}
}

func (chunk *Chunk) GetBoundingBox() (min, max lin.Vec3) {
	if len(chunk.vertices) == 0 {
		return lin.Vec3{}, lin.Vec3{}
	}

	min = lin.Vec3{
		float32(chunk.X),
		chunk.vertices[0].Position[1],
		float32(chunk.Z),
	}
	max = min

	for _, vertex := range chunk.vertices {
		// Update Y bounds only (X and Z are determined by chunk boundaries)
		if vertex.Position[1] < min[1] {
			min[1] = vertex.Position[1]
		}
		if vertex.Position[1] > max[1] {
			max[1] = vertex.Position[1]
		}
	}

	max[0] = float32(chunk.X + CHUNK_SIZE)
	max[2] = float32(chunk.Z + CHUNK_SIZE)

	return min, max
}
