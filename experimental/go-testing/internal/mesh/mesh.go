package mesh

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"go-testing/internal/types"

	lin "github.com/xlab/linmath"
)

const (
	length = 3601          // Image Height and Width
	scale  = 60.0 / 1852.0 // Meters to Arcminutes
)

func LoadHM(hmPath string) ([]types.Vertex, []types.Index, error) {
	// Preallocate slices
	vertices := make([]types.Vertex, length*length)
	indices := make([]types.Index, 6*(length-1)*(length-1))

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
	for y := 0; y < length; y++ {
		for x := 0; x < length; x++ {
			// Might need to change for performance improvements (direct array arithmetic?)
			pixelValue := grayImg.Gray16At(x, y).Y
			height := float32(int(pixelValue) - 32768)
			normX := float32(x)
			normZ := float32(y)

			vertices[y*length + x] = types.Vertex{
				Pos:    [3]float32{normX, height * scale, normZ},
				Color:  [3]float32{1.0, 1.0, 1.0},
				Normal: [3]float32{0.0, 0.0, 0.0},
				TexCoord: [2]float32{
					normZ / float32(length-1),
					normX / float32(length-1),
				},
			}
		}
	}

	// Generate indices
	indexPos := 0
	for y := 0; y < length-1; y++ {
		for x := 0; x < length-1; x++ {
			topLeft := types.Index(y*length + x)
			topRight := topLeft + 1
			botLeft := types.Index((y+1)*length + x)
			botRight := botLeft + 1

			// First triangle
			indices[indexPos] = topLeft
			indices[indexPos+1] = botLeft
			indices[indexPos+2] = topRight
			indexPos += 3

			// Second triangle
			indices[indexPos] = topRight
			indices[indexPos+1] = botLeft
			indices[indexPos+2] = botRight
			indexPos += 3
		}
	}

	calculateNormals(&vertices)
	return vertices, indices, nil
}

// Calculates the Normal Vector for each vertex in the given vertices
func calculateNormals(vertices *[]types.Vertex) {
	for y := 0; y < length-1; y++ {
		for x := 0; x < length-1; x++ {
			current := (*vertices)[y*length+x].Pos
			right := (*vertices)[y*length+(x+1)].Pos
			down := (*vertices)[(y+1)*length+x].Pos

			// Funky way of doing math with linmath
			var normal lin.Vec3
			var a lin.Vec3
			var b lin.Vec3
			a.Sub(&down, &current)
			b.Sub(&right, &current)
			normal.MultCross(&a, &b)
			normal.Norm(&normal)

			(*vertices)[y*length+x].Normal = normal
		}
	}
}
