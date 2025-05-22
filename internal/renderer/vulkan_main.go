// Package renderer for the internal renderings of the svs application
package renderer

import (
	"fmt"
	"log"
	"math"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/internal/models"
	as "github.com/cs-wwu/srg-synvis/pkg/asche"

	// as "github.com/vulkan-go/asche"
	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file is for initializing and handling the main vulkan necessities

func NewApplication(cfg models.AppConfig) *Application {
	camera := NewCamera(cfg.UseHat, cfg.CameraStartPos, cfg.CameraStartDir)
	if cfg.UseHat {
		go camera.TrySense("./internal/sensors/hat.py")
	}

	if cfg.UseGPS {
		go camera.TryGps()
	}

	fpsCounter := NewFPSCounter()
	app := &Application{
		camera:          camera,
		fpsCounter:      fpsCounter,
		Gps:             cfg.UseGPS,
		Hat:             cfg.UseHat,
		renderDistance:  cfg.RenderDistance,
		disableWarnings: cfg.DisableTerrainWarnings,
	}

	return app
}

type FPSCounter struct {
	frames   uint32
	lastTime float64
	fpsTime  float64
	fps      float64
}

func NewFPSCounter() *FPSCounter {
	return &FPSCounter{
		lastTime: glfw.GetTime(),
		fpsTime:  glfw.GetTime(),
	}
}

// Update FPS counter and print
func (fc *FPSCounter) Update(visibleChunks int) {
	currentTime := glfw.GetTime()
	fc.frames++

	// Update FPS every second
	if currentTime-fc.fpsTime >= 1.0 {
		fc.fps = float64(fc.frames) / (currentTime - fc.fpsTime)
		fc.frames = 0
		fc.fpsTime = currentTime

		// Clear line and print new FPS
		fmt.Printf("\r\033[K") // Clear current line
		fmt.Printf("FPS: %.1f | Frame Time: %.2fms | Visible Chunks: %d",
			fc.fps,
			1000.0/fc.fps,
			visibleChunks,
		)
	}
}

func (app *Application) VulkanContextCleanup() error {
	fmt.Println()

	// Wait for device to be idle before cleanup
	dev := app.Context().Device()
	vk.DeviceWaitIdle(dev)

	app.TerrainManager.Destroy()

	vk.DestroyDescriptorPool(dev, app.descPool, nil)
	vk.DestroyPipeline(dev, app.pipeline, nil)

	// Skybox resources
	vk.DestroyBuffer(dev, app.skyboxVertexBuffer, nil)
	vk.FreeMemory(dev, app.skyboxVertexMemory, nil)
	vk.DestroyBuffer(dev, app.skyboxIndexBuffer, nil)
	vk.FreeMemory(dev, app.skyboxIndexMemory, nil)
	vk.DestroyPipeline(dev, app.skyboxPipeline, nil)

	vk.DestroyPipelineCache(dev, app.pipelineCache, nil)
	vk.DestroyRenderPass(dev, app.renderPass, nil)
	vk.DestroyPipelineLayout(dev, app.pipelineLayout, nil)
	vk.DestroyDescriptorSetLayout(dev, app.descLayout, nil)

	for i := range app.textures {
		app.textures[i].Destroy(dev)
	}

	app.depth.Destroy(dev)
	return nil
}

func (app *Application) NextFrame() {
	// Get visible chunks count thread-safely
	app.frameCount++

	app.ubo.Time = float32(math.Mod(glfw.GetTime(), 3600.0))

	// Process Input and update the camera
	app.camera.ProcessInput(app.WindowHandle)
	app.ubo.View = app.camera.GetViewMatrix()

	// Set camera position in UBO for terrain height warnings
	app.ubo.CameraPos = *app.camera.Pos // Add this line to update camera position in UBO
}

// called every frame by asche
// note : we use a ubo mapped to keep track of memory which saves some steps here,
// but goes against as.createBuffer
func (app *Application) VulkanContextInvalidate(imageIdx int) error {
	// Current UBO update logic
	dev := app.Context().Device()
	res := app.Context().SwapchainImageResources()[imageIdx]

	temp := new(lin.Mat4x4)
	temp.Invert(&app.ubo.Model)
	app.ubo.NormalM.Transpose(temp)

	data := app.ubo.Data()
	var pData unsafe.Pointer
	ret := vk.MapMemory(dev, res.UniformMemory(), 0, vk.DeviceSize(len(data)), 0, &pData)
	OrPanic(as.NewError(ret))

	n := vk.Memcopy(pData, data)
	if n != len(data) {
		log.Printf("vulkan warning: failed to copy data, %d != %d", n, len(data))
	}
	vk.UnmapMemory(dev, res.UniformMemory())

	// New command buffer update logic
	cmd := res.CommandBuffer()
	app.updateCommandBuffer(res, cmd)

	if app.frameCount%300 == 0 { // Every 5 seconds at 60fps
		log.Printf("DEBUG: renderDistance in app = %.1f", app.renderDistance)
		log.Printf("DEBUG: renderDistance in UBO = %.1f", app.ubo.RenderDistance)
		log.Printf("DEBUG: camera pos = (%.1f, %.1f, %.1f)",
			app.camera.Pos[0], app.camera.Pos[1], app.camera.Pos[2])
		log.Printf("DEBUG: disable warnings = %t", app.disableWarnings)
		log.Printf("DEBUG: fogStart would be = %.1f", app.ubo.RenderDistance-1500.0)
	}

	return nil
}

func (app *Application) updateCommandBuffer(res *as.SwapchainImageResources, cmd vk.CommandBuffer) {
	// Wait for command buffer to complete before reusing
	dev := app.Context().Device()
	vk.DeviceWaitIdle(dev)

	ret := vk.BeginCommandBuffer(cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit),
	})
	OrPanic(as.NewError(ret))

	clearValues := make([]vk.ClearValue, 2)
	clearValues[1].SetDepthStencil(1, 0)
	clearValues[0].SetColor([]float32{0.0, 0.0, 0.0, 1.0}) // Black background as fallback

	vk.CmdBeginRenderPass(cmd, &vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  app.renderPass,
		Framebuffer: res.Framebuffer(),
		RenderArea: vk.Rect2D{
			Offset: vk.Offset2D{X: 0, Y: 0},
			Extent: vk.Extent2D{Width: app.width, Height: app.height},
		},
		ClearValueCount: 2,
		PClearValues:    clearValues,
	}, vk.SubpassContentsInline)

	vk.CmdSetViewport(cmd, 0, 1, []vk.Viewport{{
		Width:    float32(app.width),
		Height:   float32(app.height),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}})

	vk.CmdSetScissor(cmd, 0, 1, []vk.Rect2D{{
		Offset: vk.Offset2D{X: 0, Y: 0},
		Extent: vk.Extent2D{Width: app.width, Height: app.height},
	}})

	// FIRST DRAW THE SKYBOX
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.skyboxPipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	// Bind skybox buffers
	vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{app.skyboxVertexBuffer}, []vk.DeviceSize{0})
	vk.CmdBindIndexBuffer(cmd, app.skyboxIndexBuffer, 0, vk.IndexTypeUint32)

	// Draw skybox
	vk.CmdDrawIndexed(cmd, app.skyboxIndexCount, 1, 0, 0, 0)

	// THEN DRAW THE TERRAIN (as before)
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.pipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	app.TerrainManager.RenderAllocatedChunks(cmd)

	vk.CmdEndRenderPass(cmd)
	ret = vk.EndCommandBuffer(cmd)
	OrPanic(as.NewError(ret))
}
