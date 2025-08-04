package processor

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
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
	Scale     = 1.0
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
	// add railway types
	"rail":       3,
	"light_rail": 2,
	"tram":       1,
	// add aeroway types
	"runway":   6,
	"airstrip": 5,
	"taxiway":  2,
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

// DrawDashedLine draws a dashed line for railways
// Parameters:
//   - start: starting point of the line
//   - end: ending point of the line
//   - width: width of the line in pixels
//   - img: img to be drawn too
//   - color: color to draw line
//   - dashLength: length of each dash
//   - gapLength: length of gap between dashes
//
// Return:
//   - updates img with the line drawn between two points
//
// DrawDashedLine draws a dashed line between two points
func DrawDashedLine(start, end Point, width float64, img *image.RGBA, color color.Color, dashLength, gapLength float64) {
	// Calculate total length and direction
	dx := end.X - start.X
	dy := end.Y - start.Y
	length := math.Sqrt(dx*dx + dy*dy)

	// Early return if points are too close
	if length < 1 {
		return
	}

	// Normalize direction
	dx /= length
	dy /= length

	// Calculate number of complete dash+gap segments that fit
	segmentLength := dashLength + gapLength
	numSegments := int(length / segmentLength)

	// Draw each dash segment
	currentPoint := start
	for _ = range numSegments {
		// Calculate dash start point
		dashStart := Point{
			X: currentPoint.X,
			Y: currentPoint.Y,
		}

		// Calculate dash end point
		dashEnd := Point{
			X: currentPoint.X + dx*dashLength,
			Y: currentPoint.Y + dy*dashLength,
		}

		// Draw the dash
		DrawLine(dashStart, dashEnd, width, img, color)

		// Move to start of next segment
		currentPoint = Point{
			X: currentPoint.X + dx*segmentLength,
			Y: currentPoint.Y + dy*segmentLength,
		}
	}

	// Draw final partial segment if there's space remaining
	remainingLength := length - float64(numSegments)*segmentLength
	if remainingLength > 0 {
		dashLength := math.Min(dashLength, remainingLength)
		finalDashEnd := Point{
			X: currentPoint.X + dx*dashLength,
			Y: currentPoint.Y + dy*dashLength,
		}

		DrawLine(currentPoint, finalDashEnd, width, img, color)
	}
}

// DrawCrossties draws railway crossties (perpendicular hash marks)
func DrawCrossties(start, end Point, width float64, img *image.RGBA, color color.Color, spacing float64) {
	// Calculate direction and length
	dx := end.X - start.X
	dy := end.Y - start.Y
	length := math.Sqrt(dx*dx + dy*dy)

	// Early return if points are too close
	if length < 1 {
		return
	}

	// Normalize direction
	dx /= length
	dy /= length

	// Calculate perpendicular direction for crossties
	perpX := -dy
	perpY := dx

	// Calculate number of crossties
	numCrossties := int(length / spacing)
	crosstieLength := width * 1.5 // Make crossties 1.5x the width of the line

	// First draw the main line
	DrawLine(start, end, width/2, img, color) // Draw thinner main line

	// Then draw the crossties
	for i := range numCrossties {
		// Calculate position along the line
		position := float64(i) * spacing
		if position > length {
			break
		}

		// Calculate center point of crosstie
		centerX := start.X + dx*position
		centerY := start.Y + dy*position

		// Calculate crosstie endpoints
		crosstieStart := Point{
			X: centerX + perpX*(crosstieLength/2),
			Y: centerY + perpY*(crosstieLength/2),
		}

		crosstieEnd := Point{
			X: centerX - perpX*(crosstieLength/2),
			Y: centerY - perpY*(crosstieLength/2),
		}

		// Draw the crosstie
		DrawLine(crosstieStart, crosstieEnd, width/2, img, color)
	}
}

// DistanceBetweenPoints calculates the distance between two points
func DistanceBetweenPoints(p1, p2 Point) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return math.Sqrt(dx*dx + dy*dy)
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

	for i := range points {
		if ((points[i].Y > y) != (points[j].Y > y)) &&
			(x < (points[j].X-points[i].X)*(y-points[i].Y)/(points[j].Y-points[i].Y)+points[i].X) {
			inside = !inside
		}
		j = i
	}

	return inside
}

// Make an api request to overpass-api, process the data into and overpassResponse data type
func MakeRoadApiRequest(apiUrl string, maps []int) (OverpassResponse, error) {
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
	log.Printf("Posting request to osm...\n")
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

	roadColor := color.RGBA{255, 235, 0, 180} // Yellow in RGBA

	// Connect all the points
	for r := range roads {
		road := roads[r]
		for p := range len(road) - 1 {
			p1 := road[p]
			p2 := road[p+1]
			DrawLine(p1, p2, widths[roadType], grayImage, roadColor)
		}
	}
}

// DrawRailways draws railway lines with railway-specific styling
func DrawRailways(ways []Element, nodes map[int64]Element, coords []int, grayImage *image.RGBA, railType string) {
	// Turn the lat/lon to an array of indexed points
	railways := ConvertCoordsToPngIndex(ways, nodes, Scale, coords[0], coords[1])

	// Define the color for railways
	railwayColor := color.RGBA{255, 235, 0, 180} // Yellow in RGBA

	// Connect all the points with hashed railway lines
	for r := range railways {
		railway := railways[r]
		for p := 0; p < len(railway)-1; p++ {
			p1 := railway[p]
			p2 := railway[p+1]

			// Use different styles based on railway type
			switch railType {
			case "rail":
				// Standard railway with crossties
				DrawCrossties(p1, p2, widths[railType], grayImage, railwayColor, 15)
			case "light_rail":
				// Light rail with more frequent crossties
				DrawCrossties(p1, p2, widths[railType], grayImage, railwayColor, 10)
			case "tram":
				// Tram with shorter dashes
				DrawDashedLine(p1, p2, widths[railType], grayImage, railwayColor, 6, 4)
			default:
				// Default drawing for unknown railway types
				DrawLine(p1, p2, widths[railType]/2, grayImage, railwayColor)
			}
		}
	}
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

func GenerateOsmPng(heightmaps []string, targetDir string, apiURL string) error {
	if heightmaps == nil || targetDir == "" {
		return fmt.Errorf("no maps or targetDir specified")
	}

	maps := make([][]int, 0, len(heightmaps))
	for _, heightmap := range heightmaps {
		// Convert the heightmap string to a coordinate pair
		coords, err := ParseHeightmapToCoords(heightmap)
		if err != nil {
			log.Printf("Warning: skipping invalid heightmap %s: %v", heightmap, err)
			continue
		}
		maps = append(maps, coords)
	}

	for i := range maps {
		osmData, err := MakeRoadApiRequest(apiURL, maps[i])
		if err != nil {
			return err
		}

		// Filter the data to keep only relevant highways
		var motorways, trunks, primaries, secondaries, tertiaries []Element
		var rails, lightRails, trams []Element
		var runways, airstrips, taxiways []Element
		nodes := make(map[int64]Element)

		for _, Element := range osmData.Elements {
			// Store nodes for easy access by ID
			if Element.Type == "node" {
				nodes[Element.ID] = Element
				continue
			}

			// Sort roads and railways by type
			if Element.Type == "way" {
				// Handle highways
				if highway, exists := Element.Tags["highway"]; exists {
					switch highway {
					case "motorway":
						motorways = append(motorways, Element)
					case "trunk":
						trunks = append(trunks, Element)
					case "primary":
						primaries = append(primaries, Element)
					case "secondary":
						secondaries = append(secondaries, Element)
					case "tertiary":
						tertiaries = append(tertiaries, Element)
					}
				}

				if aeroway, exists := Element.Tags["aeroway"]; exists {
					switch aeroway {
					case "runway":
						runways = append(runways, Element)
					case "airstrip":
						airstrips = append(airstrips, Element)
					case "taxiway":
						taxiways = append(taxiways, Element)
					}

				}

				// Handle railways
				if railway, exists := Element.Tags["railway"]; exists {
					switch railway {
					case "rail":
						rails = append(rails, Element)
					case "light_rail":
						lightRails = append(lightRails, Element)
					case "tram":
						trams = append(trams, Element)
					}
				}
			}
		}

		// Generate an all white PNG
		grayImage := image.NewRGBA(image.Rect(0, 0, Scale*ImgLength, Scale*ImgLength))
		for x := range ImgLength * int(Scale) {
			for y := range ImgLength * int(Scale) {
				grayImage.Set(x, y, color.Transparent)
			}
		}

		// Draw each set of roads
		DrawRoads(motorways, nodes, maps[i], grayImage, "motorway")
		DrawRoads(trunks, nodes, maps[i], grayImage, "trunk")
		DrawRoads(primaries, nodes, maps[i], grayImage, "primary")
		DrawRoads(secondaries, nodes, maps[i], grayImage, "secondary")
		DrawRoads(tertiaries, nodes, maps[i], grayImage, "tertiary")

		// Draw each set of railways
		DrawRailways(rails, nodes, maps[i], grayImage, "rail")
		DrawRailways(lightRails, nodes, maps[i], grayImage, "light_rail")
		DrawRailways(trams, nodes, maps[i], grayImage, "tram")

		// Draw 2D airport data
		DrawRoads(runways, nodes, maps[i], grayImage, "runway")
		DrawRoads(airstrips, nodes, maps[i], grayImage, "airstrip")
		DrawRoads(taxiways, nodes, maps[i], grayImage, "taxiway")

		bounds := CoordToString(maps[i][0], maps[i][1])
		pngFilepath := targetDir + bounds + ".osm.png"
		pngFile, err := os.Create(pngFilepath)
		fmt.Println("created png", pngFilepath)
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
