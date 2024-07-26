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

    // Generate vertices and indices for this chunk
    chunk.vertices = make([]Vertex, ChunkSize*ChunkSize)
    chunk.indices = make([]Index, 2*ChunkSize*(ChunkSize-1)+2*((ChunkSize-1)-1))

    // Fill vertices
    for z := 0; z < ChunkSize; z++ {
        for x := 0; x < ChunkSize; x++ {
            globalX, globalZ := chunk.X+x, chunk.Z+z
            pixelValue := app.heightmap.Gray16At(globalX, globalZ).Y
            height := float32(int(pixelValue) - 32768)

            chunk.vertices[z*ChunkSize+x] = Vertex{
                Position: lin.Vec3{
                    float32(globalX), // Use global X coordinate
                    height * scale,
                    float32(globalZ), // Use global Z coordinate
                },
                Color:    lin.Vec3{1.0, 1.0, 1.0},
                Normal:   lin.Vec3{0.0, 0.0, 0.0},
                TexCoord: lin.Vec2{
                    float32(globalX) / float32(length-1),
                    float32(globalZ) / float32(length-1),
                },
            }
        }
    }

    // Generate indices (using triangle strips)
    indexPos := 0
    for z := 0; z < ChunkSize-1; z++ {
        if z > 0 {
            chunk.indices[indexPos] = Index(z * ChunkSize)
            indexPos++
        }

        for x := 0; x < ChunkSize; x++ {
            chunk.indices[indexPos] = Index(z*ChunkSize + x)
            chunk.indices[indexPos+1] = Index((z+1)*ChunkSize + x)
            indexPos += 2
        }

        if z < ChunkSize-2 {
            chunk.indices[indexPos] = Index((z+1)*ChunkSize + (ChunkSize - 1))
            indexPos++
        }
    }

    calculateChunkNormals(chunk)

    // Create Vulkan buffers for this chunk
    app.createChunkBuffers(chunk)

    app.chunks[key] = chunk
    app.loadedChunks = append(app.loadedChunks, chunk)
}

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

func calculateChunkNormals(c *Chunk) {
	for z := 0; z < ChunkSize; z++ {
		for x := 0; x < ChunkSize; x++ {
			if x < ChunkSize-1 && z < ChunkSize-1 {
				current := &c.vertices[z*ChunkSize+x].Position
				right := &c.vertices[z*ChunkSize+(x+1)].Position
				down := &c.vertices[(z+1)*ChunkSize+x].Position

				var normal lin.Vec3
				var a, b lin.Vec3
				a.Sub(down, current)
				b.Sub(right, current)
				normal.MultCross(&a, &b)
				normal.Norm(&normal)

				c.vertices[z*ChunkSize+x].Normal = normal
			} else {
				// For edge vertices, just use the normal of the previous vertex
				if x > 0 {
					c.vertices[z*ChunkSize+x].Normal = c.vertices[z*ChunkSize+(x-1)].Normal
				} else if z > 0 {
					c.vertices[z*ChunkSize+x].Normal = c.vertices[(z-1)*ChunkSize+x].Normal
				}
			}
		}
	}
}

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

// ------------------ Old Heightmap Loading for Reference ------------------

func LoadHM(hmPath string) ([]Vertex, []Index, error) {
	// Preallocate slices
	vertices := make([]Vertex, length*length)
	// indices := make([]Index, 6*(length-1)*(length-1))
	indices := make([]Index, 2*length*(length-1)+2*((length-1)-1))

	grayImg, err := GetHM(hmPath)
	if err != nil {
		return nil, nil, err
	}

	// Generate vertices
	for z := 0; z < length; z++ {
		for x := 0; x < length; x++ {
			// Might need to change for performance improvements (direct array arithmetic?)
			pixelValue := grayImg.Gray16At(x, z).Y
			height := float32(int(pixelValue) - 32768) // Y
			normX := float32(x)
			normZ := float32(z)

			vertices[z*length+x] = Vertex{
				Position: lin.Vec3{normX, height * scale, normZ},
				Color:    lin.Vec3{1.0, 1.0, 1.0},
				Normal:   lin.Vec3{0.0, 0.0, 0.0},
				TexCoord: lin.Vec2{
					float32(x) / float32(length-1), // Normalize to [0, 1]
					float32(z) / float32(length-1), // Normalize to [0, 1]
				},
			}
		}
	}

	indexPos := 0
	for z := 0; z < length-1; z++ {
		if z > 0 {
			indices[indexPos] = Index(z * length)
			indexPos++
		}

		for x := 0; x < length; x++ {
			indices[indexPos] = Index(z*length + x)
			indices[indexPos+1] = Index((z+1)*length + x)
			indexPos += 2
		}

		if z < length-2 {
			indices[indexPos] = Index((z+1)*length + (length - 1))
			indexPos++
		}
	}

	calculateNormals(&vertices)
	return vertices, indices, nil
}

// Calculates the Normal Vector for each vertex in the given vertices
func calculateNormals(vertices *[]Vertex) {
	for z := 0; z < length-1; z++ {
		for x := 0; x < length-1; x++ {
			current := (*vertices)[z*length+x].Position
			right := (*vertices)[z*length+(x+1)].Position
			down := (*vertices)[(z+1)*length+x].Position

			// Funky way of doing math with linmath
			var normal lin.Vec3
			var a lin.Vec3
			var b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			(*vertices)[z*length+x].Normal = normal
		}
	}
}
