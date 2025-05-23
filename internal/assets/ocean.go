package assets

import (
	"time"
)

const (
	// Map dimensions (SRTM/NASADEM standard)
	mapSize = 3601

	// Sea level in 16-bit heightmap (32768 is zero/sea level)
	seaLevel = uint16(32768)

	fullWater = 255
)

// generateOceanTile creates a flat ocean tile with all water.
func (am *assetManager) generateOceanTile(coord string) *MapData {
	heightData := generateOceanHeight()
	waterData := generateOceanWater()

	return &MapData{
		Coord:      coord,
		HeightData: heightData,
		WaterData:  waterData,
		LoadedAt:   time.Now(),
		Size:       mapSize*mapSize*2 + mapSize*mapSize,
		IsOcean:    true,
	}
}

// generateOceanHeight creates the raw data for an ocean map
func generateOceanHeight() []uint16 {
	data := make([]uint16, mapSize*mapSize)
	for i := range data {
		data[i] = seaLevel
	}
	return data
}

// generateFullWatermap creates a watermap with all water.
func generateOceanWater() []uint8 {
	data := make([]uint8, mapSize*mapSize)
	for i := range data {
		data[i] = fullWater
	}
	return data
}

// Direction represents cardinal directions for coastal ocean generation.
type Direction int

const (
	North Direction = iota
	East
	South
	West
)
