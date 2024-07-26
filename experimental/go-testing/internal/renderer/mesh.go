package renderer

import (
	"fmt"
	"image"
	"image/png"
	"os"

	lin "github.com/xlab/linmath"
)

const (
	length = 3601 // Image Height and Width
	scale  = 60.0 / 1852.0 // Meters to Arcminutes
	// scale = 0.5 // Exaggerated Scale
)

func LoadHM(hmPath string) ([]Vertex, []Index, error) {
	// Preallocate slices
	vertices := make([]Vertex, length*length)
	// indices := make([]Index, 6*(length-1)*(length-1))
	indices := make([]Index, 3*(length-1)*(length-1))

	file, err := os.Open(hmPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open heightmap: %v", err)
	}
	defer file.Close()

	// Decode image
	rawImg, err := png.Decode(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode heightmap: %v", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray16)
	if !ok {
		return nil, nil, fmt.Errorf("heightmap is not in 16-bit grayscale format")
	}

	// Verify image size
	width, height := rawImg.Bounds().Dx(), rawImg.Bounds().Dy()
	if width != length || height != length {
		return nil, nil, fmt.Errorf("bad image bounds: (%d, %d)", width, height)
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

	// Generate indices
	// indexPos := 0
	// for z := 0; z < length-1; z++ {
	// 	for x := 0; x < length-1; x++ {
	// 		topLeft := Index(z*length + x)
	// 		topRight := topLeft + 1
	// 		botLeft := Index((z+1)*length + x)
	// 		botRight := botLeft + 1

	// 		// First triangle
	// 		indices[indexPos] = topLeft
	// 		indices[indexPos+1] = botLeft
	// 		indices[indexPos+2] = topRight
	// 		indexPos += 3

	// 		// Second triangle
	// 		indices[indexPos] = topRight
	// 		indices[indexPos+1] = botLeft
	// 		indices[indexPos+2] = botRight
	// 		indexPos += 3
	// 	}
	// }

	indexPos := 0
	for z := 0; z < length-1; z++ {
		// Alternate the triangle strip direction to prevent degenerate triangles
		if z%2 == 0 {
			// Even rows: left to right
			for x := 0; x < length; x++ {
				topLeft := Index(z*length + x)
				botLeft := Index((z+1)*length + x)

				// Add two vertices per iteration (triangle strip)
				indices[indexPos] = topLeft
				indices[indexPos+1] = botLeft
				indexPos += 2
			}
		} else {
			// Odd rows: right to left
			for x := length - 1; x >= 0; x-- {
				topRight := Index(z*length + x)
				botRight := Index((z+1)*length + x)

				// Add two vertices per iteration (triangle strip)
				indices[indexPos] = topRight
				indices[indexPos+1] = botRight
				indexPos += 2
			}
		}
		if z < length-2 {
			indices[indexPos] = Index(uint32(0xFFFF)) // Insert strip-cut index
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
