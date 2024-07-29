package renderer

import (
	"fmt"
	"image"
	"image/png"
	"os"

	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

const (
	length        = 3601          // Image Height and Width
	scale         = 60.0 / 1852.0 // Meters to Arcminutes
	ChunkSize     = 64            // Vertices per side of a chunk
	ChunksPerSide = length / ChunkSize

	LOD_LEVELS   = 6
	LOD_DISTANCE = 250.0
)

type Chunk struct {
	vertices     []Vertex
	indices      []Index
	vertexBuffer vk.Buffer
	vertexMemory vk.DeviceMemory
	indexBuffer  vk.Buffer
	indexMemory  vk.DeviceMemory
	X            int // Global X coordinate of the chunk's top-left corner
	Z            int // Global Z coordinate of the chunk's top-left corner
	lodLevel     int
}

func (app *Application) LoadChunk(chunkX, chunkZ int) {
	if app.heightmap == nil {
		panic("Heightmap not loaded")
	}

	key := chunkKey(chunkX, chunkZ)
	if _, exists := app.chunks[key]; exists {
		return // Chunk already loaded
	}

	chunk := &Chunk{
		X: chunkX * ChunkSize,
		Z: chunkZ * ChunkSize,
	}

	// Add one vertex on each side for a chunk skirt
	extendedSize := ChunkSize + 2
	chunk.vertices = make([]Vertex, extendedSize*extendedSize)
	chunk.indices = make([]Index, 2*extendedSize*(extendedSize-1)+2*((extendedSize-1)-1))
	// This is for merging chunks together to stop chunk gaps
	skirtDropFactor := float32(0.1) // This value can be adjusted

	// Fill vertices
	for z := -1; z <= ChunkSize; z++ {
		for x := -1; x <= ChunkSize; x++ {
			globalX, globalZ := chunk.X+x, chunk.Z+z
			pixelValue := app.heightmap.Gray16At(clamp(globalX, 0, length-1), clamp(globalZ, 0, length-1)).Y
			height := float32(int(pixelValue) - 32768)

			// Lower the height for skirt vertices
			if x == -1 || x == ChunkSize || z == -1 || z == ChunkSize {
				height -= skirtDropFactor
			}

			chunk.vertices[(z+1)*extendedSize+(x+1)] = Vertex{
				Position: lin.Vec3{
					float32(globalX),
					height * scale,
					float32(globalZ),
				},
				Color:  lin.Vec3{1.0, 1.0, 1.0},
				Normal: lin.Vec3{0.0, 0.0, 0.0},
				TexCoord: lin.Vec2{
					float32(globalX) / float32(length-1),
					float32(globalZ) / float32(length-1),
				},
			}
		}
	}

	// Generate indices (using triangle strips)
	indexPos := 0
	for z := 0; z < extendedSize-1; z++ {
		if z > 0 {
			chunk.indices[indexPos] = Index(z * extendedSize)
			indexPos++
		}

		for x := 0; x < extendedSize; x++ {
			chunk.indices[indexPos] = Index(z*extendedSize + x)
			chunk.indices[indexPos+1] = Index((z+1)*extendedSize + x)
			indexPos += 2
		}

		if z < extendedSize-2 {
			chunk.indices[indexPos] = Index((z+1)*extendedSize + (extendedSize - 1))
			indexPos++
		}
	}

	calculateChunkNormals(chunk)

	// Create Vulkan buffers for this chunk
	app.createChunkBuffers(chunk)

	app.chunks[key] = chunk
	app.loadedChunks = append(app.loadedChunks, chunk)
}

// Creates and implements the Vulkan Vertex and Index buffers for this chunk
func (app *Application) createChunkBuffers(chunk *Chunk) {
	dev := app.Context().Device()
	memProps := app.Context().Platform().MemoryProperties()

	// Create vertex buffer
	vertexBufferSize := len(chunk.vertices) * VertexSize
	vertexDataRaw := make([]byte, vertexBufferSize)
	for i, vertex := range chunk.vertices {
		copy(vertexDataRaw[i*VertexSize:], vertex.Data())
	}
	vertexBuf := as.CreateBuffer(dev, memProps, vertexDataRaw, vk.BufferUsageVertexBufferBit)
	chunk.vertexBuffer = vertexBuf.Buffer
	chunk.vertexMemory = vertexBuf.Memory

	// Create index buffer
	indexBufferSize := len(chunk.indices) * IndexSize
	indexDataRaw := make([]byte, indexBufferSize)
	for i, index := range chunk.indices {
		copy(indexDataRaw[i*IndexSize:], index.Data())
	}
	indexBuf := as.CreateBuffer(dev, memProps, indexDataRaw, vk.BufferUsageIndexBufferBit)
	chunk.indexBuffer = indexBuf.Buffer
	chunk.indexMemory = indexBuf.Memory
}

// Unloads the chunk at the given coordinates
func (app *Application) UnloadChunk(chunkX, chunkZ int) {
	key := chunkKey(chunkX, chunkZ)
	chunk, exists := app.chunks[key]
	if !exists {
		return // Chunk not loaded
	}

	dev := app.Context().Device()
	vk.DestroyBuffer(dev, chunk.vertexBuffer, nil)
	vk.FreeMemory(dev, chunk.vertexMemory, nil)
	vk.DestroyBuffer(dev, chunk.indexBuffer, nil)
	vk.FreeMemory(dev, chunk.indexMemory, nil)

	delete(app.chunks, key)
	for i, c := range app.loadedChunks {
		if c == chunk {
			app.loadedChunks = append(app.loadedChunks[:i], app.loadedChunks[i+1:]...)
			break
		}
	}
}

// Helper function to create a unique key for each chunk
func chunkKey(x, z int) string {
	return fmt.Sprintf("%d,%d", x, z)
}

// Calculates the chunk's normal vectors (stored in the chunks vertices)
func calculateChunkNormals(c *Chunk) {
	extendedSize := ChunkSize + 2
	for z := 0; z < extendedSize; z++ {
		for x := 0; x < extendedSize; x++ {
			if x < extendedSize-1 && z < extendedSize-1 {
				current := &c.vertices[z*extendedSize+x].Position
				right := &c.vertices[z*extendedSize+(x+1)].Position
				down := &c.vertices[(z+1)*extendedSize+x].Position

				var normal lin.Vec3
				var a, b lin.Vec3
				a.Sub(down, current)
				b.Sub(right, current)
				normal.MultCross(&a, &b)
				normal.Norm(&normal)

				c.vertices[z*extendedSize+x].Normal = normal
			} else {
				// For edge vertices, just use the normal of the previous vertex
				if x > 0 {
					c.vertices[z*extendedSize+x].Normal = c.vertices[z*extendedSize+(x-1)].Normal
				} else if z > 0 {
					c.vertices[z*extendedSize+x].Normal = c.vertices[(z-1)*extendedSize+x].Normal
				}
			}
		}
	}
}

func distanceToChunk(chunk *Chunk, cameraPos *lin.Vec3) float32 {
	chunkCenter := lin.Vec3{
		float32(chunk.X) + float32(ChunkSize)/2,
		0, // Assuming Y is up
		float32(chunk.Z) + float32(ChunkSize)/2,
	}
	diff := lin.Vec3{}
	diff.Sub(&chunkCenter, cameraPos)
	return diff.Len()
}

// Returns a pointer to the heightmap image
func GetHM(hmPath string) (*image.Gray16, error) {
	file, err := os.Open(hmPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open heightmap: %v", err)
	}
	defer file.Close()

	// Decode image
	rawImg, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode heightmap: %v", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray16)
	if !ok {
		return nil, fmt.Errorf("heightmap is not in 16-bit grayscale format")
	}

	// Verify image size
	width, height := rawImg.Bounds().Dx(), rawImg.Bounds().Dy()
	if width != length || height != length {
		return nil, fmt.Errorf("bad image bounds: (%d, %d)", width, height)
	}

	return grayImg, nil
}

// Helper function to clamp values
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
