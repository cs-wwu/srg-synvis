// Package renderer for the internal renderings of the svs application
package rendering

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"time"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/internal/camera"
	"github.com/cs-wwu/srg-synvis/internal/models"
	as "github.com/vulkan-go/asche"

	// as "github.com/vulkan-go/asche"
	"github.com/elokore/glfw/v3.4/glfw"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file is for initializing and handling the main vulkan necessities

func NewApplication(cfg models.AppConfig) *Application {
	// Fixed typo: camerCfg -> cameraConfig
	cameraConfig := camera.CameraConfig{
		StartPos:    cfg.CameraStartPos,
		StartDir:    cfg.CameraStartDir,
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
		camera:       cam,
		Gps:          cfg.UseGPS,
		Hat:          cfg.UseHat,
		cfg:          cfg,
		statsTracker: NewStatsTracker(1000, 5*time.Second),
	}

	if cfg.BenchmarkMode {
		app.statsTracker.Enable()
	}

	return app
}

func (app *Application) VulkanMode() as.VulkanMode {
	if app.cfg.CaptureOnly {
		return as.VulkanGraphics
	} else {
		return as.DefaultVulkanMode // graphics + compute + present
	}
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
	if !app.cfg.CaptureOnly {
		app.ubo.Time = float32(math.Mod(glfw.GetTime(), 3600.0))
		app.camera.ProcessInput(app.WindowHandle)
	}
	app.updateUBO()

	// Set camera position in UBO for terrain height warnings
	cameraPos := app.camera.Position()
	app.ubo.CameraPos = cameraPos

	// Update the camera position in the stream manager (non-blocking)
	// This will trigger background chunk loading/unloading
	app.StreamManager.UpdateCameraPosition(cameraPos)
	app.RoadStreamer.UpdatePos(app.camera.Position())
	app.ObstructionStreamer.UpdatePos(app.camera.Position())

	if !app.cfg.CaptureOnly {
		app.camera.ProcessInput(app.WindowHandle)
	}
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

func (app *Application) RenderCapture() {
	app.NextFrame()

	app.makeOneshotPool()

	app.oneshotCmd(func(cmd vk.CommandBuffer) {
		barrier := vk.ImageMemoryBarrier{
			SType:               vk.StructureTypeImageMemoryBarrier,
			SrcAccessMask:       vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
			DstAccessMask:       vk.AccessFlags(vk.AccessTransferReadBit),
			OldLayout:           vk.ImageLayoutUndefined,
			NewLayout:           vk.ImageLayoutTransferSrcOptimal,
			SrcQueueFamilyIndex: vk.QueueFamilyIgnored,
			DstQueueFamilyIndex: vk.QueueFamilyIgnored,
			Image:               app.oneshotImage,
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
				BaseMipLevel:   0,
				LevelCount:     1,
				BaseArrayLayer: 0,
				LayerCount:     1,
			},
		}
		vk.CmdPipelineBarrier(
			cmd,
			vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
			vk.PipelineStageFlags(vk.PipelineStageTransferBit),
			0,
			0, nil,
			0, nil,
			1, []vk.ImageMemoryBarrier{barrier},
		)
	})

	imgBytes := app.cfg.CaptureWidth * app.cfg.CaptureHeight * 4
	dev := app.Context().Device()

	stagingBuf, stagingMem, err := app.createBuffer(
		vk.BufferUsageFlags(vk.BufferUsageTransferDstBit),
		vk.MemoryPropertyFlags(vk.MemoryPropertyHostVisibleBit|vk.MemoryPropertyHostCoherentBit),
		vk.DeviceSize(imgBytes),
	)
	if err != nil {
		log.Fatalf("failed to create staging buffer: %v", err)
	}
	defer func() {
		vk.DestroyBuffer(dev, stagingBuf, nil)
		vk.FreeMemory(dev, stagingMem, nil)
	}()

	app.oneshotCmd(func(cmd vk.CommandBuffer) {
		region := vk.BufferImageCopy{
			BufferOffset:      0,
			BufferRowLength:   0, // 0 = tightly packed
			BufferImageHeight: 0,
			ImageSubresource: vk.ImageSubresourceLayers{
				AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
				MipLevel:       0,
				BaseArrayLayer: 0,
				LayerCount:     1,
			},
			ImageOffset: vk.Offset3D{X: 0, Y: 0, Z: 0},
			ImageExtent: vk.Extent3D{Width: app.cfg.CaptureWidth, Height: app.cfg.CaptureHeight, Depth: 1},
		}
		vk.CmdCopyImageToBuffer(cmd, app.oneshotImage, vk.ImageLayoutTransferSrcOptimal, stagingBuf, 1, []vk.BufferImageCopy{region})
	})
	if err != nil {
		log.Fatalf("failed to copy image to buffer: %v", err)
	}

	var data unsafe.Pointer
	if res := vk.MapMemory(dev, stagingMem, 0, vk.DeviceSize(imgBytes), 0, &data); res != vk.Success {
		log.Fatalf("failed to map staging buffer memory: %v", as.NewError(res))
	}
	defer vk.UnmapMemory(dev, stagingMem)

	// Vulkan default is RGBA8. image/png expects RGBA premultiplied not required;
	// compose an *image.RGBA and copy rows.
	// Build a Go slice view over the mapped memory:
	raw := unsafe.Slice((*byte)(data), imgBytes)

	img := image.NewRGBA(image.Rect(0, 0, int(app.cfg.CaptureWidth), int(app.cfg.CaptureHeight)))
	rowBytes := int(app.cfg.CaptureWidth) * 4
	for y := 0; y < int(app.cfg.CaptureHeight); y++ {
		src := raw[y*rowBytes : (y+1)*rowBytes]
		dst := img.Pix[y*img.Stride : y*img.Stride+rowBytes]
		copy(dst, src)
	}

	f, err := os.Create("out.png")
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		log.Fatalf("failed to encode png: %v", err)
	}
}

func (app *Application) createBuffer(
	usage vk.BufferUsageFlags,
	props vk.MemoryPropertyFlags,
	size vk.DeviceSize,
) (vk.Buffer, vk.DeviceMemory, error) {
	dev := app.Context().Device()

	bufInfo := vk.BufferCreateInfo{
		SType:       vk.StructureTypeBufferCreateInfo,
		Size:        size,
		Usage:       usage,
		SharingMode: vk.SharingModeExclusive,
	}
	var buf vk.Buffer
	if res := vk.CreateBuffer(dev, &bufInfo, nil, &buf); res != vk.Success {
		return nil, nil, vk.Error(res)
	}

	var memReqs vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(dev, buf, &memReqs)
	memReqs.Deref()
	memProps := app.Context().Platform().MemoryProperties()
	memTypeIndex, ok := as.FindRequiredMemoryTypeFallback(memProps,
		vk.MemoryPropertyFlagBits(memReqs.MemoryTypeBits), vk.MemoryPropertyFlagBits(props))
	if !ok {
		vk.DestroyBuffer(dev, buf, nil)
		log.Fatalf("failed to find suitable memory type for buffer (required: %v)", props)
		return nil, nil, vk.Error(vk.ErrorValidationFailed)
	}

	alloc := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memTypeIndex,
	}
	var mem vk.DeviceMemory
	if res := vk.AllocateMemory(dev, &alloc, nil, &mem); res != vk.Success {
		vk.DestroyBuffer(dev, buf, nil)
		return nil, nil, vk.Error(res)
	}
	if res := vk.BindBufferMemory(dev, buf, mem, 0); res != vk.Success {
		vk.FreeMemory(dev, mem, nil)
		vk.DestroyBuffer(dev, buf, nil)
		return nil, nil, vk.Error(res)
	}
	return buf, mem, nil
}

func (app *Application) makeOneshotPool() {
	cmdPoolInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		Flags:            vk.CommandPoolCreateFlags(vk.CommandPoolCreateTransientBit | vk.CommandPoolCreateResetCommandBufferBit),
		QueueFamilyIndex: app.Context().Platform().GraphicsQueueFamilyIndex(),
	}
	var pool vk.CommandPool
	if res := vk.CreateCommandPool(app.Context().Device(), &cmdPoolInfo, nil, &pool); res != vk.Success {
		log.Fatalf("failed to create oneshot command pool: %v", as.NewError(res))
	}

	app.oneshotCommandPool = pool
}

func (app *Application) oneshotCmd(body func(cmd vk.CommandBuffer)) {
	allocInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        app.oneshotCommandPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}
	var cmds = make([]vk.CommandBuffer, 1)
	if res := vk.AllocateCommandBuffers(app.Context().Device(), &allocInfo, cmds); res != vk.Success {
		log.Fatalln("failed to allocate oneshot command buffer:", as.NewError(res))
	}
	cmd := cmds[0]
	begin := vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit),
	}
	vk.BeginCommandBuffer(cmd, &begin)
	body(cmd)
	if res := vk.EndCommandBuffer(cmd); res != vk.Success {
		log.Fatalln("failed to end command buffer:", as.NewError(res))
	}
	submit := vk.SubmitInfo{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    []vk.CommandBuffer{cmd},
	}
	queue := app.Context().Platform().GraphicsQueue()
	if res := vk.QueueSubmit(queue, 1, []vk.SubmitInfo{submit}, vk.NullFence); res != vk.Success {
		log.Fatalln("failed to submit oneshot command buffer:", as.NewError(res))
	}
	vk.QueueWaitIdle(queue)
	vk.FreeCommandBuffers(app.Context().Device(), app.oneshotCommandPool, 1, []vk.CommandBuffer{cmd})
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

	vk.CmdEndRenderPass(cmd)
	ret = vk.EndCommandBuffer(cmd)
	OrPanic(as.NewError(ret))
}
