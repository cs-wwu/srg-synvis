package main

import (
	"flag"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/cs-wwu/srg-synvis/internal/models"
	lin "github.com/xlab/linmath"
)

const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0

	METERS_TO_WORLD = 1852.0 / ARC_DEGREES_TO_MINUTES
)

func parseConfig() models.AppConfig {
	var config models.AppConfig

	// Existing flags
	flag.BoolVar(&config.UseGPS, "gps", false, "Enable GPS")
	flag.BoolVar(&config.UseHat, "hat", false, "Enable Hat")
	flag.IntVar(&config.MonitorNum, "monitor", 0, "Monitor selection (0 = windowed, 1 = primary monitor, etc.)")

	// New flags
	renderDistance := flag.Float64("render-distance", 150.0, "Maximum render distance in kilometers")
	
	// Enhanced position flag
	startPosString := flag.String("pos", "48.5,-123.5,2600", "Starting Camera Position (lat,lon,height) where height is in meters")
	startDirString := flag.String("dir", "0,0,-1", "Starting Camera Direction (x,y,z)")
	fov := flag.Float64("fov", 60.0, "Field of view (degrees)")
	
	flag.BoolVar(&config.DisableTerrainWarnings, "disable-warnings", false, "Disable terrain height warnings (red coloring)")
	flag.BoolVar(&config.DebugMode, "debug", false, "Enable debug mode")
	flag.BoolVar(&config.DisableFog, "disable-fog", false, "Disables Fog")

	flag.Parse()

	// Parse starting position
	config.CameraStartPos = parsePosition(*startPosString)
	config.CameraStartDir = parseDirection(*startDirString)
	*renderDistance = (*renderDistance * 1000.0) / METERS_TO_WORLD

	config.RenderDistance = float32(*renderDistance)
	config.Fov = float32(*fov)

	// Validate configuration
	validateConfig(&config)

	if config.DebugMode {
		log.Println("Debug mode enabled")
		log.Printf("%+v\n", config)
	}

	return config
}

func parsePosition(posString string) lin.Vec3 {
	// Default position
	pos := lin.Vec3{-441970, 700, -171900}

	parts := strings.Split(posString, ",")
	if len(parts) != 3 {
		log.Printf("Warning: Invalid position format '%s', using default position", posString)
		return pos
	}

	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 32)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32)
	height, err3 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 32)

	if err1 != nil || err2 != nil || err3 != nil {
		log.Printf("Warning: Failed to parse position '%s', using default position", posString)
		return pos
	}

	log.Printf("Starting position: lat=%v, lon=%v, height=%v", lat, lon, height)

	// Convert to arcseconds (Vulkan logic)
	return lin.Vec3{
		float32(lon * ARC_DEGREES_TO_SECONDS),             // longitude to arcseconds
		float32(height * (ARC_DEGREES_TO_MINUTES / 1852)), // meters to arcseconds
		float32(-lat * ARC_DEGREES_TO_SECONDS),            // latitude to arcseconds (flipped for vulkan coords)
	}
}

func parseDirection(dirString string) lin.Vec3 {
	// Default direction (looking forward)
	dir := lin.Vec3{0, 0, -1}

	parts := strings.Split(dirString, ",")
	if len(parts) != 3 {
		log.Printf("Warning: Invalid direction format '%s', using default direction", dirString)
		return dir
	}

	x, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 32)
	y, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32)
	z, err3 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 32)

	if err1 != nil || err2 != nil || err3 != nil {
		log.Printf("Warning: Failed to parse direction '%s', using default direction", dirString)
		return dir
	}

	// Create and normalize the direction vector
	parsedDir := lin.Vec3{float32(x), float32(y), float32(z)}

	// Check for zero vector
	length := float32(math.Sqrt(float64(parsedDir[0]*parsedDir[0] + parsedDir[1]*parsedDir[1] + parsedDir[2]*parsedDir[2])))
	if length < 0.001 {
		log.Printf("Warning: Direction vector too small '%s', using default direction", dirString)
		return dir
	}

	// Normalize
	parsedDir[0] /= length
	parsedDir[1] /= length
	parsedDir[2] /= length

	log.Printf("Starting direction: x=%v, y=%v, z=%v", parsedDir[0], parsedDir[1], parsedDir[2])

	return parsedDir
}

func validateConfig(config *models.AppConfig) {
	if config.RenderDistance <= 0 {
		log.Printf("Warning: Invalid render distance %v, using default 5000.0", config.RenderDistance)
		config.RenderDistance = 5000.0
	}

	if config.RenderDistance > 20000.0 {
		log.Printf("Warning: Render distance %v is very large, may cause performance issues", config.RenderDistance)
	}

	if config.Fov <= 0 || config.Fov >= 180 {
		log.Printf("Warning: Invalid FOV %v, using default 60.0", config.Fov)
		config.Fov = 60.0
	}

	if config.DisableTerrainWarnings {
		log.Printf("Terrain height warnings disabled")
	}

	if config.UseGPS {
		log.Printf("GPS enabled")
	}

	if config.UseHat {
		log.Printf("Sense Hat enabled")
	}
}
