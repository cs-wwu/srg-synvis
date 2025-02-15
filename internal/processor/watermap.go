package processor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
)

// This file should be used for processing watermaps (into image.Gray PNGs)

type Water [length][length]uint8

// Fill the water array
func (w *Water) populate(data []byte) error {
	reader := bytes.NewReader(data)
	for i := 0; i < length; i++ {
		for j := 0; j < length; j++ {
			var value uint8
			err := binary.Read(reader, binary.BigEndian, &value)
			if err != nil {
				return fmt.Errorf("error reading uint8: %v", err)
			}
			w[i][j] = value
		}
	}
	return nil
}

// Turns a water (.swb) file to a byte array
func watermapToBytes(waterFilepath string) ([]byte, error) {
	// Open the water file
	hgtFile, err := os.Open(waterFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %v", waterFilepath, err)
	}
	defer hgtFile.Close()

	// Check the watermap file and size
	hgtFileInfo, err := hgtFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("error getting file info: %v", err)
	}
	fileSize := hgtFileInfo.Size()

	// expected file size is 3601 * 3601 (size of uint8 is 1 byte)
	var expectedFileSize int64 = length * length
	if fileSize != expectedFileSize {
		return nil, fmt.Errorf("wrong file size, expected %d, received %d", expectedFileSize, fileSize)
	}

	// Create binary array and read it into data
	data := make([]byte, fileSize)
	_, err = hgtFile.Read(data)
	if err != nil {
		return nil, fmt.Errorf("error reading data from %s: %v", waterFilepath, err)
	}

	return data, nil
}

// Function to turn a water (.swb) file to a png
func WatermapToPng(waterFilepath, pngFilepath string) error {
	fmt.Printf("Creating %s\n", pngFilepath)
	// Create a binary slice from the watermap
	data, err := watermapToBytes(waterFilepath)
	if err != nil {
		return fmt.Errorf("failed to convert water to byte array: %v", err)
	}

	// Pull the water map from the water data
	var water Water
	err = water.populate(data)
	if err != nil {
		return fmt.Errorf("failed to populate water matrix: %v", err)
	}

	// Fill the grayImage with the normalized array (Pixels)
	grayImage := image.NewGray(image.Rect(0, 0, length, length))
	for y := 0; y < length; y++ {
		for x := 0; x < length; x++ {
			grayValue := water[y][x]
			grayImage.SetGray(x, y, color.Gray{Y: grayValue})
		}
	}

	// Write elevations to png
	pngFile, err := os.Create(pngFilepath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %v", pngFilepath, err)
	}
	defer pngFile.Close()

	if err := encodePNG(pngFile, grayImage); err != nil {
		return fmt.Errorf("failed to encode %s: %v", pngFilepath, err)
	}

	return nil
}
