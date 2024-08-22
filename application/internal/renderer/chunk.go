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
	cm.orderedChunks = append(cm.orderedChunks, chunk)
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
