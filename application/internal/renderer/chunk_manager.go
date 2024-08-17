package renderer

import (
	"fmt"
	"image"
	"image/png"
	"os"

	lin "github.com/xlab/linmath"
)

// This file should be used for chunk management, chunk rendering, etc.
// TODO maybe save raw arrays rather than image pointers?

const (
	WATERMAP_EXTENSION  = ".swb.png"
	HEIGHTMAP_EXTENSION = ".hgt.png"

	MAP_LENGTH      = 3601
	CHUNKS_PER_SIDE = 64
	CHUNKS_PER_MAP  = CHUNKS_PER_SIDE * CHUNKS_PER_SIDE
	CHUNK_SIZE      = MAP_LENGTH / CHUNKS_PER_SIDE
	NUM_MAPS        = 4

	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcminutes
)

// A single ChunkManager to manage load/unloading images and chunks
type ChunkManager struct {
	Heightmaps    map[string]*image.Gray16
	Watermaps     map[string]*image.Gray
	Chunks        map[string]Chunk
	VisibleChunks []*Chunk
}

func NewChunkManager() *ChunkManager {
	return &ChunkManager{
		Heightmaps:    make(map[string]*image.Gray16, NUM_MAPS),
		Watermaps:     make(map[string]*image.Gray, NUM_MAPS),
		Chunks:        make(map[string]Chunk, NUM_MAPS*CHUNKS_PER_MAP),
		VisibleChunks: make([]*Chunk, 0),
	}
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

// Loads a watermap (coord.swb.png) into the ChunkManager
func (cm *ChunkManager) LoadWatermap(coord string) error {
	filepath := coord + WATERMAP_EXTENSION
	rawImg, err := loadMap(filepath)
	if err != nil {
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray)
	if !ok {
		return fmt.Errorf("watermap is not in 8-bit grayscale format")
	}

	cm.Watermaps[coord] = grayImg

	return nil
}

// Loads a heightmap (coord.hgt.png) into the ChunkManager
func (cm *ChunkManager) LoadHeightmap(coord string) error {
	filepath := coord + HEIGHTMAP_EXTENSION
	rawImg, err := loadMap(filepath)
	if err != nil {
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray16)
	if !ok {
		return fmt.Errorf("heightmap is not in 16-bit grayscale format")
	}

	cm.Heightmaps[coord] = grayImg

	return nil
}

// Unloads both the water and heightmaps
func (cm *ChunkManager) UnloadMaps(coord string) {
	cm.unloadWatermap(coord)
	cm.unloadHeightmap(coord)
}

// Unloads a watermap (coord) from the ChunkManager
func (cm *ChunkManager) unloadWatermap(coord string) {
	delete(cm.Watermaps, coord)
}

// Unloads a heightmap (coord) from the ChunkManager
func (cm *ChunkManager) unloadHeightmap(coord string) {
	delete(cm.Heightmaps, coord)
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
//      <-  3600 * 180 * 2 ->
//		|-------------------| ▲
//	   	|	    World 		| 3600 * 90 * 2
//		|-------------------| ▼

func (cm *ChunkManager) LoadChunk(chunkX, chunkZ int) {
	// TODO move the map coord logic out of this function, user must use and verify
	mapCoords := CoordinateFloatToMap(chunkX, chunkZ)
	heightmap := cm.Heightmaps[mapCoords]
	watermap := cm.Watermaps[mapCoords]

	// Load the chunk based on heightmap and watermap
	if heightmap == nil || watermap == nil {
		fmt.Printf("Warning: Heightmap or Watermap not loaded for coordinates %s\n", mapCoords)
		return
	}

	chunkKey := ChunkKey(chunkX, chunkZ)
	if _, exists := cm.Chunks[chunkKey]; exists {
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

	cm.Chunks[chunkKey] = *chunk
}

// Generate vertices and texcoords
func calculateChunkVertices(heightmap *image.Gray16, watermap *image.Gray, chunk *Chunk) {
	// Fill the vertices
	for z := 0; z < ChunkSize; z++ {
		for x := 0; x < ChunkSize; x++ {
			worldX := chunk.X + x
			worldZ := chunk.Z + z

			if worldX >= length || worldZ >= length {
				continue // Skip vertices outside the heightmap
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
