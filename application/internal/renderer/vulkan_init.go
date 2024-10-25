package renderer

import (
	"errors"
	"fmt"
	"log"
	"unsafe"

	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// This file is for vulkan initialization

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
		"VK_LAYER_KHRONOS_validation",
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
	extensions = append(extensions, "VK_EXT_debug_report") // TODO: Remove if no debug
	return extensions
}

// Initialize the Uniform Buffer Object
func (app *Application) initUBO() {
	aspectRatio := float32(app.width) / float32(app.height)
	// 45 FOV, Aspect 1.0, Near plane, Far Plane
	app.ubo.Projection.Perspective(lin.DegreesToRadians(FOV), aspectRatio, 0.1, VIEW_DISTANCE)
	app.ubo.View = app.camera.GetViewMatrix()
	app.ubo.Model.Identity()
	app.ubo.Projection[1][1] *= -1
	temp := lin.Vec3{0, 0.1, 0.1}
	app.ubo.LightDir.Norm(&temp)
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
	_, width, height, err := loadTextureData(path, 0)
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

		data, _, _, err := loadTextureData(path, int(layout.RowPitch))
		OrPanic(err)
		if len(data) > 0 {
			var pData unsafe.Pointer
			ret = vk.MapMemory(dev, tex.mem, 0, vk.DeviceSize(len(data)), 0, &pData)
			if IsError(ret) {
				log.Printf("vulkan warning: failed to map device memory for data (len=%d)", len(data))
				return tex
			}
			n := vk.Memcopy(pData, data)
			if n != len(data) {
				log.Printf("vulkan warning: failed to copy data, %d != %d", n, len(data))
			}
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
				TexturePipelineStageTopOfPipeBit, TexturePipelineStageTransferBit)

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
			MagFilter:               SamplerFilterNearest,
			MinFilter:               SamplerFilterNearest,
			MipmapMode:              SamplerMipmapModeNearest,
			AddressModeU:            SamplerAddressModeRepeat,
			AddressModeV:            SamplerAddressModeRepeat,
			AddressModeW:            SamplerAddressModeRepeat,
			AnisotropyEnable:        SamplerAnisotropyEnable,
			MaxAnisotropy:           SamplerMaxAnisotropy,
			CompareOp:               SamplerCompareOpNever,
			BorderColor:             SamplerBorderColorFloatOpaqueWhite,
			UnnormalizedCoordinates: SamplerUnnormalizedCoordinates,
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
				StageFlags:      vk.ShaderStageFlags(ShaderStageVertexBit),
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

func (app *Application) preparePipeline() {
	dev := app.Context().Device()

	vs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/cube.vert.spv"))
	OrPanic(err)
	fs, err := as.LoadShaderModule(dev, FileToBytes("assets/shaders/cube.frag.spv"))
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
			Format:   FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(Vertex{}.Position)),
		},
		{
			Binding:  0,
			Location: 1,
			Format:   FormatR32g32b32Sfloat,
			Offset:   uint32(unsafe.Offsetof(Vertex{}.TexCoord)),
		},
		{
			Binding:  0,
			Location: 2,
			Format:   FormatR32g32b32Sfloat,
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

	// Testing for loading all maps and chunks
	// Update the last position so that we load the map
	app.ChunkManager = NewChunkManager(&dev, &memProps)
	app.ChunkManager.UpdateLastPosition(*app.camera.Pos)
	app.ChunkManager.LoadChunks()
	app.ChunkManager.FillBuffers()
	app.frustum.Update(&app.ubo.View, &app.ubo.Projection)
	app.determineVisibleChunks()

	app.prepareUniformBuffers() // need to be called after loadhm()
	app.prepareDescriptorLayout()
	app.prepareRenderPass()
	app.preparePipeline()
	app.prepareDescriptorPool()
	app.prepareDescriptorSet()
	app.prepareFramebuffers()

	swapchainImageResources := app.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		app.updateCommandBuffer(res, res.CommandBuffer())
	}
	return nil
}
