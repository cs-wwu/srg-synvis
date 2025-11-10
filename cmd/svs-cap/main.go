package main

import (
	"log"
	"runtime"

	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	as "github.com/cs-wwu/srg-synvis/pkg/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/linmath"
)

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

func main() {
	var config models.AppConfig

	config.CameraStartPos = linmath.Vec3([]float32{48.5, -123.5, 2600})
	config.CameraStartDir = linmath.Vec3([]float32{0, 0, -1})

	config.CaptureOnly = true

	err := vk.SetDefaultGetInstanceProcAddr()
	if err != nil {
		log.Fatalf("Failed to set default GetInstanceProcAddr: %v", err)
	}
	if err := vk.Init(); err != nil {
		log.Fatalf("Failed to initialize Vulkan: %v", err)
	}

	app := rendering.NewApplication(config)
	as.NewPlatform(app)
}
