package assets

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"
)

const (
	heightmapExtension = ".hgt.png"
	watermapExtension  = ".swb.png"
	textureExtension   = ".osm.png"
)

// loadFromDisk loads both heightmap and watermap from disk and converts to raw data.
func (am *assetManager) loadFromDisk(coord string) (*MapData, error) {
	// Load heightmap PNG and convert to raw data
	heightmapPath := am.resolvePath(coord, fileTypeHeightmap)
	heightmap, err := loadGray16Image(heightmapPath)
	if err != nil {
		return nil, err
	}

	// Load watermap PNG and convert to raw data
	watermapPath := am.resolvePath(coord, fileTypeWatermap)
	watermap, err := loadGrayImage(watermapPath)
	if err != nil {
		// If heightmap exists but watermap doesn't, generate all-land watermap
		if os.IsNotExist(err) {
			watermap = nil
		} else {
			return nil, err
		}
	}

	// Validate dimensions
	if err := validateMapDimensions(heightmap, watermap); err != nil {
		return nil, err
	}

	// Convert to raw data arrays
	heightData := gray16PixToArray(heightmap)
	waterData := grayPixToArray(watermap)

	return &MapData{
		Coord:      coord,
		HeightData: heightData,
		WaterData:  waterData,
		LoadedAt:   time.Now(),
		Size:       calculateRawMapSize(heightData, waterData),
		IsOcean:    false,
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

// gray16PixToArray efficiently converts Gray16.Pix to uint16 array.
// This avoids the overhead of Gray16At() method calls.
func gray16PixToArray(img *image.Gray16) []uint16 {
	if img == nil {
		return nil
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	data := make([]uint16, width*height)

	// Direct access to pixel data - much faster than Gray16At calls
	pix := img.Pix
	stride := img.Stride

	for y := range height {
		for x := range width {
			// Each pixel is 2 bytes in big-endian format
			offset := y*stride + x*2
			// Convert big-endian bytes to uint16
			data[y*width+x] = uint16(pix[offset])<<8 | uint16(pix[offset+1])
		}
	}

	return data
}

// grayPixToArray efficiently converts Gray.Pix to uint8 array.
// This avoids the overhead of GrayAt() method calls.
func grayPixToArray(img *image.Gray) []uint8 {
	if img == nil {
		return generateLandWater() // All land if no watermap
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Fast path: if no stride padding and bounds start at origin, direct copy
	if img.Stride == width && bounds.Min.X == 0 && bounds.Min.Y == 0 {
		// Direct copy - much faster!
		data := make([]uint8, len(img.Pix))
		copy(data, img.Pix)
		return data
	}

	// Slower path: handle stride padding
	data := make([]uint8, width*height)
	pix := img.Pix
	stride := img.Stride

	for y := range height {
		// Copy entire row at once - faster than pixel by pixel
		srcStart := y * stride
		dstStart := y * width
		copy(data[dstStart:dstStart+width], pix[srcStart:srcStart+width])
	}

	return data
}

// generateLandWater creates a watermap with all land (no water).
func generateLandWater() []uint8 {
	return make([]uint8, mapSize*mapSize) // All zeros = no water
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
		return nil, fmt.Errorf("image is not in grayscale 16 format")
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
		return nil, fmt.Errorf("image is not in grayscale 8 format")
	}

	return gray, nil
}

// validateMapDimensions ensures heightmap and watermap have correct dimensions.
func validateMapDimensions(heightmap *image.Gray16, watermap *image.Gray) error {
	const expectedSize = 3601

	hBounds := heightmap.Bounds()

	// Check heightmap dimensions
	if hBounds.Dx() != expectedSize || hBounds.Dy() != expectedSize {
		return fmt.Errorf("invalid heightmap dimensions: expected %dx%d, got %dx%d",
			expectedSize, expectedSize, hBounds.Dx(), hBounds.Dy())
	}

	// Check watermap dimensions if it exists
	if watermap != nil {
		wBounds := watermap.Bounds()

		if wBounds.Dx() != expectedSize || wBounds.Dy() != expectedSize {
			return fmt.Errorf("invalid watermap dimensions: expected %dx%d, got %dx%d",
				expectedSize, expectedSize, wBounds.Dx(), wBounds.Dy())
		}

		// Ensure dimensions match
		if hBounds.Dx() != wBounds.Dx() || hBounds.Dy() != wBounds.Dy() {
			return fmt.Errorf("heightmap and watermap dimensions don't match")
		}
	}

	return nil
}

// calculateRawMapSize estimates memory usage for raw map data.
func calculateRawMapSize(heightData []uint16, waterData []uint8) int64 {
	heightSize := int64(len(heightData) * 2) // 2 bytes per uint16
	waterSize := int64(len(waterData))       // 1 byte per uint8
	overhead := int64(1024)                  // Estimate for slice overhead

	return heightSize + waterSize + overhead
}
