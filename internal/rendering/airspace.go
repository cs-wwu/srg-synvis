package rendering

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
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

const outDir = "assets/chunks/airspace"

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

func ChunkGeoJson(inputFile string) {
	fileData, err := FileToBytesChecked(inputFile)
	if err != nil {
		log.Printf("ChunkGeoJson: error opening GeoJson file %s: %v", inputFile, err)
		return
	}
	var geojson GeoJSON
	err = json.Unmarshal(fileData, &geojson)
	if err != nil {
		log.Printf("ChunkGeoJson: error unmarshaling GeoJson file %s: %v", inputFile, err)
		return
	}
	// Make sure output directory exists ONCE
	dataDir := filepath.Join(outDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("error creating output directory: %v", err)
		return
	}
	features := geojson.Features
	for _, feature := range features {
		avgLat, avgLon := determineChunk(feature.Geometry)
		// Use absolute values and proper formatting

		chunkFileName := fmt.Sprintf("%s%v%s%v.geojson", "n", math.Abs(float64(avgLat)), "w", math.Abs(float64(avgLon)))
		chunkFilePath := filepath.Join(dataDir, chunkFileName)
		// Read existing FeatureCollection or create new one
		var fc FeatureCollection
		if _, err := os.Stat(chunkFilePath); os.IsNotExist(err) {
			// File doesn't exist → start with proper FeatureCollection structure
			fc = FeatureCollection{
				Type:     "FeatureCollection",
				Features: []Feature{},
			}
			fmt.Printf("Creating new file: %s\n", chunkFilePath)
		} else {
			// File exists → read and unmarshal existing JSON
			data, err := os.ReadFile(chunkFilePath)
			if err != nil {
				log.Printf("cannot read existing file %s: %v", chunkFilePath, err)
				continue
			}
			if err := json.Unmarshal(data, &fc); err != nil {
				log.Printf("cannot unmarshal existing file %s: %v", chunkFilePath, err)
				log.Printf("File content preview: %s", string(data[:min(200, len(data))]))
				continue
			}
			fmt.Printf("Appending to existing file: %s\n", chunkFilePath)
		}
		// Add feature if it doesn't already exist
		if !containsFeature(&fc, feature) {
			fc.Features = append(fc.Features, feature)
			fmt.Printf("  Added feature: %s (total: %d)\n",
				feature.Properties.Name, len(fc.Features))
		} else {
			fmt.Printf("  Feature already exists: %s\n", feature.Properties.Name)
		}
		// Marshal ENTIRE FeatureCollection to JSON
		outData, err := json.MarshalIndent(fc, "", "  ")
		if err != nil {
			log.Printf("cannot marshal FeatureCollection: %v", err)
			continue
		}
		// Write complete JSON to file (overwrites entire file)
		if err := os.WriteFile(chunkFilePath, outData, 0644); err != nil {
			log.Printf("cannot write to file %s: %v", chunkFilePath, err)
			continue
		}
	}
}
func containsFeature(fc *FeatureCollection, f Feature) bool {
	for _, existing := range fc.Features {
		if existing.Properties.GlobalId == f.Properties.GlobalId {
			return true
		}
	}
	return false
}
func determineChunk(coords Geometry) (lat, lon int) {
	if len(coords.Coordinates) == 0 || len(coords.Coordinates[0]) == 0 {
		return 0, 0
	}
	outerRing := coords.Coordinates[0]
	n := len(outerRing)
	// Skip the last point if it's a closed ring (first == last)
	if n > 1 && outerRing[0][0] == outerRing[n-1][0] &&
		outerRing[0][1] == outerRing[n-1][1] {
		n = n - 1
	}
	if n == 0 {
		return 0, 0
	}
	latTotal := 0.0
	lonTotal := 0.0
	for i := 0; i < n; i++ {
		// GeoJSON is [longitude, latitude, altitude]
		lonTotal += outerRing[i][0]
		latTotal += outerRing[i][1]
	}
	// Return as integers (floor)
	return int(math.Floor(latTotal / float64(n))),
		int(math.Floor(lonTotal / float64(n)))
}
