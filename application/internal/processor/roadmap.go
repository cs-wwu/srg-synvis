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

// Creates an array of roads: [road#][point#][x, y]
func convertCoordsToPngIndex(highways []Element, nodes map[int64]Element, scale int) [][][]int {
	roads := [][][]int{}
	c := float64(scale)
	for i := range highways {
		tempRoad := [][]int{}
		for j := range highways[i].Nodes {
			node := nodes[highways[i].Nodes[j]]
			// Transforms the lat/lon into x/y coordinates
			y := int(math.Round((1 - (math.Abs(node.Lat) - 48)) * (c*length - c)))
			x := int(math.Round((1 - (math.Abs(node.Lon) - 122)) * (c*length - c)))
			index := []int{x, y}
			tempRoad = append(tempRoad, index)
		}
		roads = append(roads, tempRoad)
	}
	return roads
}

// Draws a rectangle between each set of points, if the diagonal distance 
// is too far, it cuts it into subrectangles
func drawLine(p1 []int, p2 []int, img *image.Gray) {
	if (math.Abs(float64(p2[0]-p1[0])) > 10 && math.Abs(float64(p2[1]-p1[1])) > 10) {
		p3 := []int{(p1[0] + (p2[0] - p1[0])/2), (p1[1] + (p2[1] - p1[1])/2)}
		drawLine(p1, p3, img)
		drawLine(p3, p2, img)
	} else {
		road := image.Rect(p1[0], p1[1], p2[0], p2[1])
		for x := road.Min.X-10; x < road.Max.X+10; x++ {
			for y := road.Min.Y-10; y < road.Max.Y+10; y++ {
				img.SetGray(x, y, color.Gray{Y: math.MaxUint8})
			}
		}
	}
}

func Itoa(n int) string {
    if n == 0 {
        return "0"
    }

    sign := ""
    if n < 0 {
        sign = "-"
        n = -n
    }

    q := ""
    for n > 0 {
        digits := n % 10
        q = string(rune('0'+digits)) + q
        n /= 10
    }

    return sign + q
}

func GenerateOsmPng(maps [][]int, targetDir string) error {
	for i := range maps {
		bounds  := "(" + Itoa(maps[i][0]) + "," + Itoa(maps[i][1]) + "," + Itoa(maps[i][0] + 1) + "," + Itoa(maps[i][1] + 1) + ");"
		// Make a request to Overpass API
		query := `
		[out:json];
		(
			way["highway"="motorway"]`+bounds+
			`way["highway"="trunk"]`+bounds+
			`way["highway"="primary"]`+bounds+
			`way["highway"="secondary"]`+bounds+
			`way["highway"="tertiary"]`+bounds+
		`);
		(._;>;);
		out body;
		`
		resp, err := http.Post("http://overpass-api.de/api/interpreter", "application/x-www-form-urlencoded", strings.NewReader("data="+query))
		if err != nil {
			return fmt.Errorf("error: %e", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response: %e", err)
		}

		// Parse JSON response
		var osmData OverpassResponse
		err = json.Unmarshal(body, &osmData)
		if err != nil {
			return fmt.Errorf("error unmarshaling JSON: %e", err)
		}

		// Filter the data to keep only relevant highways
		var highways []Element
		nodes := make(map[int64]Element)

		for _, element := range osmData.Elements {
			if element.Type == "way" && element.Tags["highway"] != "" {
				// Keep only highway elements (ways)
				highways = append(highways, element)
			} else if element.Type == "node" {
				// Store nodes in a map for easy access by ID
				nodes[element.ID] = element
			}
		}

		// Turn the lat/lon to an arry of 3600 indexed float32
		c := 2
		roads := convertCoordsToPngIndex(highways, nodes, c)

		grayImage := image.NewGray(image.Rect(0, 0, c*length, c*length))
		for r := range roads {
			road := roads[r]
			for p := 0; p < len(road)-1; p++ {
				p1 := road[p]
				p2 := road[p+1]
				drawLine(p1, p2, grayImage)
			}
		}

		pngFilepath := targetDir + bounds + ".osm.png"
		pngFile, err := os.Create(pngFilepath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", pngFilepath, err)
		}
		defer pngFile.Close()

		if err := encodePNG(pngFile, grayImage); err != nil {
			return fmt.Errorf("failed to encode %s: %v", pngFilepath, err)
		}
	}

	return nil
}
