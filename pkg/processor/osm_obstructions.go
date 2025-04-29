package processor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Make an api request to overpass-api, process the data into and overpassResponse data type
func MakeAirApiRequest(apiUrl string, maps []int) (OverpassResponse, error) {
	lat := maps[0]
	lon := maps[1]
	bounds := "(" + strconv.Itoa(lat) + "," + strconv.Itoa(lon) + "," + strconv.Itoa(lat+1) + "," + strconv.Itoa(lon+1) + ");"
	query := `
	[out:json];
	(
		way["aeroway"="runway"]` + bounds +
		`way["aeroway"="terminal"]` + bounds +
		`way["aeroway"="aerodrome"]` + bounds +
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
func StoreAirData(ways []Element, nodes map[int64]Element, coords []int, airType string) [][]Point {

	airBuf := [][]Point{}

	node := nodes[ways[0].Nodes[0]]
	log.Printf("%f", node.Lat)

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
		var runway, terminal, aerodrome []Element
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
				if airport, exists := Element.Tags["aeroway"]; exists {
					switch airport {
					case "runway":
						runway = append(runway, Element)
					case "terminal":
						terminal = append(terminal, Element)
					case "aerodrome":
						aerodrome = append(aerodrome, Element)
					}

				}
			}
		}

		// Store each airport data type into a buffer
		runwayBuf := StoreAirData(runway, nodes, maps[i], "runway")
		termBuf := StoreAirData(terminal, nodes, maps[i], "terminal")
		areoBuf := StoreAirData(aerodrome, nodes, maps[i], "aerodrome")

		fmt.Println(runwayBuf, termBuf, areoBuf)
	}

	return nil
}
