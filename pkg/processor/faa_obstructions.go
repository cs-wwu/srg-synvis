package processor

import (
	"bufio"
	"fmt"
	pb "github.com/cs-wwu/srg-synvis/assets/obs"
	"google.golang.org/protobuf/proto"
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

const (
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

	hgtStart = 84
	hgtEnd   = 89
	eleStart = 90
	eleEnd   = 95
)

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

		latDeg := strings.TrimSpace(line[latDegStart:latDegEnd])
		latMin := strings.TrimSpace(line[latMinStart:latMinEnd])
		latSec := strings.TrimSpace(line[latSecStart:latSecEnd])
		latHem := strings.TrimSpace(line[latSecEnd : latSecEnd+1])

		lonDeg := strings.TrimSpace(line[lonDegStart:lonDegEnd])
		lonMin := strings.TrimSpace(line[lonMinStart:lonMinEnd])
		lonSec := strings.TrimSpace(line[lonSecStart:lonSecEnd])
		lonHem := strings.TrimSpace(line[lonSecEnd : lonSecEnd+1])

		// TODO: decide if want obstacle type
		// ObsType := strings.TrimSpace(line[63:81])
		heightStr := strings.TrimSpace(line[hgtStart:hgtEnd])
		eleStr := strings.TrimSpace(line[eleStart:eleEnd])

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

func saveToProtobufs(squareBuffers map[string]*ObsSquare) error {
	outputDir := "assets/obs"

	allData := &pb.DOFData{
		Squares: make([]*pb.CoordSquare, 0, len(squareBuffers)),
	}

	for _, square := range squareBuffers {
		pbSquare := &pb.CoordSquare{
			LatDeg:       int32(square.LatDeg),
			LonDeg:       int32(square.LonDeg),
			Obstructions: make([]*pb.FAAPoint, 0, len(square.Records)),
		}

		for _, obs := range square.Records {
			pbObs := &pb.FAAPoint{
				Lat:    obs.Lat,
				Lon:    obs.Lon,
				Height: obs.Height,
			}
			pbSquare.Obstructions = append(pbSquare.Obstructions, pbObs)
		}

		allData.Squares = append(allData.Squares, pbSquare)

	}

	// Save all data to a single protobuf file
	allDataBytes, err := proto.Marshal(allData)
	if err != nil {
		return fmt.Errorf("error marshaling all data: %v", err)
	}

	allDataFilePath := filepath.Join(outputDir, "all_dof_data.pb")
	if err := os.WriteFile(allDataFilePath, allDataBytes, 0644); err != nil {
		return fmt.Errorf("error writing all data file: %v", err)
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

	err = saveToProtobufs(squareBuffers)
	if err != nil {
		fmt.Printf("Error writing results: %v\n", err)
	}
}
