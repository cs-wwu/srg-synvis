package main

import (
	"log"
	"runtime"
	"time"

	"go-testing/internal/renderer"

	"github.com/go-gl/glfw/v3.3/glfw"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
)

const (
	WINDOW_HEIGHT = 480
	WINDOW_WIDTH  = 800
)

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

type Application struct {
	*renderer.Application
	debugEnabled bool
}

func (app *Application) VulkanSurface(instance vk.Instance) (surface vk.Surface) {
	surfPtr, err := app.WindowHandle.CreateWindowSurface(instance, nil)
	if err != nil {
		log.Println(err)
		return vk.NullSurface
	}
	return vk.SurfaceFromPointer(surfPtr)
}

func (app *Application) VulkanAppName() string {
	return "Vulkan"
}

func (app *Application) VulkanLayers() []string {
	return []string{
		// "VK_LAYER_GOOGLE_threading",
		// "VK_LAYER_LUNARG_parameter_validation",
		// "VK_LAYER_LUNARG_object_tracker",
		// "VK_LAYER_LUNARG_core_validation",
		// "VK_LAYER_LUNARG_api_dump",
		// "VK_LAYER_LUNARG_swapchain",
		// "VK_LAYER_GOOGLE_unique_objects",
	}
}

func (app *Application) VulkanDebug() bool {
	return false // a.debugEnabled
}

func (app *Application) VulkanDeviceExtensions() []string {
	return []string{
		"VK_KHR_swapchain",
	}
}

func (app *Application) VulkanSwapchainDimensions() *as.SwapchainDimensions {
	return &as.SwapchainDimensions{
		Width: WINDOW_WIDTH, Height: WINDOW_HEIGHT, Format: vk.FormatB8g8r8a8Unorm,
	}
}

func (app *Application) VulkanInstanceExtensions() []string {
	extensions := app.WindowHandle.GetRequiredInstanceExtensions()
	if app.debugEnabled {
		extensions = append(extensions, "VK_EXT_debug_report")
	}
	return extensions
}

func NewApplication(debugEnabled bool) *Application {
	return &Application{
		Application: renderer.NewApplication(),

		debugEnabled: debugEnabled,
	}
}

func main() {
	orPanic(glfw.Init())
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	orPanic(vk.Init())
	defer closer.Close()

	app := NewApplication(true)
	reqDim := app.VulkanSwapchainDimensions()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	window, err := glfw.CreateWindow(int(reqDim.Width), int(reqDim.Height), "Vulkan", nil, nil)
	orPanic(err)
	app.WindowHandle = window

	// creates a new platform, also initializes Vulkan context in the app
	platform, err := as.NewPlatform(app)
	orPanic(err)

	dim := app.Context().SwapchainDimensions()
	log.Printf("Initialized %s with %+v swapchain", app.VulkanAppName(), dim)

	// some sync logic
	doneC := make(chan struct{}, 2)
	exitC := make(chan struct{}, 2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})

	fpsDelay := time.Second / 60
	fpsTicker := time.NewTicker(fpsDelay)

	// Additional ticker for printing FPS
	fpsPrintTicker := time.NewTicker(time.Second)
	var frameCount int

	for {
		select {
		case <-exitC:
			app.Destroy()
			platform.Destroy()
			window.Destroy()
			glfw.Terminate()
			fpsTicker.Stop()
			fpsPrintTicker.Stop()
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
			orPanic(err)
			if outdated {
				imageIdx, _, err = app.Context().AcquireNextImage()
				orPanic(err)
			}
			_, err = app.Context().PresentImage(imageIdx)
			orPanic(err)

			// Increment the frame count
			frameCount++
		case <-fpsPrintTicker.C:
			// Print the FPS and reset the frame count
			log.Printf("FPS: %d", frameCount)
			frameCount = 0
		}
	}
}

func orPanic(err interface{}) {
	switch v := err.(type) {
	case error:
		if v != nil {
			panic(err)
		}
	case vk.Result:
		if err := vk.Error(v); err != nil {
			panic(err)
		}
	case bool:
		if !v {
			panic("condition failed: != true")
		}
	}
}
