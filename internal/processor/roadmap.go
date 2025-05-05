package processor

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	Scale     = 1.5
	OSMURL    = "http://overpass-api.de/api/interpreter"
)

// Definition of each road type:
// https://wiki.openstreetmap.org/wiki/International_highway_classification_equivalence
var widths = map[string]float64{
	"motorway":  8,
	"trunk":     6,
	"primary":   4,
	"secondary": 2,
	"tertiary":  1,
}

// Creates an array of roads: [road#][point#][x, y]
func ConvertCoordsToPngIndex(highways []Element, nodes map[int64]Element, scale float32, lat int, lon int) [][]Point {
	roads := [][]Point{}
	c := float64(scale)
	for i := range highways {
		tempRoad := []Point{}
		for j := range highways[i].Nodes {
			node := nodes[highways[i].Nodes[j]]
			// Transforms the lat/lon into x/y coordinates
			y := math.Round((1 - (math.Abs(node.Lat) - math.Abs(float64(lat)))) * (c * ImgLength))
			x := math.Round((1 - (math.Abs(node.Lon) - math.Abs(float64(lon)+1))) * (c * ImgLength))
			index := Point{x, y}
			tempRoad = append(tempRoad, index)
		}
		roads = append(roads, tempRoad)
	}
	return roads
}

// Point represents a 2D point with X and Y coordinates
type Point struct {
	X, Y float64
}

// DrawLine creates a new image and draws a black line between two points with specified width
// Parameters:
//   - start: starting point of the line
//   - end: ending point of the line
//   - width: width of the line in pixels
//   - img: img to be drawn too
//
// Return:
//   - updates img with the line drawn between two points
func DrawLine(start, end Point, width float64, img *image.RGBA, color color.Color) {
	// Calculate line parameters
	dx := end.X - start.X
	dy := end.Y - start.Y
	length := math.Sqrt(dx*dx + dy*dy)

	// Normalize direction vector
	if length > 0 {
		dx /= length
		dy /= length
	}

	// Calculate perpendicular vector for line thickness
	perpX := -dy * (width / 2)
	perpY := dx * (width / 2)

	// Create corners of the thick line
	corners := []Point{
		{start.X + perpX, start.Y + perpY},
		{start.X - perpX, start.Y - perpY},
		{end.X - perpX, end.Y - perpY},
		{end.X + perpX, end.Y + perpY},
	}

	// Draw filled polygon
	DrawFilledPolygon(img, corners, color)
}

// drawFilledPolygon draws a filled polygon with the specified color
func DrawFilledPolygon(img *image.RGBA, points []Point, c color.Color) {
	// Find bounding box
	minX := points[0].X
	minY := points[0].Y
	maxX := points[0].X
	maxY := points[0].Y

	for _, p := range points {
		minX = math.Min(minX, p.X)
		minY = math.Min(minY, p.Y)
		maxX = math.Max(maxX, p.X)
		maxY = math.Max(maxY, p.Y)
	}

	// Check each pixel in the bounding box
	for y := int(minY); y <= int(maxY); y++ {
		for x := int(minX); x <= int(maxX); x++ {
			if IsPointInPolygon(float64(x), float64(y), points) {
				img.Set(x, y, c)
			}
		}
	}
}

// isPointInPolygon determines if a point is inside a polygon using ray casting algorithm
func IsPointInPolygon(x, y float64, points []Point) bool {
	inside := false
	j := len(points) - 1

	for i := 0; i < len(points); i++ {
		if ((points[i].Y > y) != (points[j].Y > y)) &&
			(x < (points[j].X-points[i].X)*(y-points[i].Y)/(points[j].Y-points[i].Y)+points[i].X) {
			inside = !inside
		}
		j = i
	}

	return inside
}

// Make an api request to overpass-api, process the data into and overpassResponse data type
func MakeApiRequest(apiUrl string, maps []int) (OverpassResponse, error) {
	lat := maps[0]
	lon := maps[1]
	bounds := "(" + strconv.Itoa(lat) + "," + strconv.Itoa(lon) + "," + strconv.Itoa(lat+1) + "," + strconv.Itoa(lon+1) + ");"
	query := `
	[out:json];
	(
		way["highway"="motorway"]` + bounds +
		`way["highway"="trunk"]` + bounds +
		`way["highway"="primary"]` + bounds +
		`way["highway"="secondary"]` + bounds +
		`way["highway"="tertiary"]` + bounds +
		`);
	(._;>;);
	out body;
	`

	var osmData OverpassResponse
	resp, err := http.Post(apiUrl, "application/x-www-form-urlencoded", strings.NewReader("data="+query))
	if err != nil {
		return osmData, fmt.Errorf("failed to make post request to %v: %w", apiUrl, err)
	}
	defer resp.Body.Close()

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

func DrawRoads(ways []Element, nodes map[int64]Element, coords []int, grayImage *image.RGBA, roadType string) {
	// Turn the lat/lon to an arry of 3600 indexed float32
	roads := ConvertCoordsToPngIndex(ways, nodes, Scale, coords[0], coords[1])

	// Connect all the points
	for r := range roads {
		road := roads[r]
		for p := 0; p < len(road)-1; p++ {
			p1 := road[p]
			p2 := road[p+1]
			DrawLine(p1, p2, widths[roadType], grayImage, color.Opaque)
			DrawLine(p1, p2, widths[roadType], grayImage, color.Opaque)
		}
	}
}

func GenerateOsmPng(maps [][]int, targetDir string, apiURL string) error {
	if maps == nil || targetDir == "" {
		return fmt.Errorf("no maps or targetDir specified")
	}

	for i := range maps {

		osmData, err := MakeApiRequest(apiURL, maps[i])
		if err != nil {
			return err
		}

		// Filter the data to keep only relevant highways
		var motorways, trunks, primaries, secondaries, tertiaries []Element
		nodes := make(map[int64]Element)

		for _, Element := range osmData.Elements {
			// Sorts the roads based on type, and stores the nodes in a map for easy access by ID
			if Element.Type == "way" && Element.Tags["highway"] == "motorway" {
				motorways = append(motorways, Element)
			} else if Element.Type == "way" && Element.Tags["highway"] == "trunk" {
				trunks = append(trunks, Element)
			} else if Element.Type == "way" && Element.Tags["highway"] == "primary" {
				primaries = append(primaries, Element)
			} else if Element.Type == "way" && Element.Tags["highway"] == "secondary" {
				secondaries = append(secondaries, Element)
			} else if Element.Type == "way" && Element.Tags["highway"] == "tertiary" {
				tertiaries = append(tertiaries, Element)
			} else if Element.Type == "node" {
				nodes[Element.ID] = Element
			}
		}

		// Generate an all white PNG
		grayImage := image.NewRGBA(image.Rect(0, 0, Scale*ImgLength, Scale*ImgLength))
		for x := 0; x < ImgLength*Scale; x++ {
			for y := 0; y < ImgLength*Scale; y++ {
				grayImage.Set(x, y, color.Transparent)
				grayImage.Set(x, y, color.Transparent)
			}
		}

		// Draw each set of roads
		DrawRoads(motorways, nodes, maps[i], grayImage, "motorway")
		DrawRoads(trunks, nodes, maps[i], grayImage, "trunk")
		DrawRoads(primaries, nodes, maps[i], grayImage, "primary")
		DrawRoads(secondaries, nodes, maps[i], grayImage, "secondary")
		DrawRoads(tertiaries, nodes, maps[i], grayImage, "tertiary")

		bounds := CoordToString(maps[i][0], maps[i][1])
		pngFilepath := targetDir + bounds + ".osm.png"
		pngFile, err := os.Create(pngFilepath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", pngFilepath, err)
		}
		defer pngFile.Close()

		if err := encodePNG(pngFile, grayImage); err != nil {
			return fmt.Errorf("failed to encode %s: %w", pngFilepath, err)
		}
	}

	return nil
}
