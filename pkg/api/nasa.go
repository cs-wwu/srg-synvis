// Package api for accessing the data via our necessary APIs
package api

// This file is accessing and using the NASA EarthData API for downloading data
// This may also be used to download satellite imagery from NASA

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
)

const (
	nasaBaseURL      = "https://data.lpdaac.earthdatacloud.nasa.gov/lp-prod-protected/NASADEM_HGT.001/"
	filenameBase     = "NASADEM_HGT_"
	heightmapPattern = "[ns]\\d{2}[ew]\\d{3}"
)

type Credentials struct {
	Token string
}

// Returns the users credentials after prompting and storing them securly
func loadCredentials() (Credentials, error) {
	token := os.Getenv("EARTHDATA_TOKEN")
	if token == "" {
		return Credentials{}, fmt.Errorf("EARTHDATA_TOKEN environment variable not set")
	}

	return Credentials{
		Token: token,
	}, nil
}

// Creates the http client and initializes the cookie jar
func createClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookiejar: %v", err)
	}

	return &http.Client{
		Jar: jar,
	}, nil
}

func getResponse(client *http.Client, url string, creds Credentials) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+creds.Token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	return resp, nil
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
	// The full URL should look like this:
	// https://data.lpdaac.earthdatacloud.nasa.gov/lp-prod-protected/NASADEM_HGT.001/NASADEM_HGT_n48w123/NASADEM_HGT_n48w123.zip
	filename := filenameBase + heightmap
	return nasaBaseURL + filename + "/" + filename + ".zip", nil
}

// attempts to download the given url
// on the user to defer response.Body.Close()
func attemptDownload(client *http.Client, creds Credentials, filepath, url string) error {
	resp, err := getResponse(client, url, creds)
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
	} else {
		return fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}
}

// Downloads the selection of heightmaps to directoryPath
func DownloadHeightmaps(heightmaps []string, directoryPath string) error {
	// Create a client
	client, err := createClient()
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		os.Exit(1)
	}

	creds, err := loadCredentials()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %v", err)
	}

	for _, heightmap := range heightmaps {
		fmt.Printf("Downloading: %s\n", heightmap)
		heightmapURL, err := heightmapToURL(heightmap)
		if err != nil {
			return fmt.Errorf("failed to convert heightmap to url: %v", err)
		}

		filepath := directoryPath + heightmap + ".zip"
		err = attemptDownload(client, creds, filepath, heightmapURL)
		if err != nil {
			return fmt.Errorf("failed to download heightmap: %v", err)
		}
	}

	return nil
}

// Unzips the zipFile to the targetDir
func Unzip(zipFile, targetDir string) error {
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
