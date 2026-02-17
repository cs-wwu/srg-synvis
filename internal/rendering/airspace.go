package rendering

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	as "github.com/vulkan-go/asche"
)

type AirspaceProperties struct {
	Name            string `json:"NAME"`
	Class           string `json:"CLASS"`
	AltitudeCeiling int    `json:"UPPER_VAL"` // Altitude ceiling
	AltitudeFloor   int    `json:"LOWER_VAL"` // Altitude floor
	GlobalId        string `json:"GLOBAL_ID"`
}

type Feature struct {
	Type       string             `json:"type"`
	Properties AirspaceProperties `json:"properties"`
	Geometry   Geometry           `json:"geometry"`
}

type Geometry struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

type AirspaceBuilder struct {
	heightInfo *HeightInfo
	vertices   []AirspaceVertex
	indices    []uint32
	color      [4]float32
}

type AirspaceVertex struct {
	Position [3]float32
	Color    [4]float32
}

type Airspace struct {
	vertices   *as.Buffer
	indices    *as.Buffer
	instances  *as.Buffer
	indexCount uint32
	features   []Feature
}

type AirspaceInstance struct {
	LowerAlt float32
	UpperAlt float32
	Color    [4]float32
}

type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

const AIRSPACE_ALTITUDE_OFFSET = float32(100.0)

func newAirspaceBuilder(heightInfo *HeightInfo) *AirspaceBuilder {
	return &AirspaceBuilder{
		heightInfo: heightInfo,
		vertices:   make([]AirspaceVertex, 0),
		indices:    make([]uint32, 0),
	}
}

func (air *AirspaceBuilder) addVertex(vertex AirspaceVertex) (id uint32) {
	id = uint32(len(air.vertices))
	air.vertices = append(air.vertices, vertex)
	return
}

func (air *AirspaceBuilder) addTriangle(a, b, c uint32) {
	air.indices = append(air.indices, a, b, c)
}

func (air *AirspaceBuilder) vertexBytes() []byte {
	if len(air.vertices) == 0 {
		return nil
	}

	size := len(air.vertices) * int(unsafe.Sizeof(AirspaceVertex{}))
	return unsafe.Slice((*byte)(unsafe.Pointer(&air.vertices[0])), size)
}

func (air *AirspaceBuilder) indexBytes() []byte {
	if len(air.indices) == 0 {
		return nil
	}

	size := len(air.indices) * int(unsafe.Sizeof(uint32(0)))
	return unsafe.Slice((*byte)(unsafe.Pointer(&air.indices[0])), size)
}

func toWorldHeightAirspace(height float32) float32 {
	return float32((height * 0.3048) * (ARC_DEGREES_TO_MINUTES / 1852)) // feet to meters to arcseconds
}

func renderAirspace(builder *AirspaceBuilder, feature Feature, color [4]float32) {
	coords := feature.Geometry
	props := feature.Properties
	if len(coords.Coordinates) == 0 {
		return
	}
	outerRing := coords.Coordinates[0]
	if len(outerRing) < 3 {
		return
	}
	n := len(outerRing) - 1
	bottomIds := make([]uint32, n)
	topIds := make([]uint32, n)

	for i := range n {
		lon, lat := outerRing[i][0], outerRing[i][1]
		x := float32(lon * ARC_DEGREES_TO_SECONDS)
		y := float32(-lat * ARC_DEGREES_TO_SECONDS)
		height := builder.heightInfo.Height(x, y)
		lowerAlt := toWorldHeightAirspace(float32(props.AltitudeFloor + int(height) + 10))
		upperAlt := toWorldHeightAirspace(float32(props.AltitudeCeiling))

		bottomIds[i] = builder.addVertex(AirspaceVertex{
			Position: [3]float32{float32(x), float32(lowerAlt), float32(y)},
			Color:    [4]float32(color),
		})
		topIds[i] = builder.addVertex(AirspaceVertex{
			Position: [3]float32{float32(x), float32(upperAlt), float32(y)},
			Color:    [4]float32(color),
		})
	}

	// Ear clipping triangulation for bottom and top caps
	bottomTriangles := earClip(outerRing[:n])
	for i := 0; i < len(bottomTriangles); i += 3 {
		// Bottom cap - both sides
		builder.addTriangle(bottomIds[bottomTriangles[i]],
			bottomIds[bottomTriangles[i+1]],
			bottomIds[bottomTriangles[i+2]])
		builder.addTriangle(bottomIds[bottomTriangles[i]],
			bottomIds[bottomTriangles[i+2]],
			bottomIds[bottomTriangles[i+1]])

		// Top cap - both sides
		builder.addTriangle(topIds[bottomTriangles[i]],
			topIds[bottomTriangles[i+1]],
			topIds[bottomTriangles[i+2]])
		builder.addTriangle(topIds[bottomTriangles[i]],
			topIds[bottomTriangles[i+2]],
			topIds[bottomTriangles[i+1]])
	}

	// Build walls - both sides
	for i := range n {
		nextI := (i + 1) % n
		// Outside face
		builder.addTriangle(bottomIds[i], topIds[i], bottomIds[nextI])
		builder.addTriangle(topIds[i], topIds[nextI], bottomIds[nextI])
		// Inside face
		builder.addTriangle(bottomIds[i], bottomIds[nextI], topIds[i])
		builder.addTriangle(topIds[i], bottomIds[nextI], topIds[nextI])
	}
}

// earClip performs ear clipping triangulation on a 2D polygon
// Returns indices into the original vertex array as triplets (i1,i2,i3, i4,i5,i6, ...)
func earClip(ring [][]float64) []uint32 {
	if len(ring) < 3 {
		return nil
	}

	indices := []uint32{}
	remaining := make([]int, len(ring))
	for i := range remaining {
		remaining[i] = i
	}

	// Keep clipping ears until we have a triangle
	for len(remaining) > 3 {
		earFound := false

		for i := 0; i < len(remaining); i++ {
			prevIdx := (i - 1 + len(remaining)) % len(remaining)
			nextIdx := (i + 1) % len(remaining)

			prev := remaining[prevIdx]
			curr := remaining[i]
			next := remaining[nextIdx]

			if isEar(ring, prev, curr, next, remaining) {
				indices = append(indices, uint32(prev), uint32(curr), uint32(next))
				remaining = append(remaining[:i], remaining[i+1:]...)
				earFound = true
				break
			}
		}

		if !earFound {
			// Degenerate polygon - just do fan triangulation as fallback
			for i := 1; i < len(remaining)-1; i++ {
				indices = append(indices, uint32(remaining[0]), uint32(remaining[i]), uint32(remaining[i+1]))
			}
			break
		}
	}

	// Add final triangle
	if len(remaining) == 3 {
		indices = append(indices, uint32(remaining[0]), uint32(remaining[1]), uint32(remaining[2]))
	}

	return indices
}

// isEar checks if a vertex forms an ear (can be clipped)
func isEar(ring [][]float64, prev, curr, next int, remaining []int) bool {
	p0 := ring[prev]
	p1 := ring[curr]
	p2 := ring[next]

	// Check if triangle is convex (cross product > 0)
	cross := (p1[0]-p0[0])*(p2[1]-p0[1]) - (p1[1]-p0[1])*(p2[0]-p0[0])
	if cross >= 0 {
		return false // Reflex vertex, not an ear
	}

	// Check if any other vertex is inside this triangle
	for _, idx := range remaining {
		if idx == prev || idx == curr || idx == next {
			continue
		}
		if pointInTriangle(ring[idx], p0, p1, p2) {
			return false
		}
	}

	return true
}

// pointInTriangle checks if point p is inside triangle (a, b, c)
func pointInTriangle(p, a, b, c []float64) bool {
	// Use barycentric coordinates
	v0x := c[0] - a[0]
	v0y := c[1] - a[1]
	v1x := b[0] - a[0]
	v1y := b[1] - a[1]
	v2x := p[0] - a[0]
	v2y := p[1] - a[1]

	dot00 := v0x*v0x + v0y*v0y
	dot01 := v0x*v1x + v0y*v1y
	dot02 := v0x*v2x + v0y*v2y
	dot11 := v1x*v1x + v1y*v1y
	dot12 := v1x*v2x + v1y*v2y

	invDenom := 1.0 / (dot00*dot11 - dot01*dot01)
	u := (dot11*dot02 - dot01*dot12) * invDenom
	v := (dot00*dot12 - dot01*dot02) * invDenom

	return (u >= 0) && (v >= 0) && (u+v < 1)
}

func isInsideAirspace(pos [3]float32, features []Feature) *Feature {
	altitude := int(pos[1])

	// Convert world coords to lat/lon
	var lat, lon float64
	fmt.Sscanf(coords.WorldToGPS(pos[0], pos[2]), "%f, %f", &lat, &lon)

	for i := range features {
		// Check altitude bounds first
		low := int(math.Min(float64(toWorldHeightAirspace(float32(features[i].Properties.AltitudeFloor))), float64(toWorldHeightAirspace(float32(features[i].Properties.AltitudeCeiling)))))
		high := int(math.Max(float64(toWorldHeightAirspace(float32(features[i].Properties.AltitudeFloor))), float64(toWorldHeightAirspace(float32(features[i].Properties.AltitudeCeiling)))))

		if altitude < low || altitude > high {
			continue
		}

		// 2D point-in-polygon check (lon/x, lat/y)
		outer := features[i].Geometry.Coordinates[0] // type [][]float64
		if pointInRing(lon, lat, outer) {
			return &features[i] // Inside this airspace
		}
	}

	return nil
}

func pointInRing(x, y float64, ring [][]float64) bool {
	n := len(ring)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := ring[i][0], ring[i][1] // a point
		xj, yj := ring[j][0], ring[j][1] // another point
		if ((yi > y) != (yj > y)) && (x < (xj-xi)*(y-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

func (app *Application) UpdateCurrentAirspace() {
	cameraPos := app.camera.Position()

	// Get all loaded airspace chunks
	loadedChunks := app.AirspaceStreamer.GetLoaded()

	// Check features in all loaded chunks
	for _, chunk := range loadedChunks {
		if chunk == nil {
			continue
		}
		if inside := isInsideAirspace(cameraPos, chunk.features); inside != nil {
			app.currentAirspace = inside
			fmt.Printf("Current airspace: %s\n", inside.Properties.Name)
			return
		}
	}

	app.currentAirspace = nil
}
