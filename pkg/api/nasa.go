// Package api for accessing the data via our necessary APIs
package api

// This file is accessing and using the NASA EarthData API for downloading data
// This may also be used to download satellite imagery from NASA

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/term"
)

const (
	nasaBaseURL      = "https://e4ftl01.cr.usgs.gov/MEASURES/NASADEM_HGT.001/2000.02.11/NASADEM_HGT_"
	heightmapPattern = "[ns]\\d{2}[ew]\\d{3}"
)

type Credentials struct {
	Username string
	Password string
}

// Returns the users credentials after prompting and storing them securly
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

// Creates the http client and initializes the cookie jar
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
func authenticate(client *http.Client, url, location string, creds Credentials) error {
	//creds, err := promptCredentials()
	// if err != nil {
	// 	return fmt.Errorf("failed to get credentials: %v", err)
	// }
	var err error

	// Attempt authentication
	for range 3 {
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
func attemptDownload(client *http.Client, filepath, url string, creds Credentials) error {
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
	authenticate(client, url, location, creds)

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
func DownloadHeightmaps(heightmaps []string, directoryPath, username, password string) error {
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
		creds := Credentials{
			Username: username,
			Password: password,
		}
		err = attemptDownload(client, filepath, heightmapURL, creds)
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
