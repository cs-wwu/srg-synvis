package airspace

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cs-wwu/srg-synvis/internal/rendering"
)

var outDir = "assets/chunks/airspace/data"

func ChunkGeoJson(inputFile string) {
	fileData, err := rendering.FileToBytesChecked(inputFile)
	var geojson rendering.GeoJSON
	if err != nil {
		log.Printf("ChunkGeoJson: error opening GeoJson file %s: %v", inputFile, err)
	}
	err = json.Unmarshal(fileData, &geojson)
	if err != nil {
		log.Printf("ChunkGeoJson: error unmarshaling GeoJson file %s: %v", inputFile, err)
	}

	features := geojson.Features

	for _, feature := range features {
		avgLat, avgLon := determineChunk(feature.Geometry)

		chunkFileName := fmt.Sprintf("n%dw%d.geojson", avgLat, avgLon)
		chunkFilePath := filepath.Join(outDir, chunkFileName)
		err := os.MkdirAll(outDir, 0755)
		if err != nil {
			log.Printf("error creating output")
			return
		}

		if _, err := os.Stat(chunkFilePath); os.IsNotExist(err) {
			fmt.Println("File does not exist")
		} else {
			fmt.Println("File exists")
		}
	}
}

func determineChunk(coords rendering.Geometry) (lat, lon int) {
	latTotal := 0
	lonTotal := 0

	outerRing := coords.Coordinates[0]
	n := len(outerRing) - 1

	for i := range n {
		latTotal += int(outerRing[i][0])
		lonTotal += int(outerRing[i][1])
	}

	return latTotal / n, lonTotal / n
}
