// Package Main for running the main application of the vulkan svs
package main

import (
	"flag"
	"log"
	"os/exec"
	"runtime"
	"time"

	as "github.com/cs-wwu/srg-synvis/asche"
	"github.com/cs-wwu/srg-synvis/internal/renderer"

	"github.com/go-gl/glfw/v3.3/glfw"
	// as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
)

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

type AppConfig struct {
	gps        bool
	hat        bool
	monitorNum int
}

func parseFlags() AppConfig {
	config := AppConfig{}
	flag.BoolVar(&config.gps, "gps", false, "Enable GPS")
	flag.BoolVar(&config.hat, "hat", false, "Enable Hat")
	flag.IntVar(&config.monitorNum, "monitor", 0, "Monitor selection (0 = windowed, 1 = primary monitor, etc.)")
	flag.Parse()
	return config
}

func initGPS() error {
	cmd := exec.Command("sudo", "gpsd", "/dev/ttyAMA0", "-F", "/var/run/gpsd.sock")
	return cmd.Run()
}

// Sets up the monitor and finds the dimensions/refresh rate
// Default to nil on failure (still works)
func setupMonitor(monitorNum int) (*glfw.Monitor, *glfw.VidMode) {
	// Return nil for windowed mode
	if monitorNum <= 0 {
		log.Printf("Running in windowed mode")
		return nil, nil
	}

	monitors := glfw.GetMonitors()
	log.Printf("Found %d monitors:", len(monitors))

	// Log all available monitors
	for i, m := range monitors {
		mode := m.GetVideoMode()
		x, y := m.GetPos()
		log.Printf("  Monitor %d: %dx%d@%dHz at position (%d,%d) - %s",
			i+1,
			mode.Width,
			mode.Height,
			mode.RefreshRate,
			x,
			y,
			m.GetName())
	}

	// Check if requested monitor exists
	if monitorNum > len(monitors) {
		log.Printf("Warning: Monitor %d not found. Available monitors: %d. Defaulting to windowed mode.",
			monitorNum, len(monitors))
		return nil, nil
	}

	// Get selected monitor
	monitor := monitors[monitorNum-1]
	videoMode := monitor.GetVideoMode()
	x, y := monitor.GetPos()

	log.Printf("Selected monitor %d (%s): %dx%d@%dHz at position (%d,%d)",
		monitorNum,
		monitor.GetName(),
		videoMode.Width,
		videoMode.Height,
		videoMode.RefreshRate,
		x,
		y)

	return monitor, videoMode
}

// Create the main window based on config specifications
// Returns the created window, panics on failure (cannot proceed)
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
	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Vulkan", monitor, nil)
	orPanic(err)
	return window
}

// Returns the frame delay and refresh rate of the monitor
// Defaults to 60fps on failure
func getRefreshRate(monitor *glfw.Monitor) (time.Duration, int) {
	var refreshRate int
	if monitor != nil {
		refreshRate = monitor.GetVideoMode().RefreshRate
	} else {
		refreshRate = glfw.GetPrimaryMonitor().GetVideoMode().RefreshRate
	}

	// Default to 60fps on failure
	if refreshRate <= 0 {
		log.Printf("Unable to parse monitor refresh rate. Falling back to 60fps.")
		refreshRate = 60
	}

	return time.Second / time.Duration(refreshRate), refreshRate
}

// Main vulkan render loop
func runMainLoop(app *renderer.Application, window *glfw.Window, platform as.Platform, frameDelay time.Duration) {
	doneC := make(chan struct{}, 2)
	exitC := make(chan struct{}, 2)
	defer closer.Bind(func() {
		exitC <- struct{}{}
		<-doneC
		log.Println("Bye!")
	})

	fpsTicker := time.NewTicker(frameDelay)

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
			orPanic(err)
			if outdated {
				log.Printf("Failed to acquire next image: (idx: %v, outdated: %v, err: %s)", imageIdx, outdated, err.Error())
			}
			_, err = app.Context().PresentImage(imageIdx)
			orPanic(err)
		}
	}
}

func main() {
	config := parseFlags()

	// Start the gps if we set the gps flag(s)
	if config.gps {
		if err := initGPS(); err != nil {
			log.Fatal(err)
		}
	}

	// Initialize vulkan and glfw
	orPanic(glfw.Init())
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	orPanic(vk.Init())
	defer closer.Close()

	app := renderer.NewApplication(config.hat, config.gps)
	app.Gps = config.gps
	app.Hat = config.hat

	// Set up the window, monitor, and refresh rate
	monitor, videoMode := setupMonitor(config.monitorNum)
	window := createWindow(app, monitor, videoMode)
	app.WindowHandle = window

	platform, err := as.NewPlatform(app)
	orPanic(err)

	dim := app.Context().SwapchainDimensions()
	log.Printf("Initialized %s with %+v swapchain", app.VulkanAppName(), dim)

	frameDelay, refreshRate := getRefreshRate(monitor)
	log.Printf("Running at %d FPS", refreshRate)

	// Start the main renderer loop
	runMainLoop(app, window, platform, frameDelay)
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
