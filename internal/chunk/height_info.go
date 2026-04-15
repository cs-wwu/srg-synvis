package chunk

import "math"

const (
	HEIGHT_SCALE = 60.0 / 1852.0 // Meters to Arcseconds
	MAP_LENGTH   = 3601
)

type HeightInfo struct {
	heightData *[MAP_LENGTH * MAP_LENGTH]uint16
}

func NewHeightInfo(data []uint16) *HeightInfo {
	return &HeightInfo{
		heightData: (*[MAP_LENGTH * MAP_LENGTH]uint16)(data),
	}
}

func (h *HeightInfo) Height(x, z float32) float32 {
	localX := math.Mod(math.Mod(float64(x), 3600)+3600, 3600)
	localZ := math.Mod(math.Mod(float64(z), 3600)+3600, 3600)

	a := h.getLocal(int(localX), int(localZ))
	b := h.getLocal(int(localX+1), int(localZ))
	c := h.getLocal(int(localX), int(localZ+1))
	d := h.getLocal(int(localX+1), int(localZ+1))

	_, fractX := math.Modf(localX)
	_, fractZ := math.Modf(localZ)

	return lerp(lerp(a, b, fractX), lerp(c, d, fractX), fractZ)
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
