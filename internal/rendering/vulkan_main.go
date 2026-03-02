// Package renderer for the internal renderings of the svs application
package rendering

import (
	"fmt"
	"log"
	"math"
	"time"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/internal/camera"
	"github.com/cs-wwu/srg-synvis/internal/models"
	as "github.com/vulkan-go/asche"

	// as "github.com/vulkan-go/asche"
	"github.com/go-gl/glfw/v3.3/glfw"
	dvec3 "github.com/ungerik/go3d/float64/vec3"
	"github.com/elokore/glfw/v3.4/glfw"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file is for initializing and handling the main vulkan necessities

func NewApplication(cfg models.AppConfig) *Application {
	// Fixed typo: camerCfg -> cameraConfig
	cameraConfig := camera.CameraConfig{
		StartPos:    dvec3.T{float64(cfg.CameraStartPos[0]), float64(cfg.CameraStartPos[1]), float64(cfg.CameraStartPos[2])},
		StartDir:    dvec3.T{float64(cfg.CameraStartDir[0]), float64(cfg.CameraStartDir[1]), float64(cfg.CameraStartDir[2])},
		EnableGPS:   cfg.UseGPS,
		EnableHat:   cfg.UseHat,
		DecimalYear: time.Now(),
	}
	cam := camera.NewCamera(cameraConfig)

	// Enable hardware if needed (moved here for cleaner initialization)
	if cfg.UseHat {
		if err := cam.EnableSensors("./internal/camera/hat.py"); err != nil {
			log.Printf("Warning: Failed to enable sensors: %v", err)
		}
	}
	app := &Application{
		camera:            cam,
		Gps:               cfg.UseGPS,
		Hat:               cfg.UseHat,
		cfg:               cfg,
		statsTracker:      NewStatsTracker(1000, 5*time.Second),
		lastAirspaceCheck: time.Now(),
	}

	if cfg.BenchmarkMode {
		app.statsTracker.Enable()
	}

	return app
}

func (app *Application) VulkanContextCleanup() error {
	fmt.Println()

	// Wait for device to be idle before cleanup
	dev := app.Context().Device()
	vk.DeviceWaitIdle(dev)

	// Stop background workers before destroying resources
	app.StreamManager.Stop()
	app.StreamManager.Destroy()

	vk.DestroyDescriptorPool(dev, app.descPool, nil)
	vk.DestroyPipeline(dev, app.pipeline, nil)
	vk.DestroyPipeline(dev, app.airspacePipeline, nil)
	vk.DestroyPipeline(dev, app.airspaceStencilPipeline, nil)

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
		if app.textures[i] != nil {
			app.textures[i].Destroy(dev)
			app.textures[i] = nil
		}
	}

	if app.depth != nil {
		app.depth.Destroy(dev)
		app.depth = nil
	}

	vk.DeviceWaitIdle(dev)

	return nil
}

func (app *Application) NextFrame() {
	app.statsTracker.StartFrame()

	app.frameCount++

	// Update the camera and UBO
	app.ubo.Time = float32(math.Mod(glfw.GetTime(), 3600.0))
	app.camera.ProcessInput(app.WindowHandle)
	app.updateUBO()

	// Set camera position in UBO for terrain height warnings
	cameraPos := app.camera.Position()
	app.ubo.CameraPos = lin.Vec3{float32(cameraPos[0]), float32(cameraPos[1]), float32(cameraPos[2])}

	// Update the camera position in the stream manager (non-blocking)
	// This will trigger background chunk loading/unloading
  cameraPosLin := lin.Vec3{float32(cameraPos[0]), float32(cameraPos[1]), float32(cameraPos[2])}
	app.StreamManager.UpdateCameraPosition(cameraPos)
	app.RoadStreamer.UpdatePos(app.camera.Position())
	app.ObstructionStreamer.UpdatePos(app.camera.Position())
	app.AirspaceStreamer.UpdatePos(app.camera.Position())

	// Check current airspace periodically
	if time.Since(app.lastAirspaceCheck) > 100*time.Millisecond {
		app.UpdateCurrentAirspace()
		app.lastAirspaceCheck = time.Now()
	}

	app.camera.ProcessInput(app.WindowHandle)

	// Update system stats periodically
	app.statsTracker.UpdateSystemStats(
		app.cfg.DebugMode,                 // validation layers
		app.Context().PresentModeString(), // or get from present mode
		3,                                 // frame lag
	)

	// Report if needed
	if app.statsTracker.ShouldReport() {
		app.statsTracker.Report()
	}
}

// called every frame by asche
// note : we use a ubo mapped to keep track of memory which saves some steps here,
// but goes against as.createBuffer
func (app *Application) VulkanContextInvalidate(imageIdx int) error {
	app.statsTracker.StartRender()

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

	app.statsTracker.EndRender()
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

	// Draw the plane
	if app.camera.MapView() {
		vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.planePipeline)
		vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
			0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)
		vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{app.planeVertexBuffer}, []vk.DeviceSize{0})
		vk.CmdBindIndexBuffer(cmd, app.planeIndexBuffer, 0, vk.IndexTypeUint32)
		vk.CmdDrawIndexed(cmd, app.planeIndexCount, 1, 0, 0, 0)
	}

	// THEN DRAW THE TERRAIN (as before)
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.pipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	chunks := app.StreamManager.GetRenderableChunks()
	vertexCount := 0
	triangleCount := 0

	for _, chunk := range chunks {
		vertexCount += chunk.IndexCount // approximate
		triangleCount += chunk.IndexCount / 3
	}

	app.statsTracker.UpdateRenderingStats(
		vertexCount,
		triangleCount,
		len(chunks), // draw calls
		len(chunks), // chunk count
	)

	app.StreamManager.RenderChunks(cmd)

	// Draw roads
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.roadPipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	app.RoadStreamer.Lock()
	for _, roadSquare := range app.RoadStreamer.Chunks {
		if roadSquare != nil {
			vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{roadSquare.vertices.Buffer}, []vk.DeviceSize{0})
			vk.CmdBindIndexBuffer(cmd, roadSquare.indices.Buffer, 0, vk.IndexTypeUint32)
			vk.CmdDrawIndexed(cmd, roadSquare.indexCount, 1, 0, 0, 0)
		}
	}
	app.RoadStreamer.Unlock()

	// Draw Obstructions
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.obstructionPipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	app.ObstructionStreamer.Lock()
	for _, obstructionSquare := range app.ObstructionStreamer.Chunks {
		if obstructionSquare != nil {
			vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{obstructionSquare.vertices.Buffer}, []vk.DeviceSize{0})
			vk.CmdBindIndexBuffer(cmd, obstructionSquare.indices.Buffer, 0, vk.IndexTypeUint32)
			vk.CmdDrawIndexed(cmd, obstructionSquare.indexCount, 1, 0, 0, 0)
		}
	}
	app.ObstructionStreamer.Unlock()

	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.airspacePipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout, 0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	app.AirspaceStreamer.Lock()
	for _, airspace := range app.AirspaceStreamer.Chunks {
		if airspace != nil {
			vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{airspace.vertices.Buffer}, []vk.DeviceSize{0})
			vk.CmdBindIndexBuffer(cmd, airspace.indices.Buffer, 0, vk.IndexTypeUint32)
			vk.CmdDrawIndexed(cmd, airspace.indexCount, 1, 0, 0, 0)
		}
	}
	app.AirspaceStreamer.Unlock()

	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.airspaceStencilPipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout, 0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	app.AirspaceStreamer.Lock()
	for _, airspace := range app.AirspaceStreamer.Chunks {
		if airspace != nil {
			vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{airspace.vertices.Buffer}, []vk.DeviceSize{0})
			vk.CmdBindIndexBuffer(cmd, airspace.indices.Buffer, 0, vk.IndexTypeUint32)
			vk.CmdDrawIndexed(cmd, airspace.indexCount, 1, 0, 0, 0)
		}
	}
	app.AirspaceStreamer.Unlock()

	vk.CmdEndRenderPass(cmd)
	ret = vk.EndCommandBuffer(cmd)
	OrPanic(as.NewError(ret))
}
