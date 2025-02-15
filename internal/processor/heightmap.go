// Package processor for processing downloaded data into our useable formats
package processor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
)

// This file should be used for processing heightmaps (into image.Gray16 PNGs)

const (
	length         = 3601
	pngCompression = png.DefaultCompression
)

type Elevations [length][length]int16
type ElevationPixels [length][length]uint16

// Populate the elevations matrix with a data byte array
func (elevations *Elevations) populate(data []byte) error {
	reader := bytes.NewReader(data)
	for i := 0; i < length; i++ {
		for j := 0; j < length; j++ {
			var value int16
			err := binary.Read(reader, binary.BigEndian, &value)
			if err != nil {
				return fmt.Errorf("error reading int16: %v", err)
			}
			elevations[i][j] = value
		}
	}
	return nil
}

// Populate the pixels matrix by normalizing the elevation matrix
// This will convert every elevation number from -32768 to 32767 to 0 - 65535
//
//	e.g. -12345 will become 20424 since it is 20424 from -32768
func (pixels *ElevationPixels) populate(elevations Elevations) error {
	var minInt16 uint16 = uint16(math.Abs(math.MinInt16))
	for i := 0; i < length; i++ {
		for j := 0; j < length; j++ {
			normalized := uint16(elevations[i][j]) + minInt16
			pixels[i][j] = normalized
		}
	}
	return nil
}

// Heightmap formatted as "NASADEM_HGT_[NS]\d{2}[EW]\d{3}.hgt"
func heightmapToBytes(HGTFilepath string) ([]byte, error) {
	// Open the heightmap file
	hgtFile, err := os.Open(HGTFilepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %v", HGTFilepath, err)
	}
	defer hgtFile.Close()

	// Check the heightmap file and size
	hgtFileInfo, err := hgtFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("error getting file info: %v", err)
	}
	fileSize := hgtFileInfo.Size()

	// expected file size is 3601 * 3601 * 2 (size of int16 is 2 bytes)
	var expectedFileSize int64 = length * length * 2
	if fileSize != expectedFileSize {
		return nil, fmt.Errorf("wrong file size, expected %d, received %d", expectedFileSize, fileSize)
	}

	// Create binary array and read it into data
	data := make([]byte, fileSize)
	_, err = hgtFile.Read(data)
	if err != nil {
		return nil, fmt.Errorf("error reading data from %s: %v", HGTFilepath, err)
	}

	return data, nil
}

// Encode the PNG with higher compression rate
func encodePNG(w io.Writer, img image.Image) error {
	encoder := png.Encoder{
		CompressionLevel: pngCompression,
	}
	return encoder.Encode(w, img)
}

// Converts a heightmap at HGTFilepath to a png output at PNGFilepath
func HeightmapToPng(heightmapFilepath, pngFilepath string) error {
	fmt.Printf("Creating %s\n", pngFilepath)
	// Create a binary slice from the heightmap
	data, err := heightmapToBytes(heightmapFilepath)
	if err != nil {
		return fmt.Errorf("failed to convert heightmap to byte array: %v", err)
	}

	// Pull the elevations from the heightmap data
	var elevations Elevations
	err = elevations.populate(data)
	if err != nil {
		return fmt.Errorf("failed to populate elevation matrix: %v", err)
	}

	// Normalize the elevations to uint16 (no negatives in png)
	var pixels ElevationPixels
	err = pixels.populate(elevations)
	if err != nil {
		return fmt.Errorf("failed to normalize matrix to pixels: %v", err)
	}

	// Fill the grayImage with the normalized array (Pixels)
	grayImage := image.NewGray16(image.Rect(0, 0, length, length))
	for y := 0; y < length; y++ {
		for x := 0; x < length; x++ {
			grayValue := pixels[y][x]
			grayImage.SetGray16(x, y, color.Gray16{Y: grayValue})
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
