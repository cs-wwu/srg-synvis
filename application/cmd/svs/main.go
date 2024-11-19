// Package Main for running the main application of the vulkan svs
package main

import (
	"log"
	"runtime"
	"time"
	"flag"

	as "github.com/cs-wwu/srg-synvis/asche"
	"github.com/cs-wwu/srg-synvis/internal/renderer"

	"github.com/go-gl/glfw/v3.3/glfw"
	// as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/closer"
)

const FRAME_RATE = 60

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

func main() {
	// Define and get flags
	gpsFlag := flag.Bool("gps", false, "Enable GPS")
	hatFlag := flag.Bool("hat", false, "Enable Hat")
	flag.Parse()

	orPanic(glfw.Init())
	vk.SetGetInstanceProcAddr(glfw.GetVulkanGetInstanceProcAddress())
	orPanic(vk.Init())
	defer closer.Close()

	app := renderer.NewApplication()
	app.Gps = *gpsFlag
	app.Hat = *hatFlag
	reqDim := app.VulkanSwapchainDimensions()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	window, err := glfw.CreateWindow(int(reqDim.Width), int(reqDim.Height), "Vulkan", nil, nil)
	orPanic(err)
	app.WindowHandle = window

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

	fpsDelay := time.Second / FRAME_RATE
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
			orPanic(err)
			if outdated {
				imageIdx, _, err = app.Context().AcquireNextImage()
				orPanic(err)
			}
			_, err = app.Context().PresentImage(imageIdx)
			orPanic(err)
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
