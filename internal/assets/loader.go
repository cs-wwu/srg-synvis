package assets

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

const (
	heightmapExtension = ".hgt.png"
	watermapExtension  = ".swb.png"
	textureExtension   = ".osm.png"
)

// loadFromDisk loads both heightmap and watermap from disk.
func (am *assetManager) loadFromDisk(coord string) (*MapData, error) {
	// Load heightmap
	heightmapPath := am.resolvePath(coord, fileTypeHeightmap)
	heightmap, err := loadGray16Image(heightmapPath)
	if err != nil {
		return nil, err
	}

	// Load watermap
	watermapPath := am.resolvePath(coord, fileTypeWatermap)
	watermap, err := loadGrayImage(watermapPath)
	if err != nil {
		// If heightmap exists but watermap doesn't, generate ocean watermap
		if os.IsNotExist(err) {
			watermap = generateFullWatermap()
		} else {
			return nil, err
		}
	}

	// Validate dimensions
	if err := validateMapDimensions(heightmap, watermap); err != nil {
		return nil, err
	}

	return &MapData{
		Coord:     coord,
		Heightmap: heightmap,
		Watermap:  watermap,
		LoadedAt:  time.Now(),
		Size:      calculateMapSize(heightmap, watermap),
		IsOcean:   false,
	}, nil
}

// resolvePath builds the full file path for an asset.
func (am *assetManager) resolvePath(coord string, fType fileType) string {
	var extension string
	switch fType {
	case fileTypeHeightmap:
		extension = heightmapExtension
	case fileTypeWatermap:
		extension = watermapExtension
	case fileTypeTexture:
		extension = textureExtension
	default:
		extension = ".unknown"
	}

	return filepath.Join(am.basePath, coord+extension)
}

// loadGray16Image loads a 16-bit grayscale PNG image.
func loadGray16Image(path string) (*image.Gray16, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image %s: %w", path, err)
	}

	if format != "png" {
		return nil, fmt.Errorf("expected PNG format, got %s", format)
	}

	// Convert to Gray16 if necessary
	gray16, ok := img.(*image.Gray16)
	if !ok {
		// Convert to Gray16
		bounds := img.Bounds()
		gray16 = image.NewGray16(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				gray16.Set(x, y, img.At(x, y))
			}
		}
	}

	return gray16, nil
}

// loadGrayImage loads an 8-bit grayscale PNG image.
func loadGrayImage(path string) (*image.Gray, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image %s: %w", path, err)
	}

	if format != "png" {
		return nil, fmt.Errorf("expected PNG format, got %s", format)
	}

	// Convert to Gray if necessary
	gray, ok := img.(*image.Gray)
	if !ok {
		// Convert to Gray
		bounds := img.Bounds()
		gray = image.NewGray(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				gray.Set(x, y, img.At(x, y))
			}
		}
	}

	return gray, nil
}

// validateMapDimensions ensures heightmap and watermap have correct dimensions.
func validateMapDimensions(heightmap *image.Gray16, watermap *image.Gray) error {
	const expectedSize = 3601

	hBounds := heightmap.Bounds()
	wBounds := watermap.Bounds()

	// Check heightmap dimensions
	if hBounds.Dx() != expectedSize || hBounds.Dy() != expectedSize {
		return fmt.Errorf("invalid heightmap dimensions: expected %dx%d, got %dx%d",
			expectedSize, expectedSize, hBounds.Dx(), hBounds.Dy())
	}

	// Check watermap dimensions
	if wBounds.Dx() != expectedSize || wBounds.Dy() != expectedSize {
		return fmt.Errorf("invalid watermap dimensions: expected %dx%d, got %dx%d",
			expectedSize, expectedSize, wBounds.Dx(), wBounds.Dy())
	}

	// Ensure dimensions match
	if hBounds.Dx() != wBounds.Dx() || hBounds.Dy() != wBounds.Dy() {
		return fmt.Errorf("heightmap and watermap dimensions don't match")
	}

	return nil
}

// calculateMapSize estimates memory usage for a map.
func calculateMapSize(heightmap *image.Gray16, watermap *image.Gray) int64 {
	// Gray16: 2 bytes per pixel
	// Gray: 1 byte per pixel
	bounds := heightmap.Bounds()
	pixels := int64(bounds.Dx() * bounds.Dy())

	heightmapSize := pixels * 2
	watermapSize := pixels
	overhead := int64(2048) // Estimate for image structure overhead

	return heightmapSize + watermapSize + overhead
}

// SaveMapData saves a MapData to disk (useful for testing or caching generated ocean tiles).
func SaveMapData(basePath string, data *MapData) error {
	// Create directory if needed
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Save heightmap
	heightmapPath := filepath.Join(basePath, data.Coord+heightmapExtension)
	if err := saveGray16Image(heightmapPath, data.Heightmap); err != nil {
		return fmt.Errorf("failed to save heightmap: %w", err)
	}

	// Save watermap
	watermapPath := filepath.Join(basePath, data.Coord+watermapExtension)
	if err := saveGrayImage(watermapPath, data.Watermap); err != nil {
		return fmt.Errorf("failed to save watermap: %w", err)
	}

	return nil
}

// saveGray16Image saves a 16-bit grayscale image as PNG.
func saveGray16Image(path string, img *image.Gray16) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
	return encoder.Encode(file, img)
}

// saveGrayImage saves an 8-bit grayscale image as PNG.
func saveGrayImage(path string, img *image.Gray) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
	return encoder.Encode(file, img)
}
