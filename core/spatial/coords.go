package spatial

import "fmt"

func (m MapCoord) String() string {
	latDir := 'n'
	lat := m.Lat
	if lat < 0 {
		latDir = 's'
		lat = -lat
	}

	lonDir := 'e'
	lon := m.Lon
	if lon < 0 {
		lonDir = 'w'
		lon = -lon
	}

	return fmt.Sprintf("%c%02d%c%03d", latDir, lat, lonDir, lon)
}
