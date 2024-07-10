// Package heightmap provides downloading and processing of raw heightmap data
package heightmap

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
)

const (
	length = 3601
)

type Elevations [length][length]int16
type Pixels [length][length]uint16

// Heightmap formatted as "NASADEM_HGT_[NS]\d{2}[EW]\d{3}.hgt"
// TODO not sure how to push EarthData login
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

// Populate the pixels matrix by noramalizing the elevation matrix
// This will convert every elevation number from -32768 to 32767 to 0 - 65535
//
//	e.g. -12345 will become 20424 since it is 20424 from -32768
func (pixels *Pixels) populate(elevations Elevations) error {
	for i := 0; i < length; i++ {
		for j := 0; j < length; j++ {
			normalized := uint16(elevations[i][j]) + math.MaxInt16
			pixels[i][j] = normalized
		}
	}
	return nil
}

// Converts a heightmap at HGTFilepath to a png output at PNGFilepath
func HeightmapToPng(HGTFilepath, PNGFilepath string) error {
	// Create a binary slice from the heightmap
	data, err := heightmapToBytes(HGTFilepath)
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
	var pixels Pixels
	err = pixels.populate(elevations)
	if err != nil {
		return fmt.Errorf("failed to normalize matrix to pixels: %v", err)
	}

	// Fill the grayImage with the normalized array (Pixels)
	grayImage := image.NewGray16(image.Rect(0, 0, length, length))
	for y := 0; y < length; y++ {
		for x := 0; x < length; x++ {
			grayValue := pixels[x][y]
			grayImage.SetGray16(x, y, color.Gray16{Y: grayValue})
		}
	}

	// Write elevations to png
	pngFile, err := os.Create(PNGFilepath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %v", PNGFilepath, err)
	}
	defer pngFile.Close()

	if err := png.Encode(pngFile, grayImage); err != nil {
		return fmt.Errorf("failed to encode %s: %v", PNGFilepath, err)
	}

	return nil
}

// Downloads a file from the given url and saves it to filepath
func DownloadFile(filepath, url string) error {
	// Get the data from the url
	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to retrieve from url %s: %v", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", response.Status)
	}

	// Create the output file
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", filepath, err)
	}
	defer file.Close()

	// Write the data to the file
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return fmt.Errorf("failed to copy url data to file: %v", err)
	}

	return nil
}

// Unzips the zipFile to the targetDir
func Unzip(zipFile, targetDir string) error {
	// Open zip file
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return fmt.Errorf("failed to create zipfile reader %s: %v", zipFile, err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		readerCloser, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open zipfile: %v", err)
		}
		defer readerCloser.Close()

		extractPath := filepath.Join(targetDir, file.Name)

		// If the file is a directory, make the directory
		if file.FileInfo().IsDir() {
			err := os.MkdirAll(extractPath, file.Mode())
			if err != nil {
				return fmt.Errorf("failed to create directory: %v", err)
			}
		} else {
			// This is a file
			if err = os.MkdirAll(filepath.Dir(extractPath), os.ModePerm); err != nil {
				return fmt.Errorf("failed to create file at %s: %v", extractPath, err)
			}

			outFile, err := os.OpenFile(extractPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode().Perm())
			if err != nil {
				return fmt.Errorf("failed to open file at %s: %v", extractPath, err)
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, readerCloser)
			if err != nil {
				return fmt.Errorf("failed to write contents to output file: %v", err)
			}
		}
	}

	return nil
}

// Downloads and Processes heightmap from the given coords string
// Will download the heightmap to assets/heightmaps
//
//	coords string: [NS]\d{2}[EW]\d{3}
func NewHeightmap(coords string) error {
	// Download File to assets/heightmaps
	// Unzip File to assets/heightmaps
	// Convert Heightmap to png in assets/heightmaps
	// Remove Zip file
	// Remove unziped directory
	return nil
}

