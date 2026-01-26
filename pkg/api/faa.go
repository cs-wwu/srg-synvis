package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	air "github.com/cs-wwu/srg-synvis/internal/rendering"
)

const (
	AIRSPACE_BASE_URL = "https://services6.arcgis.com/ssFJjBXIUyZDrSYZ/arcgis/rest/services/Class_Airspace/FeatureServer/0/query"
)
const outDir = "assets/chunks"

var validStates = map[string]bool{
	// States
	"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true, "CO": true,
	"CT": true, "DE": true, "FL": true, "GA": true, "HI": true, "ID": true,
	"IL": true, "IN": true, "IA": true, "KS": true, "KY": true, "LA": true,
	"ME": true, "MD": true, "MA": true, "MI": true, "MN": true, "MS": true,
	"MO": true, "MT": true, "NE": true, "NV": true, "NH": true, "NJ": true,
	"NM": true, "NY": true, "NC": true, "ND": true, "OH": true, "OK": true,
	"OR": true, "PA": true, "RI": true, "SC": true, "SD": true, "TN": true,
	"TX": true, "UT": true, "VT": true, "VA": true, "WA": true, "WV": true,
	"WI": true, "WY": true,

	// Territories
	"AS": true, "GU": true, "MP": true, "PR": true, "VI": true, "DC": true,
}

var validClasses = map[string]bool{
	"A": true, "B": true, "C": true, "D": true, "E": true, "F": true, "G": true,
}

type AirspaceFilter struct {
	States  []string // e.g., ["WA", "OR", "CA"] - empty means all states
	Classes []string // e.g., ["D", "C", "B"] - empty means all classes
}

// Esri API Response structures
type EsriResponse struct {
	ObjectIDFieldName string           `json:"objectIdFieldName"`
	GeometryType      string           `json:"geometryType"`
	SpatialReference  SpatialReference `json:"spatialReference"`
	Fields            []Field          `json:"fields"`
	Features          []EsriFeature    `json:"features"`
}

type SpatialReference struct {
	WKID       int `json:"wkid"`
	LatestWKID int `json:"latestWkid"`
}

type Field struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Alias string `json:"alias"`
}

type EsriFeature struct {
	Attributes map[string]interface{} `json:"attributes"`
	Geometry   EsriGeometry           `json:"geometry"`
}

type EsriGeometry struct {
	Rings [][][]float64 `json:"rings,omitempty"` // For polygons
	Paths [][][]float64 `json:"paths,omitempty"` // For polylines
	X     *float64      `json:"x,omitempty"`     // For points
	Y     *float64      `json:"y,omitempty"`
	Z     *float64      `json:"z,omitempty"`
}

func DownloadAndChunkAirspaces(filter *AirspaceFilter) error {
	if filter == nil {
		filter = &AirspaceFilter{} // No filters, get everything
	}

	// Build query URL
	queryURL := filter.BuildQueryURL()
	fmt.Printf("Querying: %s\n", queryURL)

	res, err := http.Get(queryURL)
	if err != nil {
		return fmt.Errorf("error getting airspace data: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with code %d", res.StatusCode)
	}

	responseData, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	// Parse Esri response
	var esriData EsriResponse
	err = json.Unmarshal(responseData, &esriData)
	if err != nil {
		return fmt.Errorf("error parsing Esri response: %w", err)
	}

	// Convert to FeatureCollection
	featureCollection, err := convertEsriToFeatureCollection(esriData)
	if err != nil {
		return fmt.Errorf("error converting to FeatureCollection: %w", err)
	}

	fmt.Printf("Successfully downloaded and converted %d airspace features\n", len(featureCollection.Features))

	// Chunk the data
	ChunkGeoJson(featureCollection)

	return nil
}

func ChunkGeoJson(geojson air.FeatureCollection) {
	dataDir := filepath.Join(outDir, "/airspace/data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("error creating output directory: %v", err)
		return
	}

	features := geojson.Features
	for _, feature := range features {

		// Find where most of chunk lies to determine where it goes
		avgLat, avgLon := determineChunk(feature.Geometry)
		chunkFileName := fmt.Sprintf("%s%v%s%v.geojson", "n", math.Abs(float64(avgLat)), "w", math.Abs(float64(avgLon)))
		chunkFilePath := filepath.Join(dataDir, chunkFileName)

		// Load existing chunk file or create a new one
		var fc air.FeatureCollection
		if _, err := os.Stat(chunkFilePath); os.IsNotExist(err) {
			fc = air.FeatureCollection{
				Type:     "FeatureCollection",
				Features: []air.Feature{},
			}
			fmt.Printf("Creating new file: %s\n", chunkFilePath)
		} else {
			data, err := os.ReadFile(chunkFilePath)
			if err != nil {
				log.Printf("cannot read existing file %s: %v", chunkFilePath, err)
				continue
			}
			if err := json.Unmarshal(data, &fc); err != nil {
				log.Printf("cannot unmarshal existing file %s: %v", chunkFilePath, err)
				continue
			}
			fmt.Printf("Appending to existing file: %s\n", chunkFilePath)
		}

		// Add feature if it doesn't already exist based on global ID
		if !containsFeature(&fc, feature) {
			fc.Features = append(fc.Features, feature)
			fmt.Printf("  Added feature: %s (total: %d)\n",
				feature.Properties.Name, len(fc.Features))
		} else {
			fmt.Printf("  Feature already exists: %s\n", feature.Properties.Name)
		}

		// Write updated FeatureCollection to chunk file
		outData, err := json.MarshalIndent(fc, "", "  ")
		if err != nil {
			log.Printf("cannot marshal FeatureCollection: %v", err)
			continue
		}
		if err := os.WriteFile(chunkFilePath, outData, 0644); err != nil {
			log.Printf("cannot write to file %s: %v", chunkFilePath, err)
			continue
		}
	}
}

// NewFilter creates a new filter
func NewAirspaceFilter(states, classes []string) (*AirspaceFilter, error) {

	// Check if state is valid
	for _, state := range states {
		if !validStates[state] {
			return nil, fmt.Errorf("invalid state/territory: '%s'", state)
		}
	}

	// Check if class is valid
	for _, class := range classes {
		if !validClasses[class] {
			return nil, fmt.Errorf("invalid airspace class: '%s'", class)
		}
	}

	return &AirspaceFilter{
		States:  states,
		Classes: classes,
	}, nil
}

// buildWhereQuery constructs the SQL style WHERE query
func (f *AirspaceFilter) buildWhereQuery() string {
	var conditions []string

	// State filter
	if len(f.States) > 0 {
		states := make([]string, len(f.States))
		for i, state := range f.States {
			states[i] = fmt.Sprintf("'%s'", state)
		}
		conditions = append(conditions, fmt.Sprintf("STATE IN (%s)", strings.Join(states, ",")))
	}

	// Class filter
	if len(f.Classes) > 0 {
		classes := make([]string, len(f.Classes))
		for i, class := range f.Classes {
			classes[i] = fmt.Sprintf("'%s'", class)
		}
		conditions = append(conditions, fmt.Sprintf("CLASS IN (%s)", strings.Join(classes, ",")))
	}

	if len(conditions) == 0 {
		return ""
	}

	return strings.Join(conditions, " AND ")
}

func (f *AirspaceFilter) BuildQueryURL() string {
	params := url.Values{}

	// Default to B, C, D classes if none specified
	if len(f.Classes) == 0 {
		f.Classes = []string{"B", "C", "D"}
	}

	// Build WHERE clause
	whereClause := f.buildWhereQuery()
	if whereClause == "" {
		whereClause = "1=1" // Return all records if no filters
	}
	params.Add("where", whereClause)

	// Always get all fields
	params.Add("outFields", "*")

	// Always use WGS84 coord system
	params.Add("outSR", "4326")

	// Format
	params.Add("f", "json")

	return AIRSPACE_BASE_URL + "?" + params.Encode()
}

func convertEsriToFeatureCollection(esriData EsriResponse) (air.FeatureCollection, error) {
	fc := air.FeatureCollection{
		Type:     "FeatureCollection",
		Features: make([]air.Feature, 0, len(esriData.Features)),
	}

	// Look through all features in the response and convert to Airspace Properties
	for i, esriFeature := range esriData.Features {
		geometry, err := convertGeometry(esriData.GeometryType, esriFeature.Geometry)
		if err != nil {
			return fc, fmt.Errorf("feature %d: %w", i, err)
		}

		// Get airspace properties from attributes and build airspace properties for new feature
		// Helper functions are used to handle nil cases
		properties := air.AirspaceProperties{
			Name:            getStringAttr(esriFeature.Attributes, "NAME"),
			Class:           getStringAttr(esriFeature.Attributes, "CLASS"),
			AltitudeCeiling: getIntAttr(esriFeature.Attributes, "UPPER_VAL"),
			AltitudeFloor:   getIntAttr(esriFeature.Attributes, "LOWER_VAL"),
			GlobalId:        getStringAttr(esriFeature.Attributes, "GLOBAL_ID"),
		}

		feature := air.Feature{
			Type:       "Feature",
			Properties: properties,
			Geometry:   geometry,
		}

		// Append to feature collection
		fc.Features = append(fc.Features, feature)
	}

	return fc, nil
}

func getStringAttr(attrs map[string]interface{}, key string) string {
	val := attrs[key]
	if val == nil {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

func getIntAttr(attrs map[string]interface{}, key string) int {
	val := attrs[key]
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

func convertGeometry(geometryType string, esriGeom EsriGeometry) (air.Geometry, error) {
	var geom air.Geometry

	// Simple switch statement to check if the esriGeom is supported
	// (response from esri should only be PolyGons for airspaces)
	switch geometryType {
	case "esriGeometryPolygon":
		if len(esriGeom.Rings) == 0 {
			return geom, fmt.Errorf("polygon has no rings")
		}

		geom.Type = "Polygon"
		geom.Coordinates = esriGeom.Rings
	default:
		return geom, fmt.Errorf("unsupported geometry type: %s", geometryType)
	}

	return geom, nil
}

func containsFeature(fc *air.FeatureCollection, f air.Feature) bool {
	for _, existing := range fc.Features {
		if existing.Properties.GlobalId == f.Properties.GlobalId {
			return true
		}
	}
	return false
}

func determineChunk(coords air.Geometry) (lat, lon int) {
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
