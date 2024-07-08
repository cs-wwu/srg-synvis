package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/go/internal/heightmap"
	"github.com/cs-wwu/srg-synvis/go/internal/mesh"
	"github.com/cs-wwu/srg-synvis/go/internal/renderer"
	"github.com/cs-wwu/srg-synvis/go/internal/shader"
	"github.com/cs-wwu/srg-synvis/go/internal/texture"
	"github.com/cs-wwu/srg-synvis/go/internal/types"
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
	Window             *glfw.Window		// this is a windowHandle
	FramebufferResized bool
}

func init() {
	runtime.LockOSThread()
}

// for window resizing
func framebufferSizeCallback(window *glfw.Window, width int, height int) {
	appPtr := window.GetUserPointer()
	if appPtr == nil {
		return
	}
	app := (*Application)(appPtr)
	app.FramebufferResized = true
}

func initWindow(windowWidth, windowHeight int) *Application {
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)

	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Vulkan", nil, nil)
	if err != nil {
		panic(fmt.Errorf("glfw window creation failed: %v", err))
	}
	// window is resizeable + detection of resize
	app := &Application{Window: window}

	ptr := unsafe.Pointer(app)
	window.SetUserPointer(ptr)

	window.SetFramebufferSizeCallback(framebufferSizeCallback)

	return app
}

func initVulkan() {
	// TODO: decide what fctns below to source out - some of these are in as
	// as.NewPlatform(app) handles createInstance(), pickPhysicalDevice(), some of createSurface(), createLogicalDevice()

	// createInstance();
	// setupDebugMessenger();
	// createSurface();
	// pickPhysicalDevice();
	// createLogicalDevice();
	// createSwapChain();
	// createImageViews();
	// createRenderPass();
	// createDescriptorSetLayout();
	// createGraphicsPipeline();
	// createCommandPool();
	// createDepthResources();
	// createFramebuffers();
	// createTextureImage();
	// createTextureImageView();
	// createTextureSampler();
	// loadhm();
	// createVertexBuffer();
	// createIndexBuffer();
	// createUniformBuffers();
	// createDescriptorPool();
	// createDescriptorSets();
	// createCommandBuffers();
	// createSyncObjects();

}

func main() {
	flag.Parse()
	
	// Initialize Necessary Items and run the app
	utils.OrPanic(glfw.Init())
	defer glfw.Terminate()
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	utils.OrPanic(vk.Init())

	app := initWindow(*windowWidth, *windowHeight)
	defer app.Window.Destroy() // cleanup fctn?
	initVulkan()

	platform, err := as.NewPlatform(app)
	utils.OrPanic(err)
	defer platform.Destroy()

	// mainLoop
	for !app.Window.ShouldClose() {
		glfw.PollEvents()

		// Check if framebuffer was resized
		if app.FramebufferResized {
			app.FramebufferResized = false // Reset flag
		}
	}
}
