package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pb "github.com/cs-wwu/srg-synvis/assets/chunks/roads"
	"google.golang.org/protobuf/proto"
)

// Define structs for Overpass API response data
type Element struct {
	ID    int64             `json:"id"`
	Type  string            `json:"type"`
	Tags  map[string]string `json:"tags"`
	Lat   float64           `json:"lat,omitempty"`
	Lon   float64           `json:"lon,omitempty"`
	Nodes []int64           `json:"nodes,omitempty"`
}

type OverpassResponse struct {
	Elements []Element `json:"elements"`
}

const (
	ImgLength = 3600
	Scale     = 1.0
	OSMURL    = "http://overpass-api.de/api/interpreter"
)

// Definition of each road type:
// https://wiki.openstreetmap.org/wiki/International_highway_classification_equivalence
//
// TODO: Move to main package and render at runtime
var widths = map[string]float64{
	// Highway types:
	"motorway":  8,
	"trunk":     6,
	"primary":   4,
	"secondary": 2,
	"tertiary":  1,
	// Railway types:
	"rail":       3,
	"light_rail": 2,
	"tram":       1,
	// Aeroway types:
	"runway":   6,
	"airstrip": 5,
	"taxiway":  2,
}

var ErrRateLimited = errors.New("rate limited by OSM")

// Make an api request to overpass-api, process the data into and overpassResponse data type
func MakeRoadApiRequest(lat int, lon int) (OverpassResponse, error) {
	bounds := "(" + strconv.Itoa(lat) + "," + strconv.Itoa(lon) + "," + strconv.Itoa(lat+1) + "," + strconv.Itoa(lon+1) + ");"
	query := `
	[out:json];
	(
		way["highway"="motorway"]` + bounds +
		`way["highway"="trunk"]` + bounds +
		`way["highway"="primary"]` + bounds +
		`way["highway"="secondary"]` + bounds +
		`way["highway"="tertiary"]` + bounds +
		`way["railway"="rail"]` + bounds +
		`way["railway"="light_rail"]` + bounds +
		`way["railway"="tram"]` + bounds +
		`way["aeroway"="runway"]` + bounds +
		`way["aeroway"="airstrip"]` + bounds +
		`way["aeroway"="taxiway"]` + bounds +
		`);
	(._;>;);
	out body;
	`
	var osmData OverpassResponse
	resp, err := http.Post(OSMURL, "application/x-www-form-urlencoded", strings.NewReader("data="+query))
	if err != nil {
		return osmData, fmt.Errorf("failed to make post request to %v: %w", OSMURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return osmData, ErrRateLimited
	} else if resp.StatusCode != http.StatusOK {
		return osmData, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return osmData, fmt.Errorf("error reading response: %w", err)
	}

	// Parse JSON response

	err = json.Unmarshal(body, &osmData)
	if err != nil {
		return osmData, fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	return osmData, nil
}

// CoordToString takes in the integer representation of the coordinates and
// returns a string in format "(lat)s(lon)e"
// Parameters:
//   - lat, lon: integer representations of lattitude and longitude
//
// Return:
//   - ret: a string representation of lat & lon
func CoordToString(lat, lon int) string {
	var latDir, lonDir rune

	// 0 lat is n00
	if lat >= 0 {
		latDir = 'n'
	} else {
		latDir = 's'
		lat = -lat // remove negative
	}

	// 0 lon is e000
	if lon >= 0 {
		lonDir = 'e'
	} else {
		lonDir = 'w'
		lon = -lon // remove negative
	}

	return fmt.Sprintf("%c%02d%c%03d", latDir, lat, lonDir, lon)
}

// ParseHeightmapToCoords converts a heightmap string (e.g., "n48w123")
// to a coordinate pair [lat, lon]
func ParseHeightmapToCoords(heightmap string) ([]int, error) {
	// Check if the string format is valid
	if len(heightmap) != 7 {
		return nil, fmt.Errorf("invalid heightmap format: %s", heightmap)
	}

	// Extract direction characters and numeric values
	latDir := heightmap[0]
	lonDir := heightmap[3]

	lat, err := strconv.Atoi(heightmap[1:3])
	if err != nil {
		return nil, fmt.Errorf("failed to parse latitude: %w", err)
	}

	lon, err := strconv.Atoi(heightmap[4:7])
	if err != nil {
		return nil, fmt.Errorf("failed to parse longitude: %w", err)
	}

	// Apply the correct sign based on direction
	if latDir == 's' {
		lat = -lat
	}

	if lonDir == 'w' {
		lon = -lon
	}

	return []int{lat, lon}, nil
}

func GenerateRoadProtobufs(heightmaps []string) error {
	outputDir := "assets/chunks/roads/data"
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating output directory %s: %v", outputDir, err)
	}

	// OSM takes a couple seconds per request, so process them in sequence.
	// Doing them in parallel hits all the rate limits.
	//
	// See https://dev.overpass-api.de/overpass-doc/en/preface/commons.html
	for _, heightmap := range heightmaps {
		// Convert the heightmap string to a coordinate pair
		coords, err := ParseHeightmapToCoords(heightmap)
		if err != nil {
			log.Printf("Warning: skipping invalid heightmap %s: %v", heightmap, err)
			continue
		}

		err = generateRoadSquare(outputDir, coords[0], coords[1])
		if err != nil {
			log.Printf("Error generating road square for %s: %v", heightmap, err)
			continue
		}
	}

	return nil
}

func generateRoadSquare(outputDir string, lat int, lon int) error {
	attempts := 0
	delay := 1.0
	var osmData OverpassResponse
	var err error
	for attempts < 10 {
		log.Printf("Posting request to osm at %v,%v...\n", lat, lon)
		osmData, err = MakeRoadApiRequest(lat, lon)
		if err == nil {
			break
		} else if errors.Is(err, ErrRateLimited) {
			log.Printf("Rate limit from OSM, retrying in %v\n", time.Duration(delay)*time.Second)
			time.Sleep(time.Duration(delay) * time.Second)
			delay *= 2.0
			attempts++
		} else {
			return err
		}
	}
	if err != nil {
		return err
	}

	nodes := make(map[int64]Element)

	square := pb.RoadSquare{
		LatDeg: int32(lat),
		LonDeg: int32(lon),
		Roads:  make([]*pb.Road, 0),
	}

	for _, element := range osmData.Elements {
		// Store nodes for easy access by ID
		if element.Type == "node" {
			nodes[element.ID] = element
			continue
		}

		// Sort roads and railways by type
		if element.Type == "way" {
			name, err := elementName(&element)
			if err != nil {
				log.Printf("Warning: invalid way %v: %v", element.ID, err)
				continue
			}

			points := make([]*pb.Point, 0, len(element.Nodes))
			for _, nodeID := range element.Nodes {
				node, exists := nodes[nodeID]
				if !exists {
					log.Printf("Warning: node %d not found for way %d", nodeID, element.ID)
					continue
				}

				points = append(points, &pb.Point{
					Lat: node.Lat,
					Lon: node.Lon,
				})
			}

			square.Roads = append(square.Roads, &pb.Road{
				Type:   name,
				Points: points,
			})
		}
	}

	data, err := proto.Marshal(&square)
	if err != nil {
		return fmt.Errorf("error marshaling all data: %v", err)
	}

	path := filepath.Join(outputDir, CoordToString(int(square.LatDeg), int(square.LonDeg))+".pb")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error writing all data file: %v", err)
	}

	return nil
}

func elementName(element *Element) (name string, err error) {
	// Handle highways
	if highway, exists := element.Tags["highway"]; exists {
		name = highway
		return
	}

	if aeroway, exists := element.Tags["aeroway"]; exists {
		name = aeroway
		return
	}

	// Handle railways
	if railway, exists := element.Tags["railway"]; exists {
		name = railway
		return
	}

	err = fmt.Errorf("element %v has no valid name", element.ID)
	return
}
