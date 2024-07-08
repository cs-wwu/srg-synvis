package main

import (
	"flag"
	"runtime"

	"github.com/cs-wwu/srg-synvis/go/internal/renderer"
	"github.com/cs-wwu/srg-synvis/go/pkg/utils"

	"github.com/go-gl/glfw/v3.3/glfw"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
)

var (
	windowWidth        = flag.Int("width", 800, "The beginning window width")
	windowHeight       = flag.Int("height", 600, "The beginning window height")
	fragmentShaderPath = flag.String("fragPath", "assets/shaders/shader.frag", "The Path to the fragment shader")
	vertexShaderPath   = flag.String("vertPath", "assets/shaders/shader.vert", "The Path to the vertex shader")
)

// will be expanded
type Application struct {
	Window             *glfw.Window // this is a windowHandle
	FramebufferResized bool
}

func init() {
	runtime.LockOSThread()
}

func main() {
	flag.Parse()

	// Initialize Necessary Items and run the app
	utils.OrPanic(glfw.Init())
	defer glfw.Terminate()
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	utils.OrPanic(vk.Init())

	app := renderer.InitWindow(*windowWidth, *windowHeight)
	defer app.Window.Destroy() // cleanup fctn?
	renderer.InitVulkan()

	platform, err := as.NewPlatform(app)
	utils.OrPanic(err)
	defer platform.Destroy()

	// mainLoop
	for !app.Window.ShouldClose() {
		glfw.PollEvents()
		// processInput();
		// drawFrame();

		// Check if framebuffer was resized
		if app.FramebufferResized {
			app.FramebufferResized = false // Reset flag
		}
	}
}
