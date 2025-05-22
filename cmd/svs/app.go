package main

import (
	"log"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/renderer"
	as "github.com/cs-wwu/srg-synvis/pkg/asche"

	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
)

func runApplication(config models.AppConfig) error {
	// Initialize GLFW and Vulkan
	if err := glfw.Init(); err != nil {
		return err
	}
	defer glfw.Terminate()

	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	if err := vk.Init(); err != nil {
		return err
	}
	defer closer.Close()

	// Create application
	app := renderer.NewApplication(config)

	// Set up window and monitor
	monitor, videoMode := setupMonitor(config.MonitorNum)
	window := createWindow(app, monitor, videoMode)
	app.WindowHandle = window

	// Create platform
	platform, err := as.NewPlatform(app)
	if err != nil {
		return err
	}

	// Initialize and run
	dim := app.Context().SwapchainDimensions()
	frameDelay, refreshRate := getRefreshRate(monitor)
	log.Printf("Initialized %s with %+v swapchain, %d FPS", app.VulkanAppName(), dim, refreshRate)

	// Start the main render loop
	runMainLoop(app, window, platform, frameDelay)

	return nil
}

func setupMonitor(monitorNum int) (*glfw.Monitor, *glfw.VidMode) {
	if monitorNum <= 0 {
		log.Printf("Running in windowed mode")
		return nil, nil
	}

	monitors := glfw.GetMonitors()
	log.Printf("Found %d monitors:", len(monitors))

	for i, m := range monitors {
		mode := m.GetVideoMode()
		x, y := m.GetPos()
		log.Printf("  Monitor %d: %dx%d@%dHz at position (%d,%d) - %s",
			i+1, mode.Width, mode.Height, mode.RefreshRate, x, y, m.GetName())
	}

	if monitorNum > len(monitors) {
		log.Printf("Warning: Monitor %d not found. Available monitors: %d. Defaulting to windowed mode.",
			monitorNum, len(monitors))
		return nil, nil
	}

	monitor := monitors[monitorNum-1]
	videoMode := monitor.GetVideoMode()
	x, y := monitor.GetPos()

	log.Printf("Selected monitor %d (%s): %dx%d@%dHz at position (%d,%d)",
		monitorNum, monitor.GetName(), videoMode.Width, videoMode.Height,
		videoMode.RefreshRate, x, y)

	return monitor, videoMode
}

func createWindow(app *renderer.Application, monitor *glfw.Monitor, videoMode *glfw.VidMode) *glfw.Window {
	var windowWidth, windowHeight int
	if monitor != nil {
		windowWidth = videoMode.Width
		windowHeight = videoMode.Height
	} else {
		reqDim := app.VulkanSwapchainDimensions()
		windowWidth = int(reqDim.Width)
		windowHeight = int(reqDim.Height)
	}

	app.SetSwapchainDimensions(&as.SwapchainDimensions{
		Width:  uint32(windowWidth),
		Height: uint32(windowHeight),
		Format: vk.FormatB8g8r8a8Unorm,
	})

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Vulkan SVS", monitor, nil)
	if err != nil {
		panic(err)
	}

	return window
}

func getRefreshRate(monitor *glfw.Monitor) (time.Duration, int) {
	var refreshRate int
	if monitor != nil {
		refreshRate = monitor.GetVideoMode().RefreshRate
	} else {
		refreshRate = glfw.GetPrimaryMonitor().GetVideoMode().RefreshRate
	}

	if refreshRate <= 0 {
		log.Printf("Unable to parse monitor refresh rate. Falling back to 60fps.")
		refreshRate = 60
	}

	return time.Second / time.Duration(refreshRate), refreshRate
}

func runMainLoop(app *renderer.Application, window *glfw.Window, platform as.Platform, frameDelay time.Duration) {
	doneC := make(chan struct{}, 2)
	exitC := make(chan struct{}, 2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})

	fpsTicker := time.NewTicker(frameDelay)
	defer fpsTicker.Stop()

	for {
		select {
		case <-exitC:
			app.Destroy()
			platform.Destroy()
			window.Destroy()
			glfw.Terminate()
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
			if err != nil {
				panic(err)
			}
			if outdated {
				log.Printf("Failed to acquire next image: (idx: %v, outdated: %v)", imageIdx, outdated)
			}
			_, err = app.Context().PresentImage(imageIdx)
			if err != nil {
				panic(err)
			}
		}
	}
}
