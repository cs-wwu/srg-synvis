package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/term"
)

const (
	length           = 3601
	nasaBaseURL      = "https://e4ftl01.cr.usgs.gov/MEASURES/NASADEM_HGT.001/2000.02.11/NASADEM_HGT_"
	heightmapPattern = "[ns]\\d{2}[ew]\\d{3}"
	pngCompression   = png.DefaultCompression
)

type Credentials struct {
	Username string
	Password string
}

type Elevations [length][length]int16
type Pixels [length][length]uint16

func promptCredentials() (Credentials, error) {
	var creds Credentials
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Enter your Earthdata Credentials.")
	fmt.Print("Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return creds, fmt.Errorf("failed to read username: %v", err)
	}
	username = strings.TrimSpace(username)

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(0)
	if err != nil {
		return creds, fmt.Errorf("failed to read password: %v", err)
	}
	password := string(passwordBytes)
	fmt.Println() // Add a newline after the password input

	return Credentials{
		Username: username,
		Password: password,
	}, nil
}

func createClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookiejar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects automatically
		},
	}, nil
}

// Returns the *http.response and error
// Must close response body
func getResponse(client *http.Client, url string) (*http.Response, error) {
	var resp *http.Response

	resp, err := client.Get(url)
	if err != nil {
		return resp, fmt.Errorf("failed to send request: %v", err)
	}

	return resp, nil
}

// Returns the response headers last location, error
// Attempts to authenticate the user with the last given url
func attemptAuthenticate(client *http.Client, url string, creds Credentials) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(creds.Username, creds.Password)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	return resp.Header.Get("Location"), nil
}

// Downloads the responses body to the specified filepath
func downloadResponse(response *http.Response, filepath string) error {
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

// Converts a heightmap in the form [ns]\d{2}[ew]\d{3} into the equivalent
// NASA url to fetch the heightmap
// Checks that the heightmap is valid
func heightmapToURL(heightmap string) (string, error) {
	matched, err := regexp.MatchString(heightmapPattern, heightmap)
	if err != nil {
		return "", fmt.Errorf("failed to match string %s: %v", heightmap, err)
	}
	if !matched {
		return "", fmt.Errorf("bad heightmap format: %s", heightmap)
	}
	return nasaBaseURL + heightmap + ".zip", nil
}

// Authenticates the client by attempting to authenticate multiple times
func authenticate(client *http.Client, url, location string) error {
	creds, err := promptCredentials()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %v", err)
	}

	// Attempt authentication
	for i := 0; i < 3; i++ {
		location, err = attemptAuthenticate(client, location, creds)
		if err != nil {
			return fmt.Errorf("failed to authenticate at \"%v\": %v", location, err)
		}
		if location == url {
			return nil // We redirected back to the original url, success
		}
	}

	return fmt.Errorf("failed to authenticate %v, redirected to %v", url, location)
}

// attempts to download the given url
// on the user to defer response.Body.Close()
func attemptDownload(client *http.Client, filepath, url string) error {
	resp, err := getResponse(client, url)
	if err != nil {
		return fmt.Errorf("failed to get response: %v", err)
	}
	defer resp.Body.Close()

	// Download if good status code
	if resp.StatusCode == http.StatusOK {
		err = downloadResponse(resp, filepath)
		if err != nil {
			return fmt.Errorf("failed to download file: %v", err)
		} else {
			return nil
		}
	}

	// Follow redirects to authenticate
	location := resp.Header.Get("Location")
	authenticate(client, url, location)

	// Retries the download after authenticating
	resp, err = getResponse(client, url)
	if err != nil {
		return fmt.Errorf("failed to get response: %v", err)
	}
	defer resp.Body.Close()

	// Download if good status code
	if resp.StatusCode == http.StatusOK {
		err = downloadResponse(resp, filepath)
		if err != nil {
			return fmt.Errorf("failed to download file: %v", err)
		}
	}

	return nil
}

// Downloads the selection of heightmaps to directoryPath
func downloadHeightmaps(heightmaps []string, directoryPath string) error {
	// Create a client
	client, err := createClient()
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}

	for _, heightmap := range heightmaps {
		fmt.Printf("Downloading: %s\n", heightmap)
		heightmapURL, err := heightmapToURL(heightmap)
		if err != nil {
			return fmt.Errorf("failed to convert heightmap to url: %v", err)
		}
		filepath := directoryPath + heightmap + ".zip"
		err = attemptDownload(client, filepath, heightmapURL)
		if err != nil {
			return fmt.Errorf("failed to download heightmap: %v", err)
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
func (pixels *Pixels) populate(elevations Elevations) error {
	var minInt16 uint16 = uint16(math.Abs(math.MinInt16))
	for i := 0; i < length; i++ {
		for j := 0; j < length; j++ {
			normalized := uint16(elevations[i][j]) + minInt16
			pixels[i][j] = normalized
		}
	}
	return nil
}

// Encode the PNG with higher compression rate
func encodePNG(w io.Writer, img image.Image) error {
	encoder := png.Encoder{
		CompressionLevel: pngCompression,
	}
	return encoder.Encode(w, img)
}

// Converts a heightmap at HGTFilepath to a png output at PNGFilepath
func heightmapToPng(heightmapFilepath, pngFilepath string) error {
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
	var pixels Pixels
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

// Unzips the zipFile to the targetDir
func unzip(zipFile, targetDir string) error {
	fmt.Printf("Unzipping %s\n", zipFile)
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

// Processes an array of heightmaps into pngs into the targetDir
func ProcessHeightmaps(heightmaps []string, targetDir string) error {
	err := downloadHeightmaps(heightmaps, targetDir)
	if err != nil {
		return fmt.Errorf("failed to download heightmaps: %v", err)
	}

	// Unzip each heightmap and remove the old zipped folders
	for _, heightmap := range heightmaps {
		zipFilePath := targetDir + heightmap + ".zip"
		outputDir := targetDir + heightmap + "/"
		err = unzip(zipFilePath, outputDir)
		if err != nil {
			return fmt.Errorf("failed to unzip %s: %v", heightmap, err)
		}

		os.Remove(zipFilePath)

		heightmapFile := outputDir + heightmap + ".hgt"
		pngFile := targetDir + heightmap + ".png"
		err = heightmapToPng(heightmapFile, pngFile)
		if err != nil {
			return fmt.Errorf("failed to convert %s to png: %v", heightmap, err)
		}

		os.RemoveAll(outputDir)
	}

	return nil
}

// Example Usage
func main() {
	targetDir := "heightmaps/"
	heightmaps := []string{"n48w123"}
	err := ProcessHeightmaps(heightmaps, targetDir)
	if err != nil {
		fmt.Printf("failed to process heightmaps: %v", err)
		os.Exit(1)
	}
}
