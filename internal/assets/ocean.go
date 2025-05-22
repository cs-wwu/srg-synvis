package assets

import (
	"image"
	"image/color"
	"time"
)

const (
	// Map dimensions (SRTM/NASADEM standard)
	mapSize = 3601

	// Sea level in 16-bit heightmap (32768 is zero/sea level)
	seaLevel       = 32768
	oceanElevation = 32760 // 8 units below sea level

	// Water values
	noWater   = 0
	fullWater = 255
)

// generateOceanTile creates a flat ocean tile with all water.
func (am *assetManager) generateOceanTile(coord string) *MapData {
	heightmap := generateOceanHeightmap()
	watermap := generateFullWatermap()

	return &MapData{
		Coord:     coord,
		Heightmap: heightmap,
		Watermap:  watermap,
		LoadedAt:  time.Now(),
		Size:      calculateMapSize(heightmap, watermap),
		IsOcean:   true,
	}
}

// generateOceanHeightmap creates a heightmap for ocean (flat, below sea level).
func generateOceanHeightmap() *image.Gray16 {
	img := image.NewGray16(image.Rect(0, 0, mapSize, mapSize))

	// Set all pixels to ocean elevation
	oceanColor := color.Gray16{Y: oceanElevation}
	for y := range mapSize {
		for x := range mapSize {
			img.SetGray16(x, y, oceanColor)
		}
	}

	return img
}

// generateFullWatermap creates a watermap with all water.
func generateFullWatermap() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, mapSize, mapSize))

	// Set all pixels to full water
	waterColor := color.Gray{Y: fullWater}
	for y := range mapSize {
		for x := range mapSize {
			img.SetGray(x, y, waterColor)
		}
	}

	return img
}

// Direction represents cardinal directions for coastal ocean generation.
type Direction int

const (
	North Direction = iota
	East
	South
	West
)
