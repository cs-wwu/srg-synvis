package rendering

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/chunk"
	"github.com/cs-wwu/srg-synvis/internal/streaming"
	as "github.com/vulkan-go/asche"

	"github.com/udhos/gwob"

	"github.com/elokore/cimgui-go-vulkan/backend"
	glfwvulkanbackend "github.com/elokore/cimgui-go-vulkan/backend/glfwvulkan-backend"

	// as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file is for vulkan initialization

func (app *Application) IsUncappedFPS() bool {
	return app.Cfg.UncapFPS
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
	validationLayers := []string{}
	if app.Cfg.DebugMode {
		validationLayers = append(validationLayers, "VK_LAYER_KHRONOS_validation")
	} else {
		log.Println("vulkan: debug mode is off, not using validation layers")
	}
	return validationLayers
}

func (app *Application) VulkanDeviceExtensions() []string {
	return []string{
		"VK_KHR_swapchain",
	}
}

func (app *Application) VulkanSwapchainDimensions() *as.SwapchainDimensions {
	width, _ := strconv.ParseUint(os.Getenv("SYVIS_WINDOW_WIDTH"), 10, 32)
	if width <= 0 || width > 10000 {
		width = DEFAULT_WINDOW_WIDTH
	}

	height, _ := strconv.ParseUint(os.Getenv("SYVIS_WINDOW_HEIGHT"), 10, 32)
	if height <= 0 || height > 10000 {
		height = DEFAULT_WINDOW_HEIGHT
	}

	return &as.SwapchainDimensions{
		Width:  uint32(width),
		Height: uint32(height),
		Format: vk.FormatB8g8r8a8Unorm,
	}
}

func (app *Application) SetSwapchainDimensions(dim *as.SwapchainDimensions) {
	app.width = dim.Width
	app.height = dim.Height
}

func (app *Application) VulkanInstanceExtensions() []string {
	extensions := app.WindowHandle.GetRequiredInstanceExtensions()
	if app.Cfg.DebugMode {
		extensions = append(extensions, "VK_EXT_debug_report")
	}
	return extensions
}

func (app *Application) VulkanPresentMode() vk.PresentMode {
	if app.Cfg.UncapFPS {
		return vk.PresentModeImmediate
	}
	return vk.PresentModeMailbox
}

func (app *Application) VulkanDeviceFeatures() []vk.PhysicalDeviceFeatures {
	return []vk.PhysicalDeviceFeatures{
		{
			//  MultiDrawIndirect: vk.True, // Multiple Indirect Draw Calls
		},
	}
}

func (app *Application) VulkanCommandPoolCreateInfo(queueFamilyIndex uint32) vk.CommandPoolCreateInfo {
	return vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: queueFamilyIndex,
		Flags: vk.CommandPoolCreateFlags(
			vk.CommandPoolCreateResetCommandBufferBit |
				vk.CommandPoolCreateTransientBit, // Transient Command Buffers?
		),
	}
}

// Initialize the Uniform Buffer Object
func (app *Application) initUBO() {
	app.updateUBO()

	app.ubo.Model.Identity()
	temp := lin.Vec3{0, 0.1, 0.1}
	app.ubo.LightDir.Norm(&temp)
	app.ubo.RenderDistance = app.Cfg.RenderDistance
	log.Printf("vulkan: render distance: %f", app.Cfg.RenderDistance)

	// Fix the logic to match shader expectations
	if app.Cfg.DisableTerrainWarnings {
		app.ubo.DisableWarnings = 1.0 // 1.0 = disabled
	} else {
		app.ubo.DisableWarnings = 0.0 // 0.0 = enabled
	}

	if app.Cfg.DisableFog {
		app.ubo.DisableFog = 1.0 // 1.0 = disabled
		log.Printf("Disabling fog\n")
	} else {
		log.Printf("Enabling fog\n")
		app.ubo.DisableFog = 0.0 // 0.0 = enabled
	}
}

func (app *Application) updateUBO() {
	aspectRatio := float32(app.width) / float32(app.height)

	if app.Camera.MapView() {
		ortho := lin.Mat4x4{}

		// TODO: Zoom? Currently, 1000x1000 points are visible in the map view.
		view := float32(1000.0)
		ortho.Ortho(view*aspectRatio, -view*aspectRatio, view, -view, 1000.0, -1000.0)

		// Everything is negated here, so we need some correction.
		// TODO(neil): Fix the coordinate system to not negate everything.
		correction := lin.Mat4x4{
			{1.0, 0.0, 0.0, 0.0},
			{0.0, 1.0, 0.0, 0.0},
			{0.0, 0.0, -1.0, 0.0},
			{0.0, 0.0, 0.0, 1.0},
		}
		app.ubo.Projection.Mult(&correction, &ortho)
		app.ubo.Projection[1][1] *= -1

		app.Camera.PlaneMatrix(&app.ubo.PlaneDirection)
	} else {
		app.ubo.Projection.Perspective(lin.DegreesToRadians(app.Cfg.Fov), aspectRatio, 1.0, app.Cfg.RenderDistance)
		app.ubo.Projection[1][1] *= -1
	}
	app.ubo.View = app.Camera.ViewMatrix()
}

// Prepare depth buffers
func (app *Application) prepareDepth() {
	dev := app.Context().Device()
	app.depth = &Depth{
		format: DepthFormat,
	}
	ret := vk.CreateImage(dev, &vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    DepthFormat,
		Extent: vk.Extent3D{
			Width:  app.width,
			Height: app.height,
			Depth:  1,
		},
		MipLevels:   1,
		ArrayLayers: 1,
		Samples:     vk.SampleCount1Bit,
		Tiling:      vk.ImageTilingOptimal,
		Usage:       vk.ImageUsageFlags(vk.ImageUsageDepthStencilAttachmentBit),
	}, nil, &app.depth.image)
	OrPanic(as.NewError(ret))

	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(dev, app.depth.image, &memReqs)
	memReqs.Deref()

	memProps := app.Context().Platform().MemoryProperties()
	memTypeIndex, _ := as.FindRequiredMemoryTypeFallback(memProps,
		vk.MemoryPropertyFlagBits(memReqs.MemoryTypeBits), vk.MemoryPropertyDeviceLocalBit)
	app.depth.memAlloc = &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memTypeIndex,
	}

	var mem vk.DeviceMemory
	ret = vk.AllocateMemory(dev, app.depth.memAlloc, nil, &mem)
	OrPanic(as.NewError(ret))
	app.depth.mem = mem

	ret = vk.BindImageMemory(dev, app.depth.image, app.depth.mem, 0)
	OrPanic(as.NewError(ret))

	var view vk.ImageView
	ret = vk.CreateImageView(dev, &vk.ImageViewCreateInfo{
		SType:  vk.StructureTypeImageViewCreateInfo,
		Format: DepthFormat,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectDepthBit),
			LevelCount: 1,
			LayerCount: 1,
		},
		ViewType: vk.ImageViewType2d,
		Image:    app.depth.image,
	}, nil, &view)
	OrPanic(as.NewError(ret))
	app.depth.view = view
}

func (app *Application) prepareTextureImage(path string, tiling vk.ImageTiling,
	usage vk.ImageUsageFlagBits, memoryProps vk.MemoryPropertyFlagBits) *Texture {

	dev := app.Context().Device()
	_, width, height, _, err := loadTextureData(path)
	if err != nil {
		OrPanic(err)
	}

	tex := &Texture{
		texWidth:    int32(width),
		texHeight:   int32(height),
		imageLayout: vk.ImageLayoutGeneral,
	}

	var image vk.Image
	imageCreateInfo := vk.ImageCreateInfo{
		SType:         vk.StructureTypeImageCreateInfo,
		ImageType:     vk.ImageType2d,
		Format:        TextureFormat,
		Extent:        vk.Extent3D{Width: uint32(width), Height: uint32(height), Depth: 1},
		MipLevels:     1,
		ArrayLayers:   1,
		Samples:       1,
		Tiling:        tiling,
		Usage:         vk.ImageUsageFlags(usage),
		InitialLayout: vk.ImageLayoutPreinitialized,
	}

	ret := vk.CreateImage(dev, &imageCreateInfo, nil, &image)
	OrPanic(as.NewError(ret))
	tex.image = image

	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(dev, tex.image, &memReqs)
	memReqs.Deref()

	memProps := app.Context().Platform().MemoryProperties()
	memTypeIndex, _ := as.FindRequiredMemoryTypeFallback(memProps,
		vk.MemoryPropertyFlagBits(memReqs.MemoryTypeBits), memoryProps)
	tex.memAlloc = &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memTypeIndex,
	}
	var mem vk.DeviceMemory
	ret = vk.AllocateMemory(dev, tex.memAlloc, nil, &mem)
	OrPanic(as.NewError(ret))
	tex.mem = mem
	ret = vk.BindImageMemory(dev, tex.image, tex.mem, 0)
	OrPanic(as.NewError(ret))

	hostVisible := memoryProps&vk.MemoryPropertyHostVisibleBit != 0
	if hostVisible {
		var layout vk.SubresourceLayout
		vk.GetImageSubresourceLayout(dev, tex.image, &vk.ImageSubresource{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
		}, &layout)
		layout.Deref()

		data, width, height, sourceStride, err := loadTextureData(path)
		OrPanic(err)
		if len(data) > 0 {
			var pData unsafe.Pointer
			ret = vk.MapMemory(dev, tex.mem, 0, layout.Size, 0, &pData)
			if ret != vk.Success {
				log.Printf("vulkan warning: failed to map device memory for data (len=%d)", len(data))
				return tex
			}

			// Use row-by-row copy to respect GPU memory layout
			copyTextureData(pData, data, width, height, sourceStride, int(layout.RowPitch))

			vk.UnmapMemory(dev, tex.mem)
		}
	}
	return tex
}

func (app *Application) setImageLayout(image vk.Image, aspectMask vk.ImageAspectFlagBits,
	oldImageLayout, newImageLayout vk.ImageLayout,
	srcAccessMask vk.AccessFlagBits,
	srcStages, dstStages vk.PipelineStageFlagBits) {

	cmd := app.Context().CommandBuffer()
	if cmd == nil {
		OrPanic(errors.New("vulkan: command buffer not initialized"))
	}

	imageMemoryBarrier := vk.ImageMemoryBarrier{
		SType:         vk.StructureTypeImageMemoryBarrier,
		SrcAccessMask: vk.AccessFlags(srcAccessMask),
		DstAccessMask: 0,
		OldLayout:     oldImageLayout,
		NewLayout:     newImageLayout,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(aspectMask),
			LayerCount: 1,
			LevelCount: 1,
		},
		Image: image,
	}
	switch newImageLayout {
	case vk.ImageLayoutTransferDstOptimal:
		imageMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessTransferWriteBit)
	case vk.ImageLayoutColorAttachmentOptimal:
		imageMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessColorAttachmentWriteBit)
	case vk.ImageLayoutDepthStencilAttachmentOptimal:
		imageMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessDepthStencilAttachmentWriteBit)
	case vk.ImageLayoutShaderReadOnlyOptimal:
		imageMemoryBarrier.DstAccessMask =
			vk.AccessFlags(vk.AccessShaderReadBit) | vk.AccessFlags(vk.AccessInputAttachmentReadBit)
	case vk.ImageLayoutTransferSrcOptimal:
		imageMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessTransferReadBit)
	case vk.ImageLayoutPresentSrc:
		imageMemoryBarrier.DstAccessMask = vk.AccessFlags(vk.AccessMemoryReadBit)
	default:
		imageMemoryBarrier.DstAccessMask = 0
	}

	vk.CmdPipelineBarrier(cmd,
		vk.PipelineStageFlags(srcStages), vk.PipelineStageFlags(dstStages),
		/* flags */ 0,
		/* memory barrier count */ 0, nil,
		/* buffer barrier count */ 0, nil,
		/* image barrier count */ 1, []vk.ImageMemoryBarrier{imageMemoryBarrier})
}

func (app *Application) getGPUName(gpu vk.PhysicalDevice) string {
	var devProperties vk.PhysicalDeviceProperties
	vk.GetPhysicalDeviceProperties(gpu, &devProperties)
	devProperties.Deref()

	return vk.ToString(devProperties.DeviceName[:])
}

func (app *Application) prepareTextures() {
	dev := app.Context().Device()
	var props vk.FormatProperties
	gpu := app.Context().Platform().PhysicalDevice()
	vk.GetPhysicalDeviceFormatProperties(gpu, TextureFormat, &props)
	props.Deref()

	fmt.Printf("GPU: %v\n", app.getGPUName(gpu))
	prepareTex := func(path string) *Texture {
		var tex *Texture

		if (props.LinearTilingFeatures&vk.FormatFeatureFlags(vk.FormatFeatureSampledImageBit) != 0) &&
			!app.useStagingBuffers {
			// -> device can texture using linear textures

			tex = app.prepareTextureImage(path, vk.ImageTilingLinear, vk.ImageUsageSampledBit,
				vk.MemoryPropertyHostVisibleBit|vk.MemoryPropertyHostCoherentBit)

			// Nothing in the pipeline needs to be complete to start, and don't allow fragment
			// shader to run until layout transition completes
			app.setImageLayout(tex.image, vk.ImageAspectColorBit,
				vk.ImageLayoutPreinitialized, tex.imageLayout,
				vk.AccessHostWriteBit,
				vk.PipelineStageHostBit, vk.PipelineStageFragmentShaderBit)

		} else if props.OptimalTilingFeatures&vk.FormatFeatureFlags(vk.FormatFeatureSampledImageBit) != 0 {
			//  Must use staging buffer to copy linear texture to optimized
			log.Println("vulkan warn: using staging buffers")

			staging := app.prepareTextureImage(path, vk.ImageTilingLinear, vk.ImageUsageTransferSrcBit,
				vk.MemoryPropertyHostVisibleBit|vk.MemoryPropertyHostCoherentBit)
			tex = app.prepareTextureImage(path, vk.ImageTilingOptimal,
				vk.ImageUsageTransferDstBit|vk.ImageUsageSampledBit, vk.MemoryPropertyDeviceLocalBit)

			app.setImageLayout(staging.image, vk.ImageAspectColorBit,
				vk.ImageLayoutPreinitialized, vk.ImageLayoutTransferSrcOptimal,
				vk.AccessHostWriteBit,
				vk.PipelineStageHostBit, vk.PipelineStageTransferBit)

			app.setImageLayout(tex.image, vk.ImageAspectColorBit,
				vk.ImageLayoutPreinitialized, vk.ImageLayoutTransferDstOptimal,
				vk.AccessHostWriteBit,
				vk.PipelineStageTopOfPipeBit, vk.PipelineStageTransferBit)

			cmd := app.Context().CommandBuffer()
			if cmd == nil {
				OrPanic(errors.New("vulkan: command buffer not initialized"))
			}
			vk.CmdCopyImage(cmd, staging.image, vk.ImageLayoutTransferSrcOptimal,
				tex.image, vk.ImageLayoutTransferDstOptimal,
				1, []vk.ImageCopy{{
					SrcSubresource: vk.ImageSubresourceLayers{
						AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
						LayerCount: 1,
					},
					SrcOffset: vk.Offset3D{
						X: 0, Y: 0, Z: 0,
					},
					DstSubresource: vk.ImageSubresourceLayers{
						AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
						LayerCount: 1,
					},
					DstOffset: vk.Offset3D{
						X: 0, Y: 0, Z: 0,
					},
					Extent: vk.Extent3D{
						Width:  uint32(staging.texWidth),
						Height: uint32(staging.texHeight),
						Depth:  1,
					},
				}})
			app.setImageLayout(tex.image, vk.ImageAspectColorBit,
				vk.ImageLayoutTransferDstOptimal, tex.imageLayout,
				vk.AccessTransferWriteBit,
				vk.PipelineStageTransferBit, vk.PipelineStageFragmentShaderBit)
			// cannot destroy until cmd is submitted.. must keep a list somewhere
			// staging.DestroyImage(dev)
		} else {
			OrPanic(errors.New("vulkan: R8G8B8A8_UNORM not supported as texture image format"))
		}

		var sampler vk.Sampler
		ret := vk.CreateSampler(dev, &vk.SamplerCreateInfo{
			SType:                   vk.StructureTypeSamplerCreateInfo,
			MagFilter:               vk.FilterLinear,
			MinFilter:               vk.FilterLinear,
			MipmapMode:              vk.SamplerMipmapModeNearest,
			AddressModeU:            vk.SamplerAddressModeRepeat,
			AddressModeV:            vk.SamplerAddressModeRepeat,
			AddressModeW:            vk.SamplerAddressModeRepeat,
			CompareOp:               vk.CompareOpNever,
			BorderColor:             vk.BorderColorFloatOpaqueWhite,
			UnnormalizedCoordinates: vk.False,
		}, nil, &sampler)
		OrPanic(as.NewError(ret))
		tex.sampler = sampler

		var view vk.ImageView
		ret = vk.CreateImageView(dev, &vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeSamplerCreateInfo,
			Image:    tex.image,
			ViewType: vk.ImageViewType2d,
			Format:   TextureFormat,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleR,
				G: vk.ComponentSwizzleG,
				B: vk.ComponentSwizzleB,
				A: vk.ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
				LevelCount: 1,
				LayerCount: 1,
			},
		}, nil, &view)
		OrPanic(as.NewError(ret))
		tex.view = view

		return tex
	}

	app.textures = make([]*Texture, 0, len(texEnabled))
	for _, texFile := range texEnabled {
		app.textures = append(app.textures, prepareTex(texFile))
	}
}

func (app *Application) buildAirspace(pos spatial.MapCoord) *Airspace {
	airspaceFile := "assets/chunks/airspace/data/" + pos.String() + ".geojson"

	var geojson FeatureCollection
	file, err := FileToBytesChecked(airspaceFile)
	if err != nil {
		log.Printf("vulkan: error loading airspace %s: %v", airspaceFile, err)
		return nil
	}

	err = json.Unmarshal(file, &geojson)
	if err != nil {
		log.Printf("vulkan: error unmarshaling airspace %s: %v", airspaceFile, err)
		return nil
	}

	mapData, err := app.StreamManager.assetManager.LoadMapData(pos)
	if err != nil {
		log.Printf("vulkan: error loading map data for airspace: %v", err)
		return nil
	}

	heightInfo := chunk.NewHeightInfo(mapData.HeightData)
	builder := newAirspaceBuilder(heightInfo)
	instances := make([]AirspaceInstance, len(geojson.Features))

	for i, airspace := range geojson.Features {
		renderAirspace(builder, airspace, airspaceColors[airspace.Properties.Class])
		instances[i] = AirspaceInstance{
			LowerAlt: float32(airspace.Properties.AltitudeFloor),
			UpperAlt: float32(airspace.Properties.AltitudeCeiling),
			Color:    airspaceColors[airspace.Properties.Class],
		}
	}

	vertices := as.CreateBuffer(
		app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		builder.vertexBytes(),
		vk.BufferUsageVertexBufferBit,
	)
	indices := as.CreateBuffer(
		app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		builder.indexBytes(),
		vk.BufferUsageIndexBufferBit,
	)

	return &Airspace{
		vertices:   vertices,
		indices:    indices,
		indexCount: uint32(len(builder.indices)),
		features:   geojson.Features,
	}
}

func (app *Application) prepareUniformBuffers() {
	dev := app.Context().Device()

	dataRaw := app.ubo.Data()

	memProps := app.Context().Platform().MemoryProperties()
	swapchainImageResources := app.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		buf := as.CreateBuffer(dev, memProps, dataRaw, vk.BufferUsageUniformBufferBit)
		res.SetUniformBuffer(buf.Buffer, buf.Memory)
	}
}

func (app *Application) prepareDescriptorLayout() {
	dev := app.Context().Device()
	var descLayout vk.DescriptorSetLayout
	ret := vk.CreateDescriptorSetLayout(dev, &vk.DescriptorSetLayoutCreateInfo{
		SType:        vk.StructureTypeDescriptorSetLayoutCreateInfo,
		BindingCount: 2,
		PBindings: []vk.DescriptorSetLayoutBinding{
			{
				Binding:         0,
				DescriptorType:  vk.DescriptorTypeUniformBuffer,
				DescriptorCount: 1,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit | vk.ShaderStageFragmentBit),
			}, {
				Binding:         1,
				DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
				DescriptorCount: uint32(len(texEnabled)),
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}},
	}, nil, &descLayout)
	OrPanic(as.NewError(ret))
	app.descLayout = descLayout

	var pipelineLayout vk.PipelineLayout
	ret = vk.CreatePipelineLayout(dev, &vk.PipelineLayoutCreateInfo{
		SType:          vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount: 1,
		PSetLayouts:    []vk.DescriptorSetLayout{app.descLayout},
	}, nil, &pipelineLayout)
	OrPanic(as.NewError(ret))
	app.pipelineLayout = pipelineLayout
}

func (app *Application) prepareRenderPass() {
	dev := app.Context().Device()
	var renderPass vk.RenderPass
	ret := vk.CreateRenderPass(dev, &vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: 2,
		PAttachments: []vk.AttachmentDescription{{
			Format:         app.Context().SwapchainDimensions().Format,
			Samples:        vk.SampleCount1Bit,
			LoadOp:         vk.AttachmentLoadOpClear,
			StoreOp:        vk.AttachmentStoreOpStore,
			StencilLoadOp:  vk.AttachmentLoadOpDontCare,
			StencilStoreOp: vk.AttachmentStoreOpDontCare,
			InitialLayout:  vk.ImageLayoutUndefined,
			FinalLayout:    vk.ImageLayoutPresentSrc,
		}, {
			Format:         app.depth.format,
			Samples:        vk.SampleCount1Bit,
			LoadOp:         vk.AttachmentLoadOpClear,
			StoreOp:        vk.AttachmentStoreOpDontCare,
			StencilLoadOp:  vk.AttachmentLoadOpClear,
			StencilStoreOp: vk.AttachmentStoreOpStore,
			InitialLayout:  vk.ImageLayoutUndefined,
			FinalLayout:    vk.ImageLayoutDepthStencilAttachmentOptimal,
		}},
		SubpassCount: 1,
		PSubpasses: []vk.SubpassDescription{{
			PipelineBindPoint:    vk.PipelineBindPointGraphics,
			ColorAttachmentCount: 1,
			PColorAttachments: []vk.AttachmentReference{{
				Attachment: 0,
				Layout:     vk.ImageLayoutColorAttachmentOptimal,
			}},
			PDepthStencilAttachment: &vk.AttachmentReference{
				Attachment: 1,
				Layout:     vk.ImageLayoutDepthStencilAttachmentOptimal,
			},
		}},
	}, nil, &renderPass)
	OrPanic(as.NewError(ret))
	app.renderPass = renderPass
}

func (app *Application) prepareSkyboxCube() {
	// Create cube vertices
	vertices := []float32{
		// Positions
		-1.0, 1.0, -1.0, // Front top left
		-1.0, -1.0, -1.0, // Front bottom left
		1.0, -1.0, -1.0, // Front bottom right
		1.0, 1.0, -1.0, // Front top right
		-1.0, 1.0, 1.0, // Back top left
		-1.0, -1.0, 1.0, // Back bottom left
		1.0, -1.0, 1.0, // Back bottom right
		1.0, 1.0, 1.0, // Back top right
	}

	// Indices for the cube faces
	indices := []uint32{
		// Front face
		0, 1, 2, 0, 2, 3,
		// Right face
		3, 2, 6, 3, 6, 7,
		// Back face
		7, 6, 5, 7, 5, 4,
		// Left face
		4, 5, 1, 4, 1, 0,
		// Top face
		4, 0, 3, 4, 3, 7,
		// Bottom face
		1, 5, 6, 1, 6, 2,
	}

	// Create vertex buffer
	vertexBuffer := as.CreateBuffer(app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		convertFloatSliceToBytes(vertices),
		vk.BufferUsageVertexBufferBit)
	app.skyboxVertexBuffer = vertexBuffer.Buffer
	app.skyboxVertexMemory = vertexBuffer.Memory

	// Create index buffer
	indexBuffer := as.CreateBuffer(app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		convertUint32SliceToBytes(indices),
		vk.BufferUsageIndexBufferBit)
	app.skyboxIndexBuffer = indexBuffer.Buffer
	app.skyboxIndexMemory = indexBuffer.Memory

	app.skyboxIndexCount = uint32(len(indices))
}

func (app *Application) preparePlaneModel() {
	options := &gwob.ObjParserOptions{} // parser options

	obj, err := gwob.NewObjFromFile("assets/models/cessna.obj", options) // parse/load OBJ
	if err != nil {
		log.Fatal(err)
	}

	vertexBuffer := as.CreateBuffer(app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		convertFloatSliceToBytes(obj.Coord),
		vk.BufferUsageVertexBufferBit)
	app.planeVertexBuffer = vertexBuffer.Buffer
	app.planeVertexMemory = vertexBuffer.Memory

	indexBytes := make([]byte, len(obj.Indices)*4)
	for i, index := range obj.Indices {
		binary.LittleEndian.PutUint32(indexBytes[i*4:], uint32(index))
	}

	indexBuffer := as.CreateBuffer(app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		indexBytes,
		vk.BufferUsageIndexBufferBit)
	app.planeIndexBuffer = indexBuffer.Buffer
	app.planeIndexMemory = indexBuffer.Memory

	group := obj.Groups[0]
	app.planeIndexCount = uint32(group.IndexCount)
}

// Helper functions to convert slices to bytes
func convertFloatSliceToBytes(slice []float32) []byte {
	byteLen := len(slice) * 4
	bytes := make([]byte, byteLen)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(v))
	}
	return bytes
}

func convertUint32SliceToBytes(slice []uint32) []byte {
	byteLen := len(slice) * 4
	bytes := make([]byte, byteLen)
	for i, v := range slice {
		binary.LittleEndian.PutUint32(bytes[i*4:], v)
	}
	return bytes
}

func (app *Application) prepareSkyboxPipeline() {
	dev := app.Context().Device()

	// Load skybox shaders
	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/skybox.vert.spv"))
	OrPanic(err)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/skybox.frag.spv"))
	OrPanic(err)

	// Pipeline is similar to main pipeline, with key differences
	pipelineBuilder := vk.GraphicsPipelineCreateInfo{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.pipelineLayout, // Reuse existing layout
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				vk.DynamicStateScissor,
				vk.DynamicStateViewport,
			},
		},

		// Vertex input for skybox cube
		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                         vk.StructureTypePipelineVertexInputStateCreateInfo,
			VertexBindingDescriptionCount: 1,
			PVertexBindingDescriptions: []vk.VertexInputBindingDescription{{
				Binding:   0,
				Stride:    12, // 3 floats * 4 bytes = 12 bytes per vertex
				InputRate: vk.VertexInputRateVertex,
			}},
			VertexAttributeDescriptionCount: 1,
			PVertexAttributeDescriptions: []vk.VertexInputAttributeDescription{{
				Binding:  0,
				Location: 0,
				Format:   vk.FormatR32g32b32Sfloat,
				Offset:   0,
			}},
		},

		// Use triangles for the cube
		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:    vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			Topology: vk.PrimitiveTopologyTriangleList,
		},

		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeFrontBit), // Cull front faces since we're inside the cube
			FrontFace:   vk.FrontFaceClockwise,
			LineWidth:   1.0,
		},

		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask: 0xF,
				BlendEnable:    vk.False,
			}},
		},

		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
			RasterizationSamples: vk.SampleCount1Bit,
		},

		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
		},

		// Key difference: depth test but don't write depth
		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.False, // Don't write to depth buffer
			DepthCompareOp:        vk.CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			StencilTestEnable:     vk.False,
		},

		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}

	// Create the pipeline
	skyboxPipeline := make([]vk.Pipeline, 1)
	ret := vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, []vk.GraphicsPipelineCreateInfo{pipelineBuilder}, nil, skyboxPipeline)
	OrPanic(as.NewError(ret))
	app.skyboxPipeline = skyboxPipeline[0]

	// Clean up shader modules
	vk.DestroyShaderModule(dev, vs, nil)
	vk.DestroyShaderModule(dev, fs, nil)
}

func (app *Application) preparePlanePipeline() {
	dev := app.Context().Device()

	// Load plane shaders
	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/plane.vert.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, vs, nil)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/plane.frag.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, fs, nil)

	// Pipeline is similar to main pipeline, with key differences
	pipelineBuilder := vk.GraphicsPipelineCreateInfo{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.pipelineLayout, // Reuse existing layout
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				vk.DynamicStateScissor,
				vk.DynamicStateViewport,
			},
		},

		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                         vk.StructureTypePipelineVertexInputStateCreateInfo,
			VertexBindingDescriptionCount: 1,
			PVertexBindingDescriptions: []vk.VertexInputBindingDescription{{
				Binding:   0,
				Stride:    32, // 8 floats * 4 bytes = 32 bytes per vertex
				InputRate: vk.VertexInputRateVertex,
			}},
			VertexAttributeDescriptionCount: 3,
			PVertexAttributeDescriptions: []vk.VertexInputAttributeDescription{
				{
					Binding:  0,
					Location: 0,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   0,
				},
				{
					Binding:  0,
					Location: 1,
					Format:   vk.FormatR32g32Sfloat,
					Offset:   4 * 3,
				},
				{
					Binding:  0,
					Location: 2,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   4 * 5,
				},
			},
		},

		// Use triangles for the cube
		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:    vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			Topology: vk.PrimitiveTopologyTriangleList,
		},

		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeBackBit),
			FrontFace:   vk.FrontFaceCounterClockwise,
			LineWidth:   1.0,
		},

		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask: 0xF,
				BlendEnable:    vk.False,
			}},
		},

		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
			RasterizationSamples: vk.SampleCount1Bit,
		},

		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
		},

		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.True,
			DepthCompareOp:        vk.CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			StencilTestEnable:     vk.False,
		},

		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}

	// Create the pipeline
	planePipeline := make([]vk.Pipeline, 1)
	ret := vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, []vk.GraphicsPipelineCreateInfo{pipelineBuilder}, nil, planePipeline)
	OrPanic(as.NewError(ret))
	app.planePipeline = planePipeline[0]
}

func (app *Application) prepareRoadPipeline() {
	dev := app.Context().Device()

	// Load plane shaders
	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/road.vert.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, vs, nil)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/road.frag.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, fs, nil)

	pipelineBuilder := vk.GraphicsPipelineCreateInfo{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.pipelineLayout,
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				vk.DynamicStateScissor,
				vk.DynamicStateViewport,
			},
		},

		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                         vk.StructureTypePipelineVertexInputStateCreateInfo,
			VertexBindingDescriptionCount: 1,
			PVertexBindingDescriptions: []vk.VertexInputBindingDescription{{
				Binding:   0,
				Stride:    uint32(unsafe.Sizeof(RoadVertex{})),
				InputRate: vk.VertexInputRateVertex,
			}},
			VertexAttributeDescriptionCount: 2,
			PVertexAttributeDescriptions: []vk.VertexInputAttributeDescription{
				// Position
				{
					Binding:  0,
					Location: 0,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   0,
				},
				// Color
				{
					Binding:  0,
					Location: 1,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   4 * 3,
				},
			},
		},

		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:    vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			Topology: vk.PrimitiveTopologyTriangleList,
		},

		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeBackBit),
			FrontFace:   vk.FrontFaceCounterClockwise,
			LineWidth:   1.0,

			DepthBiasEnable:         vk.True,
			DepthBiasConstantFactor: -10.0,
			DepthBiasClamp:          0.0,
			DepthBiasSlopeFactor:    0.0,
		},

		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask: 0xF,
				BlendEnable:    vk.False,
			}},
		},

		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
			RasterizationSamples: vk.SampleCount1Bit,
		},

		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
		},

		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.False,
			DepthCompareOp:        vk.CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			StencilTestEnable:     vk.False,
		},

		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}

	// Create the pipeline
	roadPipeline := make([]vk.Pipeline, 1)
	ret := vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, []vk.GraphicsPipelineCreateInfo{pipelineBuilder}, nil, roadPipeline)
	OrPanic(as.NewError(ret))
	app.roadPipeline = roadPipeline[0]
}

func (app *Application) prepareObstructionPipeline() {
	dev := app.Context().Device()

	// Load obstruction shaders
	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/obstruction.vert.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, vs, nil)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/obstruction.frag.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, fs, nil)

	pipelineBuilder := vk.GraphicsPipelineCreateInfo{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.pipelineLayout,
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				vk.DynamicStateScissor,
				vk.DynamicStateViewport,
			},
		},

		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                         vk.StructureTypePipelineVertexInputStateCreateInfo,
			VertexBindingDescriptionCount: 1,
			PVertexBindingDescriptions: []vk.VertexInputBindingDescription{{
				Binding:   0,
				Stride:    uint32(unsafe.Sizeof(ObstructionVertex{})),
				InputRate: vk.VertexInputRateVertex,
			}},
			VertexAttributeDescriptionCount: 2,
			PVertexAttributeDescriptions: []vk.VertexInputAttributeDescription{
				// Position
				{
					Binding:  0,
					Location: 0,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   0,
				},
				// Color
				{
					Binding:  0,
					Location: 1,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   4 * 3,
				},
			},
		},

		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:    vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			Topology: vk.PrimitiveTopologyTriangleList,
		},

		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeBackBit),
			FrontFace:   vk.FrontFaceCounterClockwise,
			LineWidth:   1.0,

			DepthBiasEnable:         vk.True,
			DepthBiasConstantFactor: -10.0,
			DepthBiasClamp:          0.0,
			DepthBiasSlopeFactor:    0.0,
		},

		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask: 0xF,
				BlendEnable:    vk.False,
			}},
		},

		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
			RasterizationSamples: vk.SampleCount1Bit,
		},

		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
		},

		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.False,
			DepthCompareOp:        vk.CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			StencilTestEnable:     vk.False,
		},

		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}

	// Create the pipeline
	obstructionPipeline := make([]vk.Pipeline, 1)
	ret := vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, []vk.GraphicsPipelineCreateInfo{pipelineBuilder}, nil, obstructionPipeline)
	OrPanic(as.NewError(ret))
	app.obstructionPipeline = obstructionPipeline[0]
}

func (app *Application) prepareAirspacePipeline() {
	dev := app.Context().Device()

	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/airspace.vert.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, vs, nil)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/airspace.frag.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, fs, nil)

	pipelineBuilder := vk.GraphicsPipelineCreateInfo{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.pipelineLayout,
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				vk.DynamicStateScissor,
				vk.DynamicStateViewport,
			},
		},

		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                         vk.StructureTypePipelineVertexInputStateCreateInfo,
			VertexBindingDescriptionCount: 1,
			PVertexBindingDescriptions: []vk.VertexInputBindingDescription{{
				Binding:   0,
				Stride:    uint32(unsafe.Sizeof(AirspaceVertex{})),
				InputRate: vk.VertexInputRateVertex,
			}},
			VertexAttributeDescriptionCount: 2,
			PVertexAttributeDescriptions: []vk.VertexInputAttributeDescription{

				{
					Binding:  0,
					Location: 0,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   0,
				},
				{
					Binding:  0,
					Location: 1,
					Format:   vk.FormatR32g32b32a32Sfloat,
					Offset:   12,
				},
			},
		},

		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:    vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			Topology: vk.PrimitiveTopologyTriangleList,
		},

		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeBackBit),
			FrontFace:   vk.FrontFaceCounterClockwise,
			LineWidth:   1.0,

			DepthBiasEnable: vk.False,
		},

		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask:      0x0,
				BlendEnable:         vk.True,
				SrcColorBlendFactor: vk.BlendFactorSrcAlpha,
				DstColorBlendFactor: vk.BlendFactorOneMinusSrcAlpha,
				ColorBlendOp:        vk.BlendOpAdd,
				SrcAlphaBlendFactor: vk.BlendFactorOne,
				DstAlphaBlendFactor: vk.BlendFactorZero,
				AlphaBlendOp:        vk.BlendOpAdd,
			}},
		},

		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
			RasterizationSamples: vk.SampleCount1Bit,
		},

		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
		},

		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.True,
			DepthCompareOp:        vk.CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			StencilTestEnable:     vk.True,
			Front: vk.StencilOpState{
				FailOp:      vk.StencilOpKeep,
				PassOp:      vk.StencilOpReplace,
				DepthFailOp: vk.StencilOpKeep,
				CompareOp:   vk.CompareOpAlways,
				CompareMask: 0xFF,
				WriteMask:   0xFF,
				Reference:   1,
			},
			Back: vk.StencilOpState{
				FailOp:      vk.StencilOpKeep,
				PassOp:      vk.StencilOpReplace,
				DepthFailOp: vk.StencilOpKeep,
				CompareOp:   vk.CompareOpAlways,
				CompareMask: 0xFF,
				WriteMask:   0xFF,
				Reference:   1,
			},
		},

		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}

	airspacePipeline := make([]vk.Pipeline, 1)
	ret := vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, []vk.GraphicsPipelineCreateInfo{pipelineBuilder}, nil, airspacePipeline)
	OrPanic(as.NewError(ret))
	app.airspacePipeline = airspacePipeline[0]
}

func (app *Application) prepareAirspaceStencilPipeline() {
	dev := app.Context().Device()

	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/airspace.vert.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, vs, nil)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/airspace.frag.spv"))
	OrPanic(err)
	defer vk.DestroyShaderModule(dev, fs, nil)

	pipelineBuilder := vk.GraphicsPipelineCreateInfo{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.pipelineLayout,
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				vk.DynamicStateScissor,
				vk.DynamicStateViewport,
			},
		},

		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                         vk.StructureTypePipelineVertexInputStateCreateInfo,
			VertexBindingDescriptionCount: 1,
			PVertexBindingDescriptions: []vk.VertexInputBindingDescription{{
				Binding:   0,
				Stride:    uint32(unsafe.Sizeof(AirspaceVertex{})),
				InputRate: vk.VertexInputRateVertex,
			}},
			VertexAttributeDescriptionCount: 2,
			PVertexAttributeDescriptions: []vk.VertexInputAttributeDescription{

				{
					Binding:  0,
					Location: 0,
					Format:   vk.FormatR32g32b32Sfloat,
					Offset:   0,
				},
				{
					Binding:  0,
					Location: 1,
					Format:   vk.FormatR32g32b32a32Sfloat,
					Offset:   12,
				},
			},
		},

		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:    vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			Topology: vk.PrimitiveTopologyTriangleList,
		},

		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeBackBit),
			FrontFace:   vk.FrontFaceCounterClockwise,
			LineWidth:   1.0,

			DepthBiasEnable: vk.False,
		},

		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask:      0xF,
				BlendEnable:         vk.True,
				SrcColorBlendFactor: vk.BlendFactorSrcAlpha,
				DstColorBlendFactor: vk.BlendFactorOneMinusSrcAlpha,
				ColorBlendOp:        vk.BlendOpAdd,
				SrcAlphaBlendFactor: vk.BlendFactorOne,
				DstAlphaBlendFactor: vk.BlendFactorZero,
				AlphaBlendOp:        vk.BlendOpAdd,
			}},
		},

		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
			RasterizationSamples: vk.SampleCount1Bit,
		},

		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
		},

		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.True,
			DepthCompareOp:        vk.CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			StencilTestEnable:     vk.True,
			Front: vk.StencilOpState{
				FailOp:      vk.StencilOpKeep,
				PassOp:      vk.StencilOpKeep,
				DepthFailOp: vk.StencilOpKeep,
				CompareOp:   vk.CompareOpEqual,
				CompareMask: 0xFF,
				WriteMask:   0x00,
				Reference:   1,
			},
			Back: vk.StencilOpState{
				FailOp:      vk.StencilOpKeep,
				PassOp:      vk.StencilOpKeep,
				DepthFailOp: vk.StencilOpKeep,
				CompareOp:   vk.CompareOpEqual,
				CompareMask: 0xFF,
				WriteMask:   0x00,
				Reference:   1,
			},
		},

		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}

	airspaceStencilPipeline := make([]vk.Pipeline, 1)
	ret := vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, []vk.GraphicsPipelineCreateInfo{pipelineBuilder}, nil, airspaceStencilPipeline)
	OrPanic(as.NewError(ret))
	app.airspaceStencilPipeline = airspaceStencilPipeline[0]
}

func (app *Application) preparePipeline() {
	dev := app.Context().Device()

	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/shader.vert.spv"))
	OrPanic(err)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/shader.frag.spv"))
	OrPanic(err)

	var pipelineCache vk.PipelineCache
	ret := vk.CreatePipelineCache(dev, &vk.PipelineCacheCreateInfo{
		SType: vk.StructureTypePipelineCacheCreateInfo,
	}, nil, &pipelineCache)
	OrPanic(as.NewError(ret))
	app.pipelineCache = pipelineCache

	bindingDescriptions := []vk.VertexInputBindingDescription{
		{
			Binding:   0,
			Stride:    uint32(chunk.VertexSize),
			InputRate: vk.VertexInputRateVertex,
		},
	}

	attributeDescriptions := []vk.VertexInputAttributeDescription{
		{
			Binding:  0,
			Location: 0,
			Format:   vk.FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(chunk.Vertex{}.Position)),
		},
		{
			Binding:  0,
			Location: 1,
			Format:   vk.FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(chunk.Vertex{}.TexCoord)),
		},
		{
			Binding:  0,
			Location: 2,
			Format:   vk.FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(chunk.Vertex{}.Normal)),
		},
	}

	pipelineCreateInfos := []vk.GraphicsPipelineCreateInfo{{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.pipelineLayout,
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				vk.DynamicStateScissor,
				vk.DynamicStateViewport,
			},
		},
		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                           vk.StructureTypePipelineVertexInputStateCreateInfo,
			VertexBindingDescriptionCount:   uint32(len(bindingDescriptions)),
			PVertexBindingDescriptions:      bindingDescriptions,
			VertexAttributeDescriptionCount: uint32(len(attributeDescriptions)),
			PVertexAttributeDescriptions:    attributeDescriptions,
		},
		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:                  vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			Topology:               vk.PrimitiveTopologyTriangleStrip,
			PrimitiveRestartEnable: vk.True,
		},
		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeBackBit),
			FrontFace:   vk.FrontFaceCounterClockwise,
			LineWidth:   1.0,
		},
		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask: 0xF,
				BlendEnable:    vk.False,
			}},
		},
		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                vk.StructureTypePipelineMultisampleStateCreateInfo,
			RasterizationSamples: vk.SampleCount1Bit,
		},
		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
		},
		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 vk.StructureTypePipelineDepthStencilStateCreateInfo,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.True,
			DepthCompareOp:        vk.CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			Back: vk.StencilOpState{
				FailOp:    vk.StencilOpKeep,
				PassOp:    vk.StencilOpKeep,
				CompareOp: vk.CompareOpAlways,
			},
			StencilTestEnable: vk.False,
			Front: vk.StencilOpState{
				FailOp:    vk.StencilOpKeep,
				PassOp:    vk.StencilOpKeep,
				CompareOp: vk.CompareOpAlways,
			},
		},
		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}}
	pipeline := make([]vk.Pipeline, 1)
	// TODO: When obstructions and roads are rendered simultaneously, program occasionally crashes at this stage
	// Temporary fix: garbage collect and free memory
	runtime.GC()
	debug.FreeOSMemory()
	ret = vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, pipelineCreateInfos, nil, pipeline)
	OrPanic(as.NewError(ret))
	app.pipeline = pipeline[0]

	vk.DestroyShaderModule(dev, vs, nil)
	vk.DestroyShaderModule(dev, fs, nil)
}

func (app *Application) prepareDescriptorPool() {
	dev := app.Context().Device()
	swapchainImageResources := app.Context().SwapchainImageResources()
	var descPool vk.DescriptorPool
	ret := vk.CreateDescriptorPool(dev, &vk.DescriptorPoolCreateInfo{
		SType:         vk.StructureTypeDescriptorPoolCreateInfo,
		MaxSets:       uint32(len(swapchainImageResources)),
		PoolSizeCount: 2,
		PPoolSizes: []vk.DescriptorPoolSize{{
			Type:            vk.DescriptorTypeUniformBuffer,
			DescriptorCount: uint32(len(swapchainImageResources)),
		}, {
			Type:            vk.DescriptorTypeCombinedImageSampler,
			DescriptorCount: uint32(len(swapchainImageResources) * len(texEnabled)),
		}},
	}, nil, &descPool)
	OrPanic(as.NewError(ret))
	app.descPool = descPool
}

func (app *Application) prepareDescriptorSet() {
	dev := app.Context().Device()
	swapchainImageResources := app.Context().SwapchainImageResources()

	texInfos := make([]vk.DescriptorImageInfo, 0, len(app.textures))
	for _, tex := range app.textures {
		texInfos = append(texInfos, vk.DescriptorImageInfo{
			Sampler:     tex.sampler,
			ImageView:   tex.view,
			ImageLayout: vk.ImageLayoutGeneral,
		})
	}

	for _, res := range swapchainImageResources {
		var set vk.DescriptorSet
		ret := vk.AllocateDescriptorSets(dev, &vk.DescriptorSetAllocateInfo{
			SType:              vk.StructureTypeDescriptorSetAllocateInfo,
			DescriptorPool:     app.descPool,
			DescriptorSetCount: 1,
			PSetLayouts:        []vk.DescriptorSetLayout{app.descLayout},
		}, &set)
		OrPanic(as.NewError(ret))

		res.SetDescriptorSet(set)

		vk.UpdateDescriptorSets(dev, 2, []vk.WriteDescriptorSet{{
			SType:           vk.StructureTypeWriteDescriptorSet,
			DstBinding:      0,
			DstSet:          set,
			DescriptorCount: 1,
			DescriptorType:  vk.DescriptorTypeUniformBuffer,
			PBufferInfo: []vk.DescriptorBufferInfo{{
				Offset: 0,
				Range:  vk.DeviceSize(UBOSize),
				Buffer: res.UniformBuffer(),
			}},
		}, {
			SType:           vk.StructureTypeWriteDescriptorSet,
			DstBinding:      1,
			DstSet:          set,
			DescriptorCount: uint32(len(texEnabled)),
			DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
			PImageInfo:      texInfos,
		}}, 0, nil)
	}
}

func (app *Application) prepareFramebuffers() {
	dev := app.Context().Device()
	swapchainImageResources := app.Context().SwapchainImageResources()

	for _, res := range swapchainImageResources {
		var fb vk.Framebuffer

		ret := vk.CreateFramebuffer(dev, &vk.FramebufferCreateInfo{
			SType:           vk.StructureTypeFramebufferCreateInfo,
			RenderPass:      app.renderPass,
			AttachmentCount: 2,
			PAttachments:    []vk.ImageView{res.View(), app.depth.view},
			Width:           app.width,
			Height:          app.height,
			Layers:          1,
		}, nil, &fb)
		OrPanic(as.NewError(ret))

		res.SetFramebuffer(fb)
	}
}

func (app *Application) VulkanContextPrepare() error {
	dim := app.Context().SwapchainDimensions()
	dev := app.Context().Device()
	memProps := app.Context().Platform().MemoryProperties()
	app.height = dim.Height
	app.width = dim.Width

	app.initUBO()
	app.prepareDepth()
	app.prepareTextures()

	// Initialize StreamManager
	app.StreamManager = NewStreamManager(&dev, &memProps, app.Cfg)
	app.StreamManager.UpdateCameraPosition(app.Camera.Position())
	app.StreamManager.Start()

	app.RoadStreamer = streaming.NewDegreeStreamer(app.buildRoad)
	app.RoadStreamer.UpdatePos(app.Camera.Position())

	app.ObstructionStreamer = streaming.NewDegreeStreamer(app.buildObstruction)
	app.ObstructionStreamer.UpdatePos(app.Camera.Position())

	app.AirspaceStreamer = streaming.NewDegreeStreamer(app.buildAirspace)
	app.AirspaceStreamer.UpdatePos(app.Camera.Position())
	app.currentAirspace = nil

	app.prepareUniformBuffers()
	app.prepareDescriptorLayout()
	app.prepareRenderPass()
	app.preparePipeline()
	app.prepareSkyboxCube()
	app.prepareSkyboxPipeline()
	app.preparePlaneModel()
	app.preparePlanePipeline()
	app.prepareAirspacePipeline()
	app.prepareAirspaceStencilPipeline()
	app.prepareRoadPipeline()
	app.prepareObstructionPipeline()
	app.prepareDescriptorPool()
	app.prepareDescriptorSet()
	app.prepareFramebuffers()

	// Initialize command buffers
	swapchainImageResources := app.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		app.updateCommandBuffer(res, res.CommandBuffer())
	}

	platform := app.Context().Platform()
	app.CimguiBackend, _ = backend.CreateBackend(glfwvulkanbackend.NewGLFWBackend())
	app.CimguiBackend.AttachToExistingWindow(
		app.WindowHandle,
		platform.Instance(),
		platform.Device(),
		platform.PhysicalDevice(),
		platform.GraphicsQueue(),
		app.pipelineCache,
		platform.GraphicsQueueFamilyIndex(),
		app.Context().SwapchainImageResources(),
		app.Context().SwapchainDimensions(),
	)
	// app.UIRenderer = ui.NewUIRenderer(
	// 	platform.Instance(),
	// 	platform.Device(),
	// 	platform.PhysicalDevice(),
	// 	platform.GraphicsQueue(),
	// 	app.pipelineCache,
	// 	platform.GraphicsQueueFamilyIndex(),
	// )

	return nil
}

func OrPanic(err error, finalizers ...func()) {
	if err != nil {
		for _, fn := range finalizers {
			fn()
		}
		panic(err)
	}
}

// Converts the given file to a byte array to be used
func FileToBytes(filename string) []byte {
	data, err := FileToBytesChecked(filename)
	OrPanic(err)
	return data
}

func FileToBytesChecked(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file into data
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()
	data := make([]byte, fileSize)
	_, err = file.Read(data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Handle stride and alignment properly
func loadTextureData(name string) ([]byte, int, int, int, error) {
	data := FileToBytes(name)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, 0, err
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Create RGBA image with standard stride
	newImg := image.NewRGBA(bounds)

	// Draw the image
	draw.Draw(newImg, newImg.Bounds(), img, image.Point{}, draw.Src)

	// Return the stride (bytes per row) as well
	return []byte(newImg.Pix), width, height, newImg.Stride, nil
}

// copyTextureData copies texture data row by row, respecting the GPU's expected memory layout
func copyTextureData(dst unsafe.Pointer, src []byte, width, height, sourceStride, destStride int) {
	// For each row in the image
	for row := range height {
		// Calculate source and destination row pointers
		srcOffset := row * sourceStride
		dstOffset := row * destStride

		// Convert destination offset to unsafe.Pointer
		dstRow := unsafe.Add(dst, uintptr(dstOffset))

		// Copy a single row (width * 4 bytes for RGBA)
		bytesPerPixel := 4
		rowSize := width * bytesPerPixel

		// Use the Vulkan provided Memcopy for the row data
		vk.Memcopy(dstRow, src[srcOffset:srcOffset+rowSize])
	}
}
