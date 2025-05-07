package processor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type OSMPoint struct {
	Lat    float64
	Lon    float64
	Height float64
}

// Make an api request to overpass-api, process the data into and overpassResponse data type
func MakeAirApiRequest(apiUrl string, maps []int) (OverpassResponse, error) {
	lat := maps[0]
	lon := maps[1]
	bounds := "(" + strconv.Itoa(lat) + "," + strconv.Itoa(lon) + "," + strconv.Itoa(lat+1) + "," + strconv.Itoa(lon+1) + ");"
	query := `
	[out:json];
	(
		way["aeroway"="hangar"]` + bounds +
		`way["aeroway"="terminal"]` + bounds +
		`way["aeroway"="tower"]` + bounds +
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

// StoreAirData stores aeroway data into an array and returns the array
func StoreAirData(ways []Element, nodes map[int64]Element, coords []int, airType string) [][]OSMPoint {

	airBuf := [][]OSMPoint{}

	for _, way := range ways {
		wayPoints := []OSMPoint{}
		for _, nodeID := range way.Nodes {
			if node, exists := nodes[nodeID]; exists {
				point := OSMPoint{
					Lat:    node.Lat,
					Lon:    node.Lon,
					Height: 0,
				}

				if height, exists := node.Tags["ele"]; exists {
					if ele, err := strconv.ParseFloat(height, 64); err == nil {
						point.Height = ele
					}

					wayPoints = append(wayPoints, point)
				}
			}
		}
		if len(wayPoints) > 0 {
			airBuf = append(airBuf, wayPoints)
		}
	}

	return airBuf
}

// GenerateAirData makes an osm api request for a coordinate squares aeroway data and stores the results
func GenerateAirData(maps [][]int, apiURL string) error {
	if maps == nil {
		return fmt.Errorf("no maps or targetDir specified")
	}

	for i := range maps {

		osmData, err := MakeAirApiRequest(apiURL, maps[i])
		if err != nil {
			return err
		}

		// Filter the data to keep only relevant airport information
		var hangars, terminals, towers []Element
		nodes := make(map[int64]Element)

		for _, Element := range osmData.Elements {
			// Store nodes for easy access by ID
			if Element.Type == "node" {
				nodes[Element.ID] = Element
				continue
			}

			// Sort aeroways by type
			// TODO: area instead of way?
			if Element.Type == "way" {
				// Handle highways
				if airport, exists := Element.Tags["aeroway"]; exists {
					switch airport {
					case "hangar":
						hangars = append(hangars, Element)
					case "terminal":
						terminals = append(terminals, Element)
					case "tower":
						towers = append(towers, Element)
					}
				}
			}
		}

		// Store each airport data type into a buffer
		hangarBuf := StoreAirData(hangars, nodes, maps[i], "hangar")
		termBuf := StoreAirData(terminals, nodes, maps[i], "terminal")
		towerBuf := StoreAirData(towers, nodes, maps[i], "tower")

		fmt.Println(hangarBuf, termBuf, towerBuf)
	}

	return nil
}
