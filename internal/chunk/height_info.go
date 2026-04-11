package chunk

import (
	"math"

	"github.com/cs-wwu/srg-synvis/core/spatial"
)

const (
	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds
	MAP_LENGTH   = 3601
)

type TileCoord struct {
	X, Z int
}

type HeightInfoMap struct {
	tileMap map[TileCoord]HeightInfo
}

type HeightInfo struct {
	heightData *[MAP_LENGTH * MAP_LENGTH]uint16
}

func NewHeightInfo(data []uint16) *HeightInfo {
	return &HeightInfo{
		heightData: (*[MAP_LENGTH * MAP_LENGTH]uint16)(data),
	}
}

func (t *HeightInfoMap) AddTile(pos spatial.MapCoord, data []uint16) {
	t.tileMap[TileCoord{X: pos.Lon, Z: pos.Lat}] = *NewHeightInfo(data)
}

func NewHeightInfoMap(pos spatial.MapCoord, data []uint16) *HeightInfoMap {
	tileMap := make(map[TileCoord]HeightInfo)
	tileMap[TileCoord{X: pos.Lon, Z: pos.Lat}] = *NewHeightInfo(data)
	return &HeightInfoMap{tileMap: tileMap}
}

func (t *HeightInfoMap) Height(x, z float32) float32 {
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

func (t *HeightInfoMap) sample(worldX, worldZ int) float32 {
	tileX := int(math.Floor(float64(worldX) / 3600))
	tileZ := int(math.Floor(float64(-worldZ) / 3600)) // negated
	localX := worldX - tileX*3600
	localZ := (-worldZ) - tileZ*3600

	h, ok := t.tileMap[TileCoord{tileX, tileZ}]
	if !ok {
		return 0.0
	}
	return h.getLocal(localX, localZ)
}

func (h *HeightInfo) getLocal(x, z int) float32 {
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
