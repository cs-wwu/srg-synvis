// Package renderer for the internal renderings of the svs application
package renderer

import (
	"fmt"
	"log"
	"unsafe"

	as "github.com/cs-wwu/srg-synvis/asche"

	// as "github.com/vulkan-go/asche"
	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file is for initializing and handling the main vulkan necessities

func NewApplication() *Application {
	camera := NewCamera()
	fpsCounter := NewFPSCounter()
	// frustum := &Frustum{}
	app := &Application{
		camera:     camera,
		fpsCounter: fpsCounter,
		// frustum: frustum,
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
func (fc *FPSCounter) Update() {
	currentTime := glfw.GetTime()
	fc.frames++

	// Update FPS every second
	if currentTime-fc.fpsTime >= 1.0 {
		fc.fps = float64(fc.frames) / (currentTime - fc.fpsTime)
		fc.frames = 0
		fc.fpsTime = currentTime

		// Clear line and print new FPS
		fmt.Printf("\r\033[K") // Clear current line
		fmt.Printf("FPS: %.1f | Frame Time: %.2fms",
			fc.fps,
			1000.0/fc.fps,
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
	vk.DestroyPipelineCache(dev, app.pipelineCache, nil)
	vk.DestroyRenderPass(dev, app.renderPass, nil)
	vk.DestroyPipelineLayout(dev, app.pipelineLayout, nil)
	vk.DestroyDescriptorSetLayout(dev, app.descLayout, nil)

	for i := 0; i < len(app.textures); i++ {
		app.textures[i].Destroy(dev)
	}

	app.depth.Destroy(dev)
	return nil
}

func (app *Application) NextFrame() {
	app.fpsCounter.Update()

	// Process Input and update the camera
	app.camera.ProcessInput(app.WindowHandle)
	app.ubo.View = app.camera.GetViewMatrix()

	//Update the view frustum and visibleChunks
	// app.frustum.Update(&app.ubo.View, &app.ubo.Projection)
	// app.updateVisibleChunks()

	// Wait for the device to be idle before updating visible chunks
	// This ensures we're not modifying data while the GPU is using it
	// if len(app.ChunkManager.visibleChunks) > 0 {
	// 	vk.DeviceWaitIdle(app.Context().Device())
	// 	app.ChunkManager.updateDrawCommands()
	// 	app.ChunkManager.updateDrawCommandBuffer()
	// }
	vk.DeviceWaitIdle(app.Context().Device())
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

	return nil
}

func (app *Application) updateCommandBuffer(res *as.SwapchainImageResources, cmd vk.CommandBuffer) {
	// Begin command buffer
	ret := vk.BeginCommandBuffer(cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit),
	})
	OrPanic(as.NewError(ret))

	// Clear values for rendering
	clearValues := make([]vk.ClearValue, 2)
	clearValues[1].SetDepthStencil(1, 0)
	clearValues[0].SetColor([]float32{0.2, 0.2, 0.2, 0.2})

	// Begin render pass
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

	// Bind pipeline and descriptor sets
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.pipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)

	// Set viewport and scissor
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

	// Bind vertex and index buffers
	vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{app.TerrainManager.vertexBuffer}, []vk.DeviceSize{0})
	vk.CmdBindIndexBuffer(cmd, app.TerrainManager.indexBuffer, 0, vk.IndexTypeUint32)

	// Draw all chunks using indirect drawing
	vk.CmdDrawIndexedIndirect(cmd,
		app.TerrainManager.drawCommandBuffer, // renamed from ChunkManager
		0,                                    // offset
		1,                                    // drawCount (just 1 command now)
		uint32(unsafe.Sizeof(DrawCommand{})), // stride
	)
	// End render pass
	vk.CmdEndRenderPass(cmd)

	// End command buffer
	ret = vk.EndCommandBuffer(cmd)
	OrPanic(as.NewError(ret))
}
