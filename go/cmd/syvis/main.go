package main

import (
	"log"
	"runtime"
	"time"
	
    "github.com/cs-wwu/srg-synvis/go/internal/renderer"
    "github.com/cs-wwu/srg-synvis/go/internal/mesh"
    "github.com/cs-wwu/srg-synvis/go/internal/shader"
    "github.com/cs-wwu/srg-synvis/go/internal/texture"
    "github.com/cs-wwu/srg-synvis/go/internal/heightmap"
    "github.com/cs-wwu/srg-synvis/go/internal/types"
    "github.com/cs-wwu/srg-synvis/go/pkg/utils"

	"github.com/go-gl/glfw/v3.3/glfw"
	as "github.com/vulkan-go/asche"
    vk "github.com/vulkan-go/vulkan"
)

var WIDTH int = 800;
var HEIGHT int = 600;

func init() {
	runtime.LockOSThread()
}

func initWindow() {
	//glfwWindowHint(GLFW_CLIENT_API, GLFW_NO_API); 
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI);

	window, err := glfw.CreateWindow(WIDTH, HEIGHT, "Vulkan", nil, nil); // (width, height, windowTitle, windowMoniter, OpenGLparam)
	utils.orPanic(err);
	// window is resizeable + detection of resize
	glfw.Window.SetUserPointer(window);
	glfw.Window.SetFramebufferSizeCallback(framebufferResizeCallback);
}

func main() {
	// Initialize Necessary Items and run the app
	utils.orPanic(glfw.Init())
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	utils.orPanic(vk.Init())
	// defer close

	app := NewApplication(true)
	window := initWindow()
	app.windowHandle = window
	defer window.Destroy()
}
