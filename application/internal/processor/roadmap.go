package processor

import (
	"fmt"
	"os"
)

// This file should be used for processing OSM data sets (into image.BW PNGS)

type elements struct {
	elements []element
}

type element struct {
	Type      string `json:"type"`
	id        int
	bounds    bound
	nodes     []int
	geomertry []coordinate
	tags      []string
}

type coordinate struct {
	lat float32
	lon float32
}

type bound struct {
	minlat float32
	minlon float32
	maxlat float32
	maxlon float32
}

// Turns a road into a collection of points on the image
func coordToPixels()

func osmToPng(osmFilepath, pngFilepath string) error {
	fmt.Printf("Creating %s \n", pngFilepath)
	osmFile, err := os.Open(osmFilepath)
	if err != nil {
		fmt.Println(err)
	}
	defer osmFile.Close()

	return nil
}
