package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cs-wwu/srg-synvis/api"
	"github.com/cs-wwu/srg-synvis/internal/processor"
)

// This file should be used for downloading the necessary heightmaps, etc.
// This should eventually be a gui with region selection. For now it should
// at least download a portion of Washington State.

// This should only be called once with the array of heightmaps, due to
// creating a client and signing in once
// Processes an array of heightmaps into pngs into the targetDir
// See bottom of heightmap.go for example usage
func ProcessHeightmaps(heightmaps []string, targetDir string) error {
	if !strings.HasSuffix(targetDir, "/") { // append a "/" if necessary
		targetDir += "/"
	}

	new_heightmaps := []string{}
	for _, heightmap := range heightmaps {
		// if "MAPS_PATH/heightmap.hgt.png" and "MAPS_PATH/heightmap.swb.png" already exists,
		// then don't add it to the new heightmaps list
		// otherwise, add it
		hgtPngPath := targetDir + heightmap + ".hgt.png"
		swbPngPath := targetDir + heightmap + ".swb.png"
		if _, err := os.Stat(hgtPngPath); os.IsNotExist(err) {
			new_heightmaps = append(new_heightmaps, heightmap)
		} else if _, err := os.Stat(swbPngPath); os.IsNotExist(err) {
			new_heightmaps = append(new_heightmaps, heightmap)
		}
	}
	heightmaps = new_heightmaps

	// This should only be called once due to client cookies and sign-in
	err := api.DownloadHeightmaps(heightmaps, targetDir)
	if err != nil {
		return fmt.Errorf("failed to download heightmaps: %v", err)
	}

	// Unzip each heightmap and remove the old zipped folders
	for _, heightmap := range heightmaps {
		zipFilePath := targetDir + heightmap + ".zip"
		outputDir := targetDir + heightmap + "/"
		err := api.Unzip(zipFilePath, outputDir)
		if err != nil {
			return fmt.Errorf("failed to unzip %s: %v", heightmap, err)
		}

		os.Remove(zipFilePath) // remove zip file

		startTime := time.Now()
		waterFile := outputDir + heightmap + ".swb"
		waterPng := targetDir + heightmap + ".swb" + ".png"
		err = processor.WatermapToPng(waterFile, waterPng)
		if err != nil {
			return fmt.Errorf("failed to convert water to png: %w", err)
		}
		fmt.Printf("png conversion took %v\n", time.Since(startTime))

		heightmapFile := outputDir + heightmap + ".hgt"
		pngFile := targetDir + heightmap + ".hgt" + ".png"
		err = processor.HeightmapToPng(heightmapFile, pngFile)
		if err != nil {
			return fmt.Errorf("failed to convert %s to png: %v", heightmap, err)
		}

		os.RemoveAll(outputDir) // Remove leftovers
	}

	return nil
}

// Current Example Usage
func main() {
	targetDir := "assets/maps/"
	heightmaps := []string{"n48w123", "n48w122", "n47w123", "n47w122"}
	err := ProcessHeightmaps(heightmaps, targetDir)
	if err != nil {
		fmt.Printf("failed to process heightmaps: %v\n", err)
		os.Exit(1)
	}
}
