package processor

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pb "github.com/cs-wwu/srg-synvis/assets/chunks/obstructions"
	"golang.org/x/net/html"
	"google.golang.org/protobuf/proto"
)

// Constants for column offsets in DOF.DATA
const (
	headerStart = 18
	headerEnd   = 26

	latDegStart = 35
	latDegEnd   = 37
	latMinStart = 38
	latMinEnd   = 40
	latSecStart = 41
	latSecEnd   = 46

	lonDegStart = 48
	lonDegEnd   = 51
	lonMinStart = 52
	lonMinEnd   = 54
	lonSecStart = 55
	lonSecEnd   = 60

	obsStart = 62
	obsEnd   = 81
	qtyStart = 81
	qtyEnd   = 82
	hgtStart = 84
	hgtEnd   = 89
	eleStart = 90
	eleEnd   = 95
)

type FAAPoint struct {
	Lat      float64
	Lon      float64
	Height   float64
	Type     string
	Quantity float64
}

type ObsSquare struct {
	LatDeg  int
	LonDeg  int
	Records []FAAPoint
}

// Key generation for coordinate square map
func generateSquareKey(lat, lon float64) string {
	latDeg := int(math.Floor(lat))
	lonDeg := int(math.Floor(lon))
	return fmt.Sprintf("%d,%d", latDeg, lonDeg)
}

func parseDOFFile(filePath string, squareBuffers map[string]*ObsSquare) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip header
		if lineNum < 5 {
			continue
		}

		latDeg := strings.TrimSpace(line[latDegStart:latDegEnd])
		latMin := strings.TrimSpace(line[latMinStart:latMinEnd])
		latSec := strings.TrimSpace(line[latSecStart:latSecEnd])
		latHem := strings.TrimSpace(line[latSecEnd : latSecEnd+1])

		lonDeg := strings.TrimSpace(line[lonDegStart:lonDegEnd])
		lonMin := strings.TrimSpace(line[lonMinStart:lonMinEnd])
		lonSec := strings.TrimSpace(line[lonSecStart:lonSecEnd])
		lonHem := strings.TrimSpace(line[lonSecEnd : lonSecEnd+1])

		obsType := strings.TrimSpace(line[obsStart:obsEnd])
		quantity := strings.TrimSpace(line[qtyStart:qtyEnd])

		heightStr := strings.TrimSpace(line[hgtStart:hgtEnd])
		eleStr := strings.TrimSpace(line[eleStart:eleEnd])

		latDegVal, err1 := strconv.Atoi(latDeg)
		latMinVal, err2 := strconv.Atoi(latMin)
		latSecVal, err3 := strconv.ParseFloat(latSec, 64)
		lonDegVal, err4 := strconv.Atoi(lonDeg)
		lonMinVal, err5 := strconv.Atoi(lonMin)
		lonSecVal, err6 := strconv.ParseFloat(lonSec, 64)
		quantityVal, err7 := strconv.Atoi(quantity)

		elevation, err8 := strconv.ParseFloat(eleStr, 64)
		height, err9 := strconv.ParseFloat(heightStr, 64)

		// Skip line if any parsing errors
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil ||
			err5 != nil || err6 != nil || err7 != nil || err8 != nil || err9 != nil {
			continue
		}

		lat := float64(latDegVal) + float64(latMinVal)/60.0 + latSecVal/3600.0
		lon := float64(lonDegVal) + float64(lonMinVal)/60.0 + lonSecVal/3600.0
		if latHem == "S" {
			lat = -lat
		}
		if lonHem == "W" {
			lon = -lon
		}
		ele := elevation + height

		obs := FAAPoint{
			Lat:      lat,
			Lon:      lon,
			Height:   ele,
			Type:     obsType,
			Quantity: float64(quantityVal),
		}

		squareKey := generateSquareKey(lat, lon)
		square, exists := squareBuffers[squareKey]
		if !exists {
			square = &ObsSquare{
				LatDeg:  int(math.Floor(lat)),
				LonDeg:  int(math.Floor(lon)),
				Records: []FAAPoint{},
			}
			squareBuffers[squareKey] = square
		}
		square.Records = append(square.Records, obs)

	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file %s: %v", filePath, err)
	}

	return nil
}

func saveToProtobufs(squareBuffers map[string]*ObsSquare) error {
	outputDir := "assets/chunks/obstructions/data"
	os.MkdirAll(outputDir, 0755)

	for _, square := range squareBuffers {
		pbSquare := &pb.ObstructionSquare{
			LatDeg:       int32(square.LatDeg),
			LonDeg:       int32(square.LonDeg),
			Obstructions: make([]*pb.FAAPoint, 0, len(square.Records)),
		}

		for _, obs := range square.Records {
			pbObs := &pb.FAAPoint{
				Lat:      obs.Lat,
				Lon:      obs.Lon,
				Height:   obs.Height,
				Type:     obs.Type,
				Quantity: obs.Quantity,
			}
			pbSquare.Obstructions = append(pbSquare.Obstructions, pbObs)
		}

		data, err := proto.Marshal(pbSquare)
		if err != nil {
			return fmt.Errorf("error marshaling all data: %v", err)
		}

		path := filepath.Join(outputDir, CoordToString(square.LatDeg, square.LonDeg)+".pb")
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("error writing all data file: %v", err)
		}
	}

	return nil
}

// Function to scrape FAA DOF webpage and get link to most recent DOF release
func getDOFLink() (string, error) {
	FAAurl := "https://www.faa.gov/air_traffic/flight_info/aeronav/digital_products/dof"
	DOFurl := ""

	res, err := http.Get(FAAurl)
	if err != nil {
		return DOFurl, fmt.Errorf("error finding dof webpage: %v", err)
	}
	defer res.Body.Close()

	tokenizer := html.NewTokenizer(res.Body)
	counter := 0

	for {
		// End if type is error
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}

		// Grab token and look for <a> tag (hyperlink)
		token := tokenizer.Token()

		if token.Type == html.StartTagToken && token.Data == "a" {
			for _, attr := range token.Attr {
				if attr.Key == "href" {
					href := attr.Val
					if strings.Contains(href, "DOF_") && strings.HasSuffix(href, ".zip") {
						// We want the second DOF link (most recent)
						counter++
						if counter == 2 {
							DOFurl = href
							break
						}
					}
				}
			}
		}
		if DOFurl != "" {
			break
		}
	}
	if DOFurl == "" {
		return DOFurl, fmt.Errorf("error finding link on webpage: %v", err)
	}
	return DOFurl, nil
}

// Function to download latest DOF.DAT file from FAA
func downloadFAADOF(finalPath string) error {
	DOFurl, err := getDOFLink()
	if err != nil {
		return fmt.Errorf("error getting dof link: %v", err)
	}

	res, err := http.Get(DOFurl)
	if (err != nil) || (res.StatusCode != http.StatusOK) {
		return fmt.Errorf("error finding dof webpage: %v", err)
	}
	defer res.Body.Close()

	initialPath := "./assets/DOF_Files/DOF.zip"

	// Create and copy ZIP file
	zipFile, err1 := os.Create(initialPath)
	_, err2 := io.Copy(zipFile, res.Body)

	if err1 != nil || err2 != nil {
		return fmt.Errorf("error forming zipfile: %v, %v", err1, err2)
	}
	defer zipFile.Close()

	// Read ZIP file
	zipReader, err := zip.OpenReader(initialPath)
	if err != nil {
		return fmt.Errorf("error opening zip: %v", err)
	}
	defer zipReader.Close()

	// Find DOF.DAT in ZIP file
	for _, file := range zipReader.File {
		if file.Name == "DOF.DAT" {
			fileReader, err1 := file.Open()
			DATFile, err2 := os.Create(finalPath)
			_, err3 := io.Copy(DATFile, fileReader)

			if err1 != nil || err2 != nil || err3 != nil {
				return fmt.Errorf("error forming datfile: %v, %v, %v", err1, err2, err3)
			}
			defer fileReader.Close()
			defer DATFile.Close()

			// Delete ZIP file
			err = os.Remove(initialPath)
			return err
		}
	}

	// If didn't find DOF.DAT
	return fmt.Errorf("error finding dof.dat")
}

// Function to convert the release date in the DOF.DAT file to time.Time
func parseCurrencyDate(file *os.File) (time.Time, error) {
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	header := scanner.Text()

	currencyString := strings.TrimSpace(header[headerStart:headerEnd])
	formatString := "01/02/06" // Stored month/day/year
	currencyTime, err := time.Parse(formatString, currencyString)

	return currencyTime, err
}

// GenerateObstructionData puts FAA DOF data into coordinate buffers that are written to files
func GenerateObstructionData() error {
	// Check for file existence and date modified
	filePath := "./assets/DOF_Files/DOF.DAT"
	file, err := os.Open(filePath)
	daysSinceUpdate := 0.0
	if err == nil {
		fileCurrencyDate, err := parseCurrencyDate(file)
		if err != nil {
			return fmt.Errorf("Error parsing file currency: %v", err)
		}
		currentTime := time.Now()
		daysSinceUpdate = (currentTime.Sub(fileCurrencyDate)).Hours() / 24
	}
	defer file.Close()

	// DOF is updated every 56 days, and the new version is released 2 days after that
	if err != nil || daysSinceUpdate >= 58 {
		// File either doesn't exist, or is outdated
		err = downloadFAADOF(filePath)
		if err != nil {
			return fmt.Errorf("Error downloading DOF data: %v", err)
		}
	}

	squareBuffers := make(map[string]*ObsSquare)
	err = parseDOFFile(filePath, squareBuffers)
	if err != nil {
		return fmt.Errorf("Error parsing file: %v", err)
	}

	err = saveToProtobufs(squareBuffers)
	if err != nil {
		return fmt.Errorf("Error writing results: %v", err)
	}
	return nil
}
