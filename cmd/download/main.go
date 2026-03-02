package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cs-wwu/srg-synvis/pkg/api"
	"github.com/cs-wwu/srg-synvis/pkg/processor"
)

// This file should be used for downloading the necessary heightmaps, etc.
// This should eventually be a gui with region selection. For now it should
// at least download a portion of Washington State.

func init() {
	// Configure logging
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("download: ")
}

type Config struct {
	// Data source selection
	useNASA bool
	useOSM  bool
	useFAA  bool

	// FAA source selection
	useFAAObstructions bool
	useFAAAirspace     bool

	// Region selection
	region     string // Custom region string (e.g., "n48w123,n48w122")
	boundBox   string // Bounding box for area (e.g., "48.5,-123.5,47.5,-122.5")
	usePreset  string // Preset region name (e.g., "washington", "bellingham")
	faaStates  string // States to filter by for FAA airspace data
	faaClasses string // Classes of airspace to filter by for FAA airspace data

	// Output options
	outputDir    string
	keepZipFiles bool
	verbose      bool
}

type BoundingBox struct {
	North int // exclusive
	South int
	East  int // exclusive
	West  int
}

var presetRegions = map[string]BoundingBox{
	"bellingham": {
		North: 49,
		South: 48,
		East:  122,
		West:  -123,
	},
	"washington": {
		North: 49,
		South: 45,
		East:  117,
		West:  -125,
	},
}

// Returns all of the preset names from the regions
func getPresetNames() []string {
	names := make([]string, 0, len(presetRegions))
	for name := range presetRegions {
		names = append(names, name)
	}
	return names
}

func parseFlags() *Config {
	cfg := &Config{}

	// Data source flags
	flag.BoolVar(&cfg.useNASA, "nasa", false, "Download NASA elevation data")
	flag.BoolVar(&cfg.useOSM, "osm", false, "Download OpenStreetMap data")
	flag.BoolVar(&cfg.useFAA, "faa", false, "Download FAA airport/navigation data")

	// FAA source flags
	flag.BoolVar(&cfg.useFAAObstructions, "obs", false, "Download FAA obstruction data")
	flag.BoolVar(&cfg.useFAAAirspace, "air", false, "Download FAA airspace data")

	// Region selection flags
	flag.StringVar(&cfg.region, "region", "", "Comma-separated list of region codes (e.g., 'n48w123,n48w122')")
	flag.StringVar(&cfg.boundBox, "bbox", "", "Bounding box coordinates (e.g., '49,-125,45,-118') in format 'n, e, s, w'")
	flag.StringVar(&cfg.usePreset, "preset", "", fmt.Sprintf("Use preset region: %v", getPresetNames()))
	flag.StringVar(&cfg.faaStates, "faaStates", "", "States for airspace (e.g., 'WA,OR,CA')")
	flag.StringVar(&cfg.faaClasses, "faaClasses", "", "Airspace classes (e.g., 'B,C,D')")

	// Output option flags
	flag.StringVar(&cfg.outputDir, "out", "assets/maps/", "Output directory for downloaded files")
	flag.BoolVar(&cfg.verbose, "v", false, "Verbose output")

	flag.Parse()

	return cfg
}

// Validates the configuration
func validateConfig(cfg *Config) error {
	// Check if at least one data source is selected
	if !cfg.useNASA && !cfg.useOSM && !cfg.useFAA {
		return fmt.Errorf("at least one data source must be selected (-nasa, -osm, or -faa)")
	}

	if cfg.useFAA && (!cfg.useFAAAirspace && !cfg.useFAAObstructions) {
		return fmt.Errorf("at least one FAA source must be selected (-obs or -air)")
	}

	// Check if exactly one region selection method is used
	regionMethods := 0
	if cfg.region != "" {
		regionMethods++
	}
	if cfg.boundBox != "" {
		regionMethods++
	}
	if cfg.usePreset != "" {
		regionMethods++
	}

	if regionMethods == 0 && !cfg.useFAA {
		return fmt.Errorf("must specify region using -region, -bbox, or -preset")
	}
	if regionMethods > 1 && !cfg.useFAA {
		return fmt.Errorf("can only use one region selection method (-region, -bbox, or -preset)")
	}

	// Validate preset name if used
	if cfg.usePreset != "" {
		if _, exists := presetRegions[cfg.usePreset]; !exists {
			return fmt.Errorf("invalid preset name. Available presets: %v", getPresetNames())
		}
	}

	// Ensure output directory exists or can be created
	if err := os.MkdirAll(cfg.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	return nil
}

// BoundBoxToMaps converts a bounding box to an array of NASA region codes
// Note: North and East are exclusive bounds
func BoundBoxToMaps(bbox BoundingBox) []string {
	regions := make([]string, 0)

	// Validate bounds
	if bbox.North <= bbox.South {
		return regions
	}

	// Convert West/East to negative values for western hemisphere
	west := -abs(bbox.West)
	east := -abs(bbox.East)

	// Ensure west < east after conversion
	if west < east {
		west, east = east, west // Swap them if needed
	}

	// Iterate through the latitude bands (North to South)
	for lat := bbox.North - 1; lat >= bbox.South; lat-- {
		// Iterate through longitude bands (West to East)
		for lon := west; lon > east; lon-- {
			// Format the region code
			ns := "n"
			if lat < 0 {
				ns = "s"
			}

			// Always use 'w' since we're in western hemisphere
			ew := "w"

			// Get absolute values for formatting
			absLat := abs(lat)
			absLon := abs(lon)

			// Format the region code (e.g., "n48w123")
			region := fmt.Sprintf("%s%02d%s%03d", ns, absLat, ew, absLon)
			regions = append(regions, region)
		}
	}

	return regions
}

// Helper function to get absolute value of int
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// This should only be called once with the array of heightmaps, due to
// creating a client and signing in once
// Processes an array of heightmaps into pngs into the targetDir
// See bottom of heightmap.go for example usage
func NasaProcessHeightmaps(heightmaps []string, targetDir string) error {
	if !strings.HasSuffix(targetDir, "/") { // append a "/" if necessary
		targetDir += "/"
	}

	newHeightmaps := []string{}
	for _, heightmap := range heightmaps {
		// if "MAPS_PATH/heightmap.hgt.png" and "MAPS_PATH/heightmap.swb.png" already exists,
		// then don't add it to the new heightmaps list
		// otherwise, add it
		hgtPngPath := targetDir + heightmap + ".hgt.png"
		swbPngPath := targetDir + heightmap + ".swb.png"
		if _, err := os.Stat(hgtPngPath); os.IsNotExist(err) {
			newHeightmaps = append(newHeightmaps, heightmap)
		} else if _, err := os.Stat(swbPngPath); os.IsNotExist(err) {
			newHeightmaps = append(newHeightmaps, heightmap)
		}
	}
	heightmaps = newHeightmaps

	// This should only be called once due to client cookies and sign-in
	err := api.DownloadHeightmaps(heightmaps, targetDir)
	if err != nil {
		return fmt.Errorf("failed to download heightmaps: %w", err)
	}

	// Unzip each heightmap and remove the old zipped folders
	for _, heightmap := range heightmaps {
		zipFilePath := targetDir + heightmap + ".zip"
		outputDir := targetDir + heightmap + "/"
		err := api.Unzip(zipFilePath, outputDir)
		if err != nil {
			return fmt.Errorf("failed to unzip %s: %w", heightmap, err)
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
			return fmt.Errorf("failed to convert %s to png: %w", heightmap, err)
		}

		os.RemoveAll(outputDir) // Remove leftovers
	}

	return nil
}

// Verifies that a string is a valid region to download
func isValidRegionFormat(region string) bool {
	// Basic format validation for region codes (e.g., n48w123)
	if len(region) != 7 {
		return false
	}

	// First char should be 'n' or 's'
	if region[0] != 'n' && region[0] != 's' {
		return false
	}

	// Fourth char should be 'e' or 'w'
	if region[3] != 'e' && region[3] != 'w' {
		return false
	}

	// Other positions should be digits
	for _, pos := range []int{1, 2, 4, 5, 6} {
		if region[pos] < '0' || region[pos] > '9' {
			return false
		}
	}

	return true
}

// Parses a string bounding box to an actual bounding box
func parseBoundingBox(bbox string) (BoundingBox, error) {
	var north, south, east, west int

	// Split "49,-125,45,-118" into components
	parts := strings.Split(bbox, ",")
	if len(parts) != 4 {
		return BoundingBox{}, fmt.Errorf("invalid bounding box format: expected 4 numbers")
	}

	// Parse coordinates
	var err error
	if north, err = strconv.Atoi(strings.TrimSpace(parts[0])); err != nil {
		return BoundingBox{}, fmt.Errorf("invalid north coordinate: %w", err)
	}
	if west, err = strconv.Atoi(strings.TrimSpace(parts[1])); err != nil {
		return BoundingBox{}, fmt.Errorf("invalid west coordinate: %w", err)
	}
	if south, err = strconv.Atoi(strings.TrimSpace(parts[2])); err != nil {
		return BoundingBox{}, fmt.Errorf("invalid south coordinate: %w", err)
	}
	if east, err = strconv.Atoi(strings.TrimSpace(parts[3])); err != nil {
		return BoundingBox{}, fmt.Errorf("invalid east coordinate: %w", err)
	}

	return BoundingBox{
		North: north,
		South: south,
		East:  east,
		West:  west,
	}, nil
}

func main() {
	// Parse command line flags
	cfg := parseFlags()

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	if cfg.verbose {
		log.Printf("Config: %v", cfg)
	}

	var heightmaps []string

	if cfg.usePreset != "" {
		// Use preset regions
		if regions, exists := presetRegions[cfg.usePreset]; exists {
			heightmaps = BoundBoxToMaps(regions)
		} else {
			log.Fatalf("Unknown preset region: %s", cfg.usePreset)
		}
	} else if cfg.region != "" {
		// Use directly specified regions
		heightmaps = strings.Split(cfg.region, ",")
		// Validate each region code
		for _, region := range heightmaps {
			if !isValidRegionFormat(region) {
				log.Fatalf("Invalid region format: %s", region)
			}
		}
	} else if cfg.boundBox != "" {
		// Convert bounding box to regions
		boundingBox, err := parseBoundingBox(cfg.boundBox)
		if err != nil {
			log.Fatalf("Failed to parse bounding box: %b", err)
		}
		heightmaps = BoundBoxToMaps(boundingBox)
		if err != nil {
			log.Fatalf("Failed to process bounding box: %b", err)
		}
	}

	if cfg.useNASA {
		if err := NasaProcessHeightmaps(heightmaps, cfg.outputDir); err != nil {
			log.Printf("Failed to download Nasa Heightmaps: %v", err)
		}
	}

	if cfg.useFAA {
		if cfg.useFAAAirspace {
			// Turn user specified filters into filters for query
			airspaceStates := strings.Fields(strings.ToUpper(strings.ReplaceAll(cfg.faaStates, ",", " ")))
			airspaceClasses := strings.Fields(strings.ToUpper(strings.ReplaceAll(cfg.faaClasses, ",", " ")))
			airspaceFilters, err := api.NewAirspaceFilter(airspaceStates, airspaceClasses)
			if err != nil {
				log.Printf("Failed to download FAA Airspace data: %v", err)
				return
			}
			log.Printf("Downloading airspaces with filters: %s", airspaceFilters)
			err = api.DownloadAndChunkAirspaces(airspaceFilters)
			if err != nil {
				log.Printf("Failed to download FAA Airspace data: %v", err)
			}
		} else {
			if err := processor.GenerateObstructionData(); err != nil {
				log.Printf("Failed to download FAA data: %v", err)
			}
		}
	}

	if cfg.useOSM {
		if err := processor.GenerateRoadProtobufs(heightmaps); err != nil {
			log.Printf("Failed to download OSM data: %v", err)
		}
	}

	log.Printf("Finished Downloading\n")
}
