package main

import (
	"flag"
	"log"
	"runtime"
	"time"

	"github.com/cs-wwu/srg-synvis/go/internal/renderer"
	"github.com/cs-wwu/srg-synvis/go/internal/shader"
	"github.com/cs-wwu/srg-synvis/go/pkg/utils"

	"github.com/go-gl/glfw/v3.3/glfw"

	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
)

var (
	windowWidth        = flag.Int("width", 800, "The beginning window width")
	windowHeight       = flag.Int("height", 600, "The beginning window height")
	fragmentShaderPath = flag.String("fragPath", "assets/shaders/shader.frag", "The Path to the fragment shader")
	vertexShaderPath   = flag.String("vertPath", "assets/shaders/shader.vert", "The Path to the vertex shader")
)

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile) // TODO unecessary?
}

func main() {
	flag.Parse()

	shader.CompileShaders(*vertexShaderPath, *fragmentShaderPath)

	// Initialize Necessary Items and run the app
	utils.OrPanic(glfw.Init())
	defer glfw.Terminate()
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	utils.OrPanic(vk.Init())

	app := renderer.NewApplication(1.0)
	defer app.Window.Destroy() // cleanup fctn?
	// renderer.InitVulkan()
	reqDim := app.VulkanSwapchainDimensions()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	window, err := glfw.CreateWindow(int(reqDim.Width), int(reqDim.Height), "VulkanCube (GLFW)", nil, nil)
	utils.OrPanic(err)
	app.WindowHandle = window

	platform, err := as.NewPlatform(app)
	utils.OrPanic(err)
	defer platform.Destroy()

	dim := app.Context().SwapchainDimensions()
	log.Printf("Initialized %s with %+v swapchain", app.VulkanAppName(), dim)

	doneC := make(chan struct{}, 2)
	exitC := make(chan struct{}, 2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})

	fpsDelay := time.Second / 60
	fpsTicker := time.NewTicker(fpsDelay)
	for {
		select {
		case <-exitC:
			app.Destroy()
			platform.Destroy()
			window.Destroy()
			glfw.Terminate()
			fpsTicker.Stop()
			doneC <- struct{}{}
			return
		case <-fpsTicker.C:
			if window.ShouldClose() {
				exitC <- struct{}{}
				continue
			}
			glfw.PollEvents()
			app.NextFrame()

			imageIdx, outdated, err := app.Context().AcquireNextImage()
			utils.OrPanic(err)
			if outdated {
				imageIdx, _, err = app.Context().AcquireNextImage()
				utils.OrPanic(err)
			}
			_, err = app.Context().PresentImage(imageIdx)
			utils.OrPanic(err)
		}
	}

	// mainLoop
	// for !app.Window.ShouldClose() {
	// 	glfw.PollEvents()
	// 	// ProcessInput(); // TODO unecessary until the rest is rendered
	// 	app.NextFrame() // TODO unsure if this is the write function for drawFrame()

	// 	// Check if framebuffer was resized
	// 	if app.FramebufferResized {
	// 		app.FramebufferResized = false // Reset flag
	// 	}
	// }
}
