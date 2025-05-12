package processor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// TODO group into types
// const (
// 	TOWER = iota
// 	BLDG
// 	POLE
// 	ELEC_SYS
// 	T-L_TWR
// 	TANK
// 	FENCE
// 	SIGN
// 	NAVAID
// 	GEN_UTIL
// 	WINDMILL
// 	SPIRE
// 	CTRL_TWR
// 	LANDFILL
// 	VERICAL_STRUCTURE
// 	UTILITY_POLE
// 	STACK
// 	CATENARY
// 	SOLAR_PANELS
// 	ANTENNA
// 	VERTICAL_STRUCTURE
// 	CRANE
// 	BLDG_TWR
//  RIG
// )

type FAAPoint struct {
	Lat    float64
	Lon    float64
	Height float64
}

type ObsSquare struct {
	LatDeg  int
	LonDeg  int
	Records []FAAPoint
}

// Key generation for coordinate square map
func generateSquareKey(lat, lon float64) string {
	latDeg := int(lat)
	lonDeg := int(lon)
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

		latDeg := strings.TrimSpace(line[35:37])
		latMin := strings.TrimSpace(line[38:40])
		latSec := strings.TrimSpace(line[41:46])
		latHem := strings.TrimSpace(line[46:47])

		lonDeg := strings.TrimSpace(line[48:51])
		lonMin := strings.TrimSpace(line[52:54])
		lonSec := strings.TrimSpace(line[55:60])
		lonHem := strings.TrimSpace(line[60:61])

		// TODO: decide if want obstacle type
		// ObsType := strings.TrimSpace(line[63:81])
		heightStr := strings.TrimSpace(line[84:89])
		eleStr := strings.TrimSpace(line[90:95])

		latDegVal, err1 := strconv.Atoi(latDeg)
		latMinVal, err2 := strconv.Atoi(latMin)
		latSecVal, err3 := strconv.ParseFloat(latSec, 64)
		lonDegVal, err4 := strconv.Atoi(lonDeg)
		lonMinVal, err5 := strconv.Atoi(lonMin)
		lonSecVal, err6 := strconv.ParseFloat(lonSec, 64)

		elevation, err7 := strconv.ParseFloat(eleStr, 64)
		height, err8 := strconv.ParseFloat(heightStr, 64)

		// Skip line if any parsing errors
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil ||
			err5 != nil || err6 != nil || err7 != nil || err8 != nil {
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
			Lat:    lat,
			Lon:    lon,
			Height: ele,
		}

		squareKey := generateSquareKey(lat, lon)
		square, exists := squareBuffers[squareKey]
		if !exists {
			square = &ObsSquare{
				LatDeg:  int(lat),
				LonDeg:  int(lon),
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

func writeResultsToFiles(squareBuffers map[string]*ObsSquare) error {
	outputDir := "assets/obs"

	for key, square := range squareBuffers {
		filePath := filepath.Join(outputDir, fmt.Sprintf("square_%s.csv", key))
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("error creating file %s: %v", filePath, err)
		}

		writer := bufio.NewWriter(file)

		for _, obs := range square.Records {
			_, err := writer.WriteString(fmt.Sprintf("%.6f,%.6f,%.2f\n",
				obs.Lat, obs.Lon, obs.Height))
			if err != nil {
				file.Close()
				return fmt.Errorf("error writing record: %v", err)
			}
		}

		writer.Flush()
		file.Close()
	}

	return nil
}

// this file assumes that the data is already downloaded and just needs to be parsed
// TODO: script to download and delete DOF data
// GenerateObsData puts FAA DOF data into coordinate buffers that are written to files
func GenerateObsData() {
	squareBuffers := make(map[string]*ObsSquare)
	err := parseDOFFile("./assets/DOF_Files/DOF.DAT", squareBuffers)
	if err != nil {
		fmt.Println("Error parsing file", err)
	}

	err = writeResultsToFiles(squareBuffers)
	if err != nil {
		fmt.Printf("Error writing results: %v\n", err)
	}
}
