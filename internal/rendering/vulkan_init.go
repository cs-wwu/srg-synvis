package rendering

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"unsafe"

	ui "github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	as "github.com/cs-wwu/srg-synvis/pkg/asche"

	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file is for vulkan initialization

func (app *Application) IsUncappedFPS() bool {
	return app.cfg.UncapFPS
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
	if app.cfg.DebugMode {
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
	return &as.SwapchainDimensions{
		Width: WINDOW_WIDTH, Height: WINDOW_HEIGHT, Format: vk.FormatB8g8r8a8Unorm,
	}
}

func (app *Application) SetSwapchainDimensions(dim *as.SwapchainDimensions) {
	app.width = dim.Width
	app.height = dim.Height
}

func (app *Application) VulkanInstanceExtensions() []string {
	extensions := app.WindowHandle.GetRequiredInstanceExtensions()
	if app.cfg.DebugMode {
		extensions = append(extensions, "VK_EXT_debug_report")
	}
	return extensions
}

func (app *Application) VulkanPresentMode() vk.PresentMode {
	if app.cfg.UncapFPS {
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
	aspectRatio := float32(app.width) / float32(app.height)
	app.ubo.Projection.Perspective(lin.DegreesToRadians(app.cfg.Fov), aspectRatio, 1.0, app.cfg.RenderDistance)
	app.ubo.View = app.camera.ViewMatrix()
	app.ubo.Model.Identity()
	app.ubo.Projection[1][1] *= -1

	temp := lin.Vec3{0, 0.1, 0.1}
	app.ubo.LightDir.Norm(&temp)
	app.ubo.RenderDistance = app.cfg.RenderDistance
	log.Printf("vulkan: render distance: %f", app.cfg.RenderDistance)

	// Fix the logic to match shader expectations
	if app.cfg.DisableTerrainWarnings {
		app.ubo.DisableWarnings = 1.0 // 1.0 = disabled
	} else {
		app.ubo.DisableWarnings = 0.0 // 0.0 = enabled
	}

	if app.cfg.DisableFog {
		app.ubo.DisableFog = 1.0 // 1.0 = disabled
		log.Printf("Disabling fog\n")
	} else {
		log.Printf("Enabling fog\n")
		app.ubo.DisableFog = 0.0 // 0.0 = enabled
	}
}

// Prepare depth buffers
func (app *Application) prepareDepth() {
	dev := app.Context().Device()
	app.depth = &Depth{
		format: DepthFormat,
	}
	ret := vk.CreateImage(dev, &vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: DepthImageType,
		Format:    DepthFormat,
		Extent: vk.Extent3D{
			Width:  app.width,
			Height: app.height,
			Depth:  DepthImageDepth,
		},
		MipLevels:   DepthMipLevels,
		ArrayLayers: DepthArrayLayers,
		Samples:     DepthSamples,
		Tiling:      DepthImageTiling,
		Usage:       vk.ImageUsageFlags(DepthImageUsage),
	}, nil, &app.depth.image)
	OrPanic(as.NewError(ret))

	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(dev, app.depth.image, &memReqs)
	memReqs.Deref()

	memProps := app.Context().Platform().MemoryProperties()
	memTypeIndex, _ := as.FindRequiredMemoryTypeFallback(memProps,
		vk.MemoryPropertyFlagBits(memReqs.MemoryTypeBits), DepthMemoryProperty)
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
			AspectMask: vk.ImageAspectFlags(DepthAspectMask),
			LevelCount: DepthSubresourceLevels,
			LayerCount: DepthSubresourceLayers,
		},
		ViewType: DepthViewType,
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
		imageLayout: TextureDefaultImageLayout}

	var image vk.Image
	imageCreateInfo := vk.ImageCreateInfo{
		SType:         vk.StructureTypeImageCreateInfo,
		ImageType:     TextureImageType,
		Format:        TextureFormat,
		Extent:        vk.Extent3D{Width: uint32(width), Height: uint32(height), Depth: TextureImageDepth},
		MipLevels:     TextureMipLevels,
		ArrayLayers:   TextureArrayLayers,
		Samples:       TextureSamples,
		Tiling:        tiling,
		Usage:         vk.ImageUsageFlags(usage),
		InitialLayout: TextureInitialLayout,
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

	hostVisible := memoryProps&TextureMemoryPropertyHost != 0
	if hostVisible {
		var layout vk.SubresourceLayout
		vk.GetImageSubresourceLayout(dev, tex.image, &vk.ImageSubresource{
			AspectMask: vk.ImageAspectFlags(TextureAspectMask),
		}, &layout)
		layout.Deref()

		data, width, height, sourceStride, err := loadTextureData(path)
		OrPanic(err)
		if len(data) > 0 {
			var pData unsafe.Pointer
			ret = vk.MapMemory(dev, tex.mem, 0, layout.Size, 0, &pData)
			if IsError(ret) {
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
		SType:         ImageMemoryBarrierSType,
		SrcAccessMask: vk.AccessFlags(srcAccessMask),
		DstAccessMask: 0,
		OldLayout:     oldImageLayout,
		NewLayout:     newImageLayout,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(aspectMask),
			LayerCount: ImageSubresourceLayerCount,
			LevelCount: ImageSubresourceLevelCount,
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
		ImageBarrierDependencyFlags,
		ImageBarrierMemoryBarrierCount, nil,
		ImageBarrierBufferMemoryBarrierCount, nil,
		ImageBarrierImageMemoryBarrierCount, []vk.ImageMemoryBarrier{imageMemoryBarrier})
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

			tex = app.prepareTextureImage(path, TextureImageTilingLinear, TextureImageUsageSampledBit,
				TextureMemoryPropertyHostVisibleBit|TextureMemoryPropertyHostCoherentBit)

			// Nothing in the pipeline needs to be complete to start, and don't allow fragment
			// shader to run until layout transition completes
			app.setImageLayout(tex.image, TextureImageAspect,
				TextureImageLayoutPreinitialized, tex.imageLayout,
				TextureAccessHostWriteBit,
				TexturePipelineStageHostBit, TexturePipelineStageFragmentShaderBit)

		} else if props.OptimalTilingFeatures&vk.FormatFeatureFlags(vk.FormatFeatureSampledImageBit) != 0 {
			//  Must use staging buffer to copy linear texture to optimized
			log.Println("vulkan warn: using staging buffers")

			staging := app.prepareTextureImage(path, TextureImageTilingLinear, TextureImageUsageTransferSrcBit,
				TextureMemoryPropertyHostVisibleBit|TextureMemoryPropertyHostCoherentBit)
			tex = app.prepareTextureImage(path, TextureImageTilingOptimal,
				TextureImageUsageTransferDstBit|TextureImageUsageSampledBit, TextureMemoryPropertyDeviceLocalBit)

			app.setImageLayout(staging.image, TextureImageAspect,
				TextureImageLayoutPreinitialized, TextureImageLayoutTransferSrcOptimal,
				TextureAccessHostWriteBit,
				TexturePipelineStageHostBit, TexturePipelineStageTransferBit)

			app.setImageLayout(tex.image, TextureImageAspect,
				TextureImageLayoutPreinitialized, TextureImageLayoutTransferDstOptimal,
				TextureAccessHostWriteBit,
				TexturePipelineStageTopOfPipeBit, TexturePipelineStageTransferBit)

			cmd := app.Context().CommandBuffer()
			if cmd == nil {
				OrPanic(errors.New("vulkan: command buffer not initialized"))
			}
			vk.CmdCopyImage(cmd, staging.image, TextureImageLayoutTransferSrcOptimal,
				tex.image, TextureImageLayoutTransferDstOptimal,
				1, []vk.ImageCopy{{
					SrcSubresource: vk.ImageSubresourceLayers{
						AspectMask: vk.ImageAspectFlags(TextureImageAspect),
						LayerCount: 1,
					},
					SrcOffset: vk.Offset3D{
						X: 0, Y: 0, Z: 0,
					},
					DstSubresource: vk.ImageSubresourceLayers{
						AspectMask: vk.ImageAspectFlags(TextureImageAspect),
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
			app.setImageLayout(tex.image, TextureImageAspect,
				TextureImageLayoutTransferDstOptimal, tex.imageLayout,
				TextureAccessTransferWriteBit,
				TexturePipelineStageTransferBit, TexturePipelineStageFragmentShaderBit)
			// cannot destroy until cmd is submitted.. must keep a list somewhere
			// staging.DestroyImage(dev)
		} else {
			OrPanic(errors.New("vulkan: R8G8B8A8_UNORM not supported as texture image format"))
		}

		var sampler vk.Sampler
		ret := vk.CreateSampler(dev, &vk.SamplerCreateInfo{
			SType:                   SamplerCreateInfoSType,
			MagFilter:               vk.FilterLinear,
			MinFilter:               vk.FilterLinear,
			MipmapMode:              SamplerMipmapModeNearest,
			AddressModeU:            SamplerAddressModeRepeat,
			AddressModeV:            SamplerAddressModeRepeat,
			AddressModeW:            SamplerAddressModeRepeat,
			CompareOp:               SamplerCompareOpNever,
			BorderColor:             SamplerBorderColorFloatOpaqueWhite,
			UnnormalizedCoordinates: vk.False,
		}, nil, &sampler)
		OrPanic(as.NewError(ret))
		tex.sampler = sampler

		var view vk.ImageView
		ret = vk.CreateImageView(dev, &vk.ImageViewCreateInfo{
			SType:    ImageViewCreateInfoSType,
			Image:    tex.image,
			ViewType: ImageViewType2D,
			Format:   TextureFormat,
			Components: vk.ComponentMapping{
				R: ComponentSwizzleR,
				G: ComponentSwizzleG,
				B: ComponentSwizzleB,
				A: ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(TextureImageAspect),
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
func (app *Application) prepareUniformBuffers() {
	dev := app.Context().Device()

	dataRaw := app.ubo.Data()

	memProps := app.Context().Platform().MemoryProperties()
	swapchainImageResources := app.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		buf := as.CreateBuffer(dev, memProps, dataRaw, UniformBufferUsage)
		res.SetUniformBuffer(buf.Buffer, buf.Memory)
	}
}

func (app *Application) prepareDescriptorLayout() {
	dev := app.Context().Device()
	var descLayout vk.DescriptorSetLayout
	ret := vk.CreateDescriptorSetLayout(dev, &vk.DescriptorSetLayoutCreateInfo{
		SType:        DescriptorSetLayoutCreateInfoSType,
		BindingCount: DescriptorSetLayoutBindingCount,
		PBindings: []vk.DescriptorSetLayoutBinding{
			{
				Binding:         DescriptorSetLayoutBinding0,
				DescriptorType:  DescriptorTypeUniformBuffer,
				DescriptorCount: 1,
				StageFlags:      vk.ShaderStageFlags(ShaderStageVertexBit | vk.ShaderStageFragmentBit),
			}, {
				Binding:         DescriptorSetLayoutBinding1,
				DescriptorType:  DescriptorTypeCombinedImageSampler,
				DescriptorCount: uint32(len(texEnabled)),
				StageFlags:      vk.ShaderStageFlags(ShaderStageFragmentBit),
			}},
	}, nil, &descLayout)
	OrPanic(as.NewError(ret))
	app.descLayout = descLayout

	var pipelineLayout vk.PipelineLayout
	ret = vk.CreatePipelineLayout(dev, &vk.PipelineLayoutCreateInfo{
		SType:          PipelineLayoutCreateInfoSType,
		SetLayoutCount: PipelineLayoutSetCount,
		PSetLayouts: []vk.DescriptorSetLayout{
			app.descLayout,
		},
	}, nil, &pipelineLayout)
	OrPanic(as.NewError(ret))
	app.pipelineLayout = pipelineLayout
}

func (app *Application) prepareRenderPass() {
	dev := app.Context().Device()
	var renderPass vk.RenderPass
	ret := vk.CreateRenderPass(dev, &vk.RenderPassCreateInfo{
		SType:           RenderPassCreateInfoSType,
		AttachmentCount: RenderPassAttachmentCount,
		PAttachments: []vk.AttachmentDescription{{
			Format:         app.Context().SwapchainDimensions().Format,
			Samples:        SampleCount1Bit,
			LoadOp:         AttachmentLoadOpClear,
			StoreOp:        AttachmentStoreOpStore,
			StencilLoadOp:  AttachmentLoadOpDontCare,
			StencilStoreOp: AttachmentStoreOpDontCare,
			InitialLayout:  ImageLayoutUndefined,
			FinalLayout:    ImageLayoutPresentSrc,
		}, {
			Format:         app.depth.format,
			Samples:        SampleCount1Bit,
			LoadOp:         AttachmentLoadOpClear,
			StoreOp:        AttachmentStoreOpDontCare,
			StencilLoadOp:  AttachmentLoadOpDontCare,
			StencilStoreOp: AttachmentStoreOpDontCare,
			InitialLayout:  ImageLayoutUndefined,
			FinalLayout:    ImageLayoutDepthStencilAttachmentOptimal,
		}},
		SubpassCount: RenderPassSubpassCount,
		PSubpasses: []vk.SubpassDescription{{
			PipelineBindPoint:    PipelineBindPointGraphics,
			ColorAttachmentCount: 1,
			PColorAttachments: []vk.AttachmentReference{{
				Attachment: 0,
				Layout:     ImageLayoutColorAttachmentOptimal,
			}},
			PDepthStencilAttachment: &vk.AttachmentReference{
				Attachment: 1,
				Layout:     ImageLayoutDepthStencilAttachmentOptimal,
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

func (app *Application) preparePipeline() {
	dev := app.Context().Device()

	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/shader.vert.spv"))
	OrPanic(err)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/shader.frag.spv"))
	OrPanic(err)

	var pipelineCache vk.PipelineCache
	ret := vk.CreatePipelineCache(dev, &vk.PipelineCacheCreateInfo{
		SType: PipelineCacheCreateInfoSType,
	}, nil, &pipelineCache)
	OrPanic(as.NewError(ret))
	app.pipelineCache = pipelineCache

	bindingDescriptions := []vk.VertexInputBindingDescription{
		{
			Binding:   0,
			Stride:    uint32(VertexSize),
			InputRate: VertexInputRateVertex,
		},
	}

	attributeDescriptions := []vk.VertexInputAttributeDescription{
		{
			Binding:  0,
			Location: 0,
			Format:   vk.FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(Vertex{}.Position)),
		},
		{
			Binding:  0,
			Location: 1,
			Format:   vk.FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(Vertex{}.TexCoord)),
		},
		{
			Binding:  0,
			Location: 2,
			Format:   vk.FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(Vertex{}.Normal)),
		},
	}

	pipelineCreateInfos := []vk.GraphicsPipelineCreateInfo{{
		SType:      GraphicsPipelineCreateInfoSType,
		Layout:     app.pipelineLayout,
		RenderPass: app.renderPass,

		PDynamicState: &vk.PipelineDynamicStateCreateInfo{
			SType:             PipelineDynamicStateCreateInfoSType,
			DynamicStateCount: 2,
			PDynamicStates: []vk.DynamicState{
				DynamicStateScissor,
				DynamicStateViewport,
			},
		},
		PVertexInputState: &vk.PipelineVertexInputStateCreateInfo{
			SType:                           PipelineVertexInputStateCreateInfoSType,
			VertexBindingDescriptionCount:   uint32(len(bindingDescriptions)),
			PVertexBindingDescriptions:      bindingDescriptions,
			VertexAttributeDescriptionCount: uint32(len(attributeDescriptions)),
			PVertexAttributeDescriptions:    attributeDescriptions,
		},
		PInputAssemblyState: &vk.PipelineInputAssemblyStateCreateInfo{
			SType:                  PipelineInputAssemblyStateCreateInfoSType,
			Topology:               PrimitiveTopologyTriangleStrip,
			PrimitiveRestartEnable: vk.True,
		},
		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       PipelineRasterizationStateCreateInfoSType,
			PolygonMode: PolygonModeFill,
			CullMode:    vk.CullModeFlags(CullModeBackBit),
			FrontFace:   FrontFaceCounterClockwise,
			LineWidth:   1.0,
		},
		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           PipelineColorBlendStateCreateInfoSType,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				ColorWriteMask: 0xF,
				BlendEnable:    vk.False,
			}},
		},
		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType:                PipelineMultisampleStateCreateInfoSType,
			RasterizationSamples: SampleCount1Bit,
		},
		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         PipelineViewportStateCreateInfoSType,
			ScissorCount:  1,
			ViewportCount: 1,
		},
		PDepthStencilState: &vk.PipelineDepthStencilStateCreateInfo{
			SType:                 PipelineDepthStencilStateCreateInfoSType,
			DepthTestEnable:       vk.True,
			DepthWriteEnable:      vk.True,
			DepthCompareOp:        CompareOpLessOrEqual,
			DepthBoundsTestEnable: vk.False,
			Back: vk.StencilOpState{
				FailOp:    StencilOpKeep,
				PassOp:    StencilOpKeep,
				CompareOp: CompareOpAlways,
			},
			StencilTestEnable: vk.False,
			Front: vk.StencilOpState{
				FailOp:    StencilOpKeep,
				PassOp:    StencilOpKeep,
				CompareOp: CompareOpAlways,
			},
		},
		StageCount: 2,
		PStages: []vk.PipelineShaderStageCreateInfo{{
			SType:  PipelineShaderStageCreateInfoSType,
			Stage:  ShaderStageVertexBit,
			Module: vs,
			PName:  "main\x00",
		}, {
			SType:  PipelineShaderStageCreateInfoSType,
			Stage:  ShaderStageFragmentBit,
			Module: fs,
			PName:  "main\x00",
		}},
	}}
	pipeline := make([]vk.Pipeline, 1)
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
		SType:         DescriptorPoolCreateInfoSType,
		MaxSets:       uint32(len(swapchainImageResources)),
		PoolSizeCount: DescriptorPoolSizeCount,
		PPoolSizes: []vk.DescriptorPoolSize{{
			Type:            DescriptorTypeUniformBuffer,
			DescriptorCount: uint32(len(swapchainImageResources)),
		}, {
			Type:            DescriptorTypeCombinedImageSampler,
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
			ImageLayout: ImageLayoutGeneral,
		})
	}

	for _, res := range swapchainImageResources {
		var set vk.DescriptorSet
		ret := vk.AllocateDescriptorSets(dev, &vk.DescriptorSetAllocateInfo{
			SType:              DescriptorSetAllocateInfoSType,
			DescriptorPool:     app.descPool,
			DescriptorSetCount: DescriptorSetCount,
			PSetLayouts:        []vk.DescriptorSetLayout{app.descLayout},
		}, &set)
		OrPanic(as.NewError(ret))

		res.SetDescriptorSet(set)

		vk.UpdateDescriptorSets(dev, 2, []vk.WriteDescriptorSet{{
			SType:           WriteDescriptorSetSType,
			DstBinding:      0,
			DstSet:          set,
			DescriptorCount: 1,
			DescriptorType:  DescriptorTypeUniformBuffer,
			PBufferInfo: []vk.DescriptorBufferInfo{{
				Offset: 0,
				Range:  vk.DeviceSize(UBOSize),
				Buffer: res.UniformBuffer(),
			}},
		}, {
			SType:           WriteDescriptorSetSType,
			DstBinding:      1,
			DstSet:          set,
			DescriptorCount: uint32(len(texEnabled)),
			DescriptorType:  DescriptorTypeCombinedImageSampler,
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
			SType:           FramebufferCreateInfoSType,
			RenderPass:      app.renderPass,
			AttachmentCount: FramebufferAttachmentCount,
			PAttachments: []vk.ImageView{
				res.View(),
				app.depth.view,
			},
			Width:  app.width,
			Height: app.height,
			Layers: FramebufferLayers,
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
	app.StreamManager = NewStreamManager(&dev, &memProps, app.cfg)
	app.StreamManager.UpdateCameraPosition(app.camera.Position())
	app.StreamManager.Start()

	app.prepareUniformBuffers()
	app.prepareDescriptorLayout()
	app.prepareRenderPass()
	app.preparePipeline()
	app.prepareSkyboxCube()
	app.prepareSkyboxPipeline()
	app.prepareDescriptorPool()
	app.prepareDescriptorSet()
	app.prepareFramebuffers()

	// Initialize command buffers
	swapchainImageResources := app.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		app.updateCommandBuffer(res, res.CommandBuffer())
	}

	platform := app.Context().Platform()
	app.UIRenderer = ui.NewUIRenderer(
		platform.Instance(),
		platform.Device(),
		platform.PhysicalDevice(),
		platform.GraphicsQueue(),
		app.pipelineCache,
		platform.GraphicsQueueFamilyIndex(),
	)

	return nil
}

func IsError(ret vk.Result) bool {
	return ret != vk.Success
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
	file, err := os.Open(filename)
	OrPanic(err)
	defer file.Close()

	// Read the file into data
	fileInfo, err := file.Stat()
	OrPanic(err)
	fileSize := fileInfo.Size()
	data := make([]byte, fileSize)
	_, err = file.Read(data)
	OrPanic(err)

	return data
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
		dstRow := unsafe.Pointer(uintptr(dst) + uintptr(dstOffset))

		// Copy a single row (width * 4 bytes for RGBA)
		bytesPerPixel := 4
		rowSize := width * bytesPerPixel

		// Use the Vulkan provided Memcopy for the row data
		vk.Memcopy(dstRow, src[srcOffset:srcOffset+rowSize])
	}
}
