package processor

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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

func RunQuery(outputPath string) {
	// Make a request to Overpass API
	query := `
	[out:json];
	way["highway"](48,-123,49,-122);
	(._;>;);
	out body;
	`
	resp, err := http.Post("http://overpass-api.de/api/interpreter", "application/x-www-form-urlencoded", strings.NewReader("data="+query))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return
	}

	// Parse JSON response
	var osmData OverpassResponse
	err = json.Unmarshal(body, &osmData)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return
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

	jsonData, err := json.Marshal(osmData)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	jsonfile, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	_, err = jsonfile.Write(jsonData)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

}
