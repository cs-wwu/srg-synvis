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

type LODLevel int

const (
	LOD_0 LODLevel = iota // Full detail (180x180)
	LOD_1                 // Half detail (90x90)
	LOD_2                 // Quarter detail (45x45)
	LOD_3                 // Eighth detail (23x23)
)

type Chunk struct {
	vertices []Vertex
	indices  []Index
	X        int // Global X Position of the top-left chunk corner
	Z        int // Global Z Position of the top-left chunk corner
	lodLevel LODLevel
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

	lodlevel := tm.selectLODLevel(chunkX, chunkZ, tm.lastPosition)
	chunkKey := ChunkKey(chunkX, chunkZ)
	_, exists := tm.chunks[chunkKey]

	// Skip loading if it exists and the lodLevel's are the same
	if exists && (lodlevel == tm.chunks[chunkKey].lodLevel) {
		return
	}

	chunk := &Chunk{
		X:        chunkX,
		Z:        chunkZ,
		lodLevel: lodlevel,
	}

	calculateChunkVertices(heightmap, watermap, chunk)
	calculateChunkIndices(chunk)
	calculateChunkNormals(chunk)

	tm.chunks[chunkKey] = chunk
	tm.orderedChunks = append(tm.orderedChunks, chunk)
}

// Calculate vertices and texcoords for a chunk
func calculateChunkVertices(heightmap *image.Gray16, watermap *image.Gray, chunk *Chunk) {
	// Calculate vertex stride based on LOD
	stride := 1 << chunk.lodLevel // 1, 2, 4, or 8
	verticesPerSide := (CHUNK_SIZE / stride)

	// Allocate vertex array based on LOD level
	chunk.vertices = make([]Vertex, verticesPerSide*verticesPerSide)

	for z := range verticesPerSide {
		worldZ := chunk.Z + (z * stride)
		for x := range verticesPerSide {
			worldX := chunk.X + (x * stride)
			localX, localZ := CoordinateIntToPixel(worldX, worldZ)

			pixelValue := heightmap.Gray16At(localX, localZ).Y
			height := float32(int(pixelValue) - 32768)
			isWater := watermap.GrayAt(localX, localZ).Y

			idx := z*verticesPerSide + x
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
	stride := 1 << chunk.lodLevel
	verticesPerSide := (CHUNK_SIZE / stride)
	numStrips := verticesPerSide - 1
	verticesPerStrip := verticesPerSide * 2
	degenerateVertices := (numStrips - 1) * 2

	chunk.indices = make([]Index, (numStrips*verticesPerStrip)+degenerateVertices)
	indexPos := 0

	for z := range verticesPerSide - 1 {
		if z > 0 {
			// Degenerate triangle to connect strips
			chunk.indices[indexPos] = Index(z * verticesPerSide)
			indexPos++
		}

		for x := range verticesPerSide {
			chunk.indices[indexPos] = Index(z*verticesPerSide + x)
			chunk.indices[indexPos+1] = Index((z+1)*verticesPerSide + x)
			indexPos += 2
		}

		if z < verticesPerSide-2 {
			// Degenerate triangle for next strip
			chunk.indices[indexPos] = Index((z+1)*verticesPerSide + (verticesPerSide - 1))
			indexPos++
		}
	}
}

// Calculate normals for the chunk
func calculateChunkNormals(chunk *Chunk) {
	stride := 1 << chunk.lodLevel
	verticesPerSide := CHUNK_SIZE / stride

	// For each vertex except the last row and column
	for z := range verticesPerSide - 1 {
		for x := range verticesPerSide - 1 {
			idx := z*verticesPerSide + x
			current := chunk.vertices[idx].Position
			right := chunk.vertices[idx+1].Position
			down := chunk.vertices[idx+verticesPerSide].Position

			var normal, a, b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			chunk.vertices[idx].Normal = normal
		}
	}

	// Handle right edge - use the normal from the vertex to the left
	for z := range verticesPerSide - 1 {
		idx := z*verticesPerSide + (verticesPerSide - 1)
		chunk.vertices[idx].Normal = chunk.vertices[idx-1].Normal
	}

	// Handle bottom edge - use the normal from the vertex above
	for x := range verticesPerSide {
		idx := (verticesPerSide-1)*verticesPerSide + x
		chunk.vertices[idx].Normal = chunk.vertices[idx-verticesPerSide].Normal
	}

	// Handle bottom-right corner - use normal from vertex to the left
	lastIdx := (verticesPerSide * verticesPerSide) - 1
	chunk.vertices[lastIdx].Normal = chunk.vertices[lastIdx-1].Normal
}

func (tm *TerrainManager) selectLODLevel(chunkX, chunkZ int, cameraPos lin.Vec3) LODLevel {
	// Calculate chunk center position
	chunkCenterX := float32(chunkX) + float32(CHUNK_SIZE)/2
	chunkCenterZ := float32(chunkZ) + float32(CHUNK_SIZE)/2

	// Calculate average height for chunk center (or use actual terrain height if available)
	// For now, using camera Y directly since terrain height might be complex to calculate
	chunkCenterY := float32(0.0) // This could be refined based on actual terrain height

	// Calculate 3D distance
	dx := chunkCenterX - cameraPos[0]
	dy := chunkCenterY - cameraPos[1]
	dz := chunkCenterZ - cameraPos[2]
	dist3DSq := dx*dx + dy*dy + dz*dz

	// Updated thresholds for 3D distance
	// Square root of previous values to maintain similar visual transitions
	switch {
	case dist3DSq < 240_000: // ~490 units away (including height)
		return LOD_0
	case dist3DSq < 960_000: // ~980 units away
		return LOD_1
	case dist3DSq < 3_840_000: // ~1960 units away
		return LOD_2
	default:
		return LOD_3
	}
}
