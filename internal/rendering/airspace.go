package rendering

import (
	"unsafe"

	as "github.com/vulkan-go/asche"
)

type GeoJSON struct {
	Features []Feature `json:"features"`
}

type Feature struct {
	Properties AirspaceProperties `json:"properties"`
	Geometry   Geometry           `json:"geometry"`
}

type AirspaceProperties struct {
	Name     string `json:"NAME"`
	Class    string `json:"CLASS"`
	UpperVal int    `json:"UPPER_VAL"` // Altitude ceiling
	LowerVal int    `json:"LOWER_VAL"` // Altitude floor
	GlobalId string `json:"GLOBAL_ID"`
}

type Geometry struct {
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

var outDir = "assets/chunks/airspace"

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
		lowerAlt := props.LowerVal + int(height) + 10
		upperAlt := props.UpperVal

		bottomIds[i] = builder.addVertex(AirspaceVertex{
			Position: [3]float32{float32(x), float32(lowerAlt), float32(y)},
			Color:    [4]float32(color),
		})

		topIds[i] = builder.addVertex(AirspaceVertex{
			Position: [3]float32{float32(x), float32(upperAlt), float32(y)},
			Color:    [4]float32(color),
		})
	}

	// Fan triangulation from first vertex
	for i := 1; i < n-1; i++ {
		builder.addTriangle(bottomIds[0], bottomIds[i], bottomIds[i+1])
		builder.addTriangle(topIds[0], topIds[i+1], topIds[i])
	}

	// build walls
	for i := 0; i < n; i++ {
		nextI := (i + 1) % n
		builder.addTriangle(bottomIds[i], topIds[i], bottomIds[nextI])
		builder.addTriangle(topIds[i], topIds[nextI], bottomIds[nextI])
	}
}
