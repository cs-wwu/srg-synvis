package chunk

import (
	"math"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/assets"
)

const (
	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds
	MAP_LENGTH   = 3601
)

type HeightInfo struct {
	tileMap map[spatial.MapCoord]chunkHeightInfo
}

type chunkHeightInfo struct {
	heightData *[MAP_LENGTH * MAP_LENGTH]uint16
	waterData  *[MAP_LENGTH * MAP_LENGTH]uint8
}

func newChunkHeightInfo(data *assets.MapData) *chunkHeightInfo {
	return &chunkHeightInfo{
		heightData: (*[MAP_LENGTH * MAP_LENGTH]uint16)(data.HeightData),
		waterData:  (*[MAP_LENGTH * MAP_LENGTH]uint8)(data.WaterData),
	}
}

func (t *HeightInfo) addTile(pos spatial.MapCoord, data []uint16) {
}

func NewHeightInfo(assetManager assets.AssetManager, pos spatial.MapCoord) *HeightInfo {
	heightInfo := &HeightInfo{tileMap: make(map[spatial.MapCoord]chunkHeightInfo)}

	for dlat := -1; dlat <= 1; dlat++ {
		for dlon := -1; dlon <= 1; dlon++ {
			dir := spatial.MapCoord{Lat: pos.Lat + dlat, Lon: pos.Lon + dlon}
			adjData, err := assetManager.LoadMapData(dir)
			if err == nil {
				heightInfo.tileMap[dir] = *newChunkHeightInfo(adjData)
			}
		}
	}

	return heightInfo
}

func (t *HeightInfo) Height(x, z float32) float32 {
	ix := int(math.Floor(float64(x)))
	iz := int(math.Floor(float64(z)))
	fractX := float64(x) - float64(ix)
	fractZ := float64(z) - float64(iz)

	a := t.sample(ix, iz)
	b := t.sample(ix+1, iz)
	c := t.sample(ix, iz+1)
	d := t.sample(ix+1, iz+1)

	return lerp(lerp(a, b, fractX), lerp(c, d, fractX), fractZ)
}

func (t *HeightInfo) IsWater(worldX, worldZ int) bool {
	tileX := int(math.Floor(float64(worldX) / 3600))
	tileZ := int(math.Floor(float64(-worldZ) / 3600)) // negated
	localX := worldX - tileX*3600
	localZ := (-worldZ) - tileZ*3600

	h, ok := t.tileMap[spatial.MapCoord{Lon: tileX, Lat: tileZ}]
	if !ok {
		return false
	}
	return h.waterData[localZ*MAP_LENGTH+localX] > 0
}

func (t *HeightInfo) sample(worldX, worldZ int) float32 {
	tileX := int(math.Floor(float64(worldX) / 3600))
	tileZ := int(math.Floor(float64(-worldZ) / 3600)) // negated
	localX := worldX - tileX*3600
	localZ := (-worldZ) - tileZ*3600

	h, ok := t.tileMap[spatial.MapCoord{Lon: tileX, Lat: tileZ}]
	if !ok {
		return 0.0
	}
	return h.getLocal(localX, localZ)
}

func (h *chunkHeightInfo) getLocal(x, z int) float32 {
	if (x < 0 || x >= MAP_LENGTH) || (z < 0 || z >= MAP_LENGTH) {
		return 0.0
	}

	value := h.heightData[z*MAP_LENGTH+x]
	// normalize from 0-65535 to -32768 to 32767
	return float32(int(value)-32768) * HEIGHT_SCALE
}

func lerp(a, b float32, t float64) float32 {
	return a + (b-a)*float32(t)
}
