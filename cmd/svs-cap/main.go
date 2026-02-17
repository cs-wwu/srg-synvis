package main

import (
	"log"
	"runtime"

	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/linmath"
)

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0

	METERS_TO_WORLD = 1852.0 / ARC_DEGREES_TO_MINUTES
)

func main() {
	var config models.AppConfig

	config.DebugMode = true

	config.CameraStartPos = linmath.Vec3{-441970, 700, -171900}
	config.CameraStartDir = linmath.Vec3{0, 0, -1}

	config.CaptureOnly = true
	config.CaptureWidth = 1024
	config.CaptureHeight = 1024

	config.RenderDistance = (150.0 * 1000.0) / METERS_TO_WORLD
	config.Fov = 70

	err := vk.SetDefaultGetInstanceProcAddr()
	if err != nil {
		log.Fatalf("Failed to set default GetInstanceProcAddr: %v", err)
	}
	if err := vk.Init(); err != nil {
		log.Fatalf("Failed to initialize Vulkan: %v", err)
	}

	app := rendering.NewApplication(config)
	as.NewPlatform(app)

	app.RenderCapture()
}
