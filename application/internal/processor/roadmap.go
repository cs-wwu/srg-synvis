package processor

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
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

// Creates an array of roads: [road#][x][y]
func convertCoordsToPngIndex(highways []Element, nodes map[int64]Element) [][][]int {
	roads := [][][]int{}
	for i := range highways {
		tempRoad := [][]int{}
		for j := range highways[i].Nodes {
			node := nodes[highways[i].Nodes[j]]
			x := int(math.Round((math.Abs(node.Lat)-48) * (2*length - 2)))
			y := int(math.Round((1-(math.Abs(node.Lon)-122)) * (2*length - 2)))
			index := []int{x, y}
			tempRoad = append(tempRoad, index)
		}
		roads = append(roads, tempRoad)
	}
	return roads
}

func GenerateOsmPng(outputPath string) error {
	// Make a request to Overpass API
	query := `
	[out:json];
	way["highway"](48,-123,49,-122);
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
	roads := convertCoordsToPngIndex(highways, nodes)

	for r := range roads {
		fmt.Println(roads[r])
	}

	return nil
}
