package renderer

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"unsafe"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"

	as "go-testing/asche2"

	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

func prepareFont(fontPath string) *truetype.Font {
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		panic("Failed to read font file")
	}

	font, err := freetype.ParseFont(fontBytes)
	if err != nil {
		panic("Failed to parse font")
	}
	return font
}

func renderTextToBitmap(font *truetype.Font, text string, size float64, width int, height int, bgc color.RGBA) image.Image {
	c := freetype.NewContext()
	c.SetFont(font)
	c.SetFontSize(size)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	bg := image.NewUniform(bgc)
	draw.Draw(dst, dst.Bounds(), bg, image.Point{0, 0}, draw.Src)
	c.SetDst(dst) // Adjust size as needed
	c.SetSrc(image.White)
	c.SetClip(dst.Bounds())

	pt := freetype.Pt(10, 10+int(c.PointToFixed(size)>>6))
	_, err := c.DrawString(text, pt)
	if err != nil {
		panic("could not draw text")
	}
	return dst
}

// returns a cmd buffer that has begun recording
// intended for use only once
func (app *Application) beginSingleTimeCommands() []vk.CommandBuffer {
	cmdBufferAllocateInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        app.Context().CommandPool(),
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}

	var cmdBuffer = make([]vk.CommandBuffer, 1)
	ret := vk.AllocateCommandBuffers(app.Context().Device(), &cmdBufferAllocateInfo, cmdBuffer)
	OrPanic(as.NewError(ret))

	beginInfo := vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit),
	}
	ret = vk.BeginCommandBuffer(cmdBuffer[0], &beginInfo)
	OrPanic(as.NewError(ret))

	return cmdBuffer
}

// ends an active cmd buffer
func (app *Application) endSingleTimeCommands(cmdBuffer []vk.CommandBuffer) {
	ret := vk.EndCommandBuffer(cmdBuffer[0])
	OrPanic(as.NewError(ret))

	submitInfo := vk.SubmitInfo{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    cmdBuffer,
	}

	ret = vk.QueueSubmit(app.Context().Platform().GraphicsQueue(), 1, []vk.SubmitInfo{submitInfo}, vk.Fence(nil)) // Optional fence for synchronization
	OrPanic(as.NewError(ret))

	// Optionally wait for queue to complete
	vk.QueueWaitIdle(app.Context().Platform().GraphicsQueue())
}

func (app *Application) updateText(str string, size float64, width, height, quad int, bgc color.RGBA) {
	dev := app.Context().Device()
	text := renderTextToBitmap(app.font, str, size, width, height, bgc)

	// upload to buffer
	var layout vk.SubresourceLayout
	vk.GetImageSubresourceLayout(dev, app.uiTextures[quad].image, &vk.ImageSubresource{
		AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
	}, &layout)
	layout.Deref()
	data, width, height, err := loaduiTextureData(text, int(layout.RowPitch))
	if err != nil {
		panic("could not get tex data")
	}
	// uploadsize := width * height * 4
	var pData unsafe.Pointer
	ret := vk.MapMemory(dev, app.uiTextures[quad].stagingbuf.Memory, 0, vk.DeviceSize(len(data)), 0, &pData)
	OrPanic(as.NewError(ret))
	n := vk.Memcopy(pData, data)
	if n != len(data) {
		log.Printf("vulkan warning: failed to copy data, %d != %d", n, len(data))
	}
	vk.FlushMappedMemoryRanges(dev, 1, []vk.MappedMemoryRange{{
		SType:  vk.StructureTypeMappedMemoryRange,
		Memory: app.uiTextures[quad].stagingbuf.Memory,
		Size:   vk.DeviceSize(len(data)),
	}})
	vk.UnmapMemory(dev, app.uiTextures[quad].stagingbuf.Memory)
	// copy to image
	cmd := app.beginSingleTimeCommands()
	if cmd == nil {
		OrPanic(errors.New("vulkan: command buffer not initialized"))
	}
	vk.CmdPipelineBarrier(cmd[0], vk.PipelineStageFlags(vk.PipelineStageHostBit), vk.PipelineStageFlags(vk.PipelineStageTransferBit), 0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{{
		SType:               vk.StructureTypeImageMemoryBarrier,
		DstAccessMask:       vk.AccessFlags(vk.AccessTransferWriteBit),
		OldLayout:           vk.ImageLayoutUndefined,
		NewLayout:           vk.ImageLayoutTransferDstOptimal,
		SrcQueueFamilyIndex: vk.QueueFamilyIgnored,
		DstQueueFamilyIndex: vk.QueueFamilyIgnored,
		Image:               app.uiTextures[quad].image,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
			LayerCount: 1,
			LevelCount: 1,
		},
	}})
	vk.CmdCopyBufferToImage(cmd[0], app.uiTextures[quad].stagingbuf.Buffer, app.uiTextures[quad].image, vk.ImageLayoutTransferDstOptimal, 1, []vk.BufferImageCopy{{
		ImageSubresource: vk.ImageSubresourceLayers{
			MipLevel:   0,
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
			LayerCount: 1,
		},
		// ImageExtent: vk.Extent3D{Width: app.width, Height: app.height, Depth: 1},
		ImageExtent: vk.Extent3D{Width: uint32(width), Height: uint32(height), Depth: 1},
	}})
	vk.CmdPipelineBarrier(cmd[0], vk.PipelineStageFlags(vk.PipelineStageTransferBit), vk.PipelineStageFlags(vk.PipelineStageFragmentShaderBit), 0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{{
		SType:               vk.StructureTypeImageMemoryBarrier,
		SrcAccessMask:       vk.AccessFlags(vk.AccessTransferWriteBit),
		DstAccessMask:       vk.AccessFlags(vk.AccessShaderReadBit),
		OldLayout:           vk.ImageLayoutTransferDstOptimal,
		NewLayout:           vk.ImageLayoutShaderReadOnlyOptimal,
		SrcQueueFamilyIndex: vk.QueueFamilyIgnored,
		DstQueueFamilyIndex: vk.QueueFamilyIgnored,
		Image:               app.uiTextures[quad].image,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
			LayerCount: 1,
			LevelCount: 1,
		},
	}})
	app.endSingleTimeCommands(cmd)
}

func (app *Application) prepareuiQuads() {
	dev := app.Context().Device()

	vertBufSize := len(app.vertices) * UIVertexSize
	dataRaw := make([]byte, vertBufSize)

	for i, vertex := range app.vertices {
		copy(dataRaw[i*UIVertexSize:], vertex.Data())
	}

	memProps := app.Context().Platform().MemoryProperties()
	buf := as.CreateBuffer(dev, memProps, dataRaw, vk.BufferUsageVertexBufferBit)
	app.vertexBufferMemory = buf.Memory
	app.vertexBuffer = buf.Buffer

	// Prepare raw data slice
	indBufSize := len(app.indices) * IndexSize
	dataRaw2 := make([]byte, indBufSize)
	for i, index := range app.indices {
		copy(dataRaw2[i*IndexSize:], index.Data())
	}

	memProps2 := app.Context().Platform().MemoryProperties()
	buf2 := as.CreateBuffer(dev, memProps2, dataRaw2, vk.BufferUsageIndexBufferBit)
	app.indexBufferMemory = buf2.Memory
	app.indexBuffer = buf2.Buffer
}

func (app *Application) initUIDimensions() {
	vertices := []UIVertex{
		// fps box
		{Position: lin.Vec2{-1.0, -1.0}, TexCoord: lin.Vec2{0, 0}},
		{Position: lin.Vec2{0.0, -1.0}, TexCoord: lin.Vec2{1, 0}},
		{Position: lin.Vec2{0.0, 0.0}, TexCoord: lin.Vec2{1, 1}},
		{Position: lin.Vec2{-1.0, 0.0}, TexCoord: lin.Vec2{0, 1}},

		// button
		{Position: lin.Vec2{0.5, -0.75}, TexCoord: lin.Vec2{0, 0}},
		{Position: lin.Vec2{0.75, -0.75}, TexCoord: lin.Vec2{1, 0}},
		{Position: lin.Vec2{0.75, -0.5}, TexCoord: lin.Vec2{1, 1}},
		{Position: lin.Vec2{0.5, -0.5}, TexCoord: lin.Vec2{0, 1}},
	}
	app.vertices = vertices

	app.indices = []Index{
		// Front face (CLOCKWISE)
		0, 1, 2, 2, 3, 0, // fps box
		4, 5, 6, 6, 7, 4, // button
	}
}

func (app *Application) PrepareUI() error {
	app.font = prepareFont("assets/fonts/SourceSansPro-Regular.ttf")
	text := renderTextToBitmap(app.font, "X", 72, int(app.width), int(app.height), color.RGBA{0, 0, 0, 128})
	app.fontImages = append(app.fontImages, text)
	text2 := renderTextToBitmap(app.font, "click", 72, 200, 125, color.RGBA{0, 0, 0, 128})
	app.fontImages = append(app.fontImages, text2)
	app.prepareButtons()
	// look at
	//	descriptorTypeStorageBufferDynamic
	//	descriptorTypeUniformBufferDynamic
	//	GraphicsPipelineCreateInfo.PDynamicState
	//	PipelineDynamicStateCreateFlags

	app.initUIDimensions()
	app.prepareuiQuads()
	return nil
}

func (app *Application) prepareuiDescriptorLayout() {
	dev := app.Context().Device()
	var descLayout vk.DescriptorSetLayout
	ret := vk.CreateDescriptorSetLayout(dev, &vk.DescriptorSetLayoutCreateInfo{
		SType:        vk.StructureTypeDescriptorSetLayoutCreateInfo,
		BindingCount: 1,
		PBindings: []vk.DescriptorSetLayoutBinding{{ // diff
			Binding:         0,
			DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
			DescriptorCount: uint32(len(app.fontImages)),
			StageFlags:      vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
		}},
	}, nil, &descLayout)
	OrPanic(as.NewError(ret))
	app.uidescLayout = descLayout

	var pipelineLayout vk.PipelineLayout
	ret = vk.CreatePipelineLayout(dev, &vk.PipelineLayoutCreateInfo{
		SType:          vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount: 1,
		PSetLayouts: []vk.DescriptorSetLayout{
			app.uidescLayout,
		},
	}, nil, &pipelineLayout)
	OrPanic(as.NewError(ret))
	app.uipipelineLayout = pipelineLayout
}

// second 2d render pass - after worldRenderPass
func (app *Application) prepareuiRenderPass() {
	dev := app.Context().Device()
	var renderPass vk.RenderPass
	ret := vk.CreateRenderPass(dev, &vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: 1,
		PAttachments: []vk.AttachmentDescription{{
			Format:         app.Context().SwapchainDimensions().Format,
			Samples:        vk.SampleCount1Bit,
			LoadOp:         vk.AttachmentLoadOpLoad,
			StoreOp:        vk.AttachmentStoreOpStore,
			StencilLoadOp:  vk.AttachmentLoadOpDontCare,
			StencilStoreOp: vk.AttachmentStoreOpDontCare,
			InitialLayout:  vk.ImageLayoutColorAttachmentOptimal,
			FinalLayout:    vk.ImageLayoutPresentSrc,
		}},
		SubpassCount: 1,
		PSubpasses: []vk.SubpassDescription{{
			PipelineBindPoint:    vk.PipelineBindPointGraphics,
			ColorAttachmentCount: 1,
			PColorAttachments: []vk.AttachmentReference{{
				Attachment: 0,
				Layout:     vk.ImageLayoutColorAttachmentOptimal,
			}},
		}},
	}, nil, &renderPass)
	// subpass dependency?
	OrPanic(as.NewError(ret))
	app.uirenderPass = renderPass
}

func (app *Application) prepareuiPipeline() {
	dev := app.Context().Device()

	vs, err := as.LoadShaderModule(dev, fileToBytes("assets/shaders/ui.vert.spv"))
	OrPanic(err)
	fs, err := as.LoadShaderModule(dev, fileToBytes("assets/shaders/ui.frag.spv"))
	OrPanic(err)

	// note this can be shared
	var pipelineCache vk.PipelineCache
	ret := vk.CreatePipelineCache(dev, &vk.PipelineCacheCreateInfo{
		SType: vk.StructureTypePipelineCacheCreateInfo,
	}, nil, &pipelineCache)
	OrPanic(as.NewError(ret))
	app.pipelineCache = pipelineCache

	// Vertex Descriptions needed for glsl shader
	bindingDescriptions := []vk.VertexInputBindingDescription{
		{
			Binding:   0,
			Stride:    uint32(UIVertexSize),
			InputRate: vk.VertexInputRateVertex,
		},
	}

	attributeDescriptions := []vk.VertexInputAttributeDescription{
		{
			Binding:  0,
			Location: 0,
			Format:   vk.FormatR32g32Sfloat, // vec2
			Offset:   uint32(unsafe.Offsetof(UIVertex{}.Position)),
		},
		{
			Binding:  0,
			Location: 1,
			Format:   vk.FormatR32g32Sfloat, // vec2
			Offset:   uint32(unsafe.Offsetof(UIVertex{}.TexCoord)),
		},
	}

	pipelineCreateInfos := []vk.GraphicsPipelineCreateInfo{{
		SType:      vk.StructureTypeGraphicsPipelineCreateInfo,
		Layout:     app.uipipelineLayout,
		RenderPass: app.uirenderPass,

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
			Topology:               vk.PrimitiveTopologyTriangleList,
			PrimitiveRestartEnable: vk.True,
		},
		PRasterizationState: &vk.PipelineRasterizationStateCreateInfo{
			SType:       vk.StructureTypePipelineRasterizationStateCreateInfo,
			PolygonMode: vk.PolygonModeFill,
			CullMode:    vk.CullModeFlags(vk.CullModeBackBit),
			FrontFace:   vk.FrontFaceClockwise,
			LineWidth:   1.0,
		},
		PColorBlendState: &vk.PipelineColorBlendStateCreateInfo{
			SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
			AttachmentCount: 1,
			PAttachments: []vk.PipelineColorBlendAttachmentState{{
				BlendEnable:         vk.True,
				ColorWriteMask:      vk.ColorComponentFlags(vk.ColorComponentRBit) | vk.ColorComponentFlags(vk.ColorComponentGBit) | vk.ColorComponentFlags(vk.ColorComponentBBit) | vk.ColorComponentFlags(vk.ColorComponentABit),
				ColorBlendOp:        vk.BlendOpAdd,
				AlphaBlendOp:        vk.BlendOpAdd,
				SrcColorBlendFactor: vk.BlendFactorSrcAlpha,
				DstColorBlendFactor: vk.BlendFactorOneMinusSrcAlpha,
				SrcAlphaBlendFactor: vk.BlendFactorOne,
				DstAlphaBlendFactor: vk.BlendFactorZero,
			}},
		},
		PMultisampleState: &vk.PipelineMultisampleStateCreateInfo{
			SType: vk.StructureTypePipelineMultisampleStateCreateInfo,
			// SampleShadingEnable: vk.False,
			RasterizationSamples: vk.SampleCount1Bit,
		},
		PViewportState: &vk.PipelineViewportStateCreateInfo{
			SType:         vk.StructureTypePipelineViewportStateCreateInfo,
			ScissorCount:  1,
			ViewportCount: 1,
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
	ret = vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, pipelineCreateInfos, nil, pipeline)
	OrPanic(as.NewError(ret))
	app.uipipeline = pipeline[0]

	vk.DestroyShaderModule(dev, vs, nil)
	vk.DestroyShaderModule(dev, fs, nil)
}

func (app *Application) prepareuiDescriptorPool() {
	dev := app.Context().Device()
	uiswapchainImageResources := app.Context().UISwapchainImageResources()
	var uidescPool vk.DescriptorPool
	ret := vk.CreateDescriptorPool(dev, &vk.DescriptorPoolCreateInfo{
		SType:         vk.StructureTypeDescriptorPoolCreateInfo,
		MaxSets:       uint32(len(uiswapchainImageResources)),
		PoolSizeCount: 1,
		PPoolSizes: []vk.DescriptorPoolSize{{
			Type:            vk.DescriptorTypeCombinedImageSampler,
			DescriptorCount: uint32(len(uiswapchainImageResources) * len(app.fontImages)),
		}},
	}, nil, &uidescPool)
	OrPanic(as.NewError(ret))
	app.uidescPool = uidescPool
}

func (app *Application) prepareuiDescriptorSet() {
	dev := app.Context().Device()
	texInfos2 := make([]vk.DescriptorImageInfo, 0, len(app.uiTextures))
	for _, tex := range app.uiTextures {
		texInfos2 = append(texInfos2, vk.DescriptorImageInfo{
			Sampler:     tex.sampler, // diff
			ImageView:   tex.view,
			ImageLayout: vk.ImageLayoutShaderReadOnlyOptimal,
		})
	}
	uiswapchainImageResources := app.Context().UISwapchainImageResources()
	for _, res := range uiswapchainImageResources {
		var set vk.DescriptorSet
		ret := vk.AllocateDescriptorSets(dev, &vk.DescriptorSetAllocateInfo{
			SType:              vk.StructureTypeDescriptorSetAllocateInfo,
			DescriptorPool:     app.uidescPool,
			DescriptorSetCount: 1,
			PSetLayouts:        []vk.DescriptorSetLayout{app.uidescLayout},
		}, &set)
		OrPanic(as.NewError(ret))

		res.SetDescriptorSet(set)

		vk.UpdateDescriptorSets(dev, 1, []vk.WriteDescriptorSet{{ // diff
			SType:           vk.StructureTypeWriteDescriptorSet,
			DstBinding:      0,
			DstSet:          set,
			DescriptorCount: uint32(len(app.fontImages)),
			DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
			PImageInfo:      texInfos2,
		}}, 0, nil)
	}
}

func (app *Application) prepareuiFramebuffers() {
	dev := app.Context().Device()
	uiswapchainImageResources := app.Context().UISwapchainImageResources()
	for _, res := range uiswapchainImageResources {
		var uifb vk.Framebuffer
		ret := vk.CreateFramebuffer(dev, &vk.FramebufferCreateInfo{
			SType:           vk.StructureTypeFramebufferCreateInfo,
			RenderPass:      app.uirenderPass,
			AttachmentCount: 1,
			PAttachments: []vk.ImageView{
				res.View(),
			},
			Width:  app.width,
			Height: app.height,
			Layers: 1,
		}, nil, &uifb)
		OrPanic(as.NewError(ret))
		res.SetFramebuffer(uifb)
	}
}

func (app *Application) updateuiCommandBuffer(res *as.UISwapchainImageResources, cmd vk.CommandBuffer) {
	// Begin command buffer
	ret := vk.BeginCommandBuffer(cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit),
	})
	OrPanic(as.NewError(ret))

	// Begin render pass
	vk.CmdBeginRenderPass(cmd, &vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  app.uirenderPass,
		Framebuffer: res.Framebuffer(),
		RenderArea: vk.Rect2D{
			Offset: vk.Offset2D{X: 0, Y: 0},
			Extent: vk.Extent2D{Width: app.width, Height: app.height},
		},
		ClearValueCount: 0,
	}, vk.SubpassContentsInline)

	// Bind pipeline and descriptor sets
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.uipipeline)
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.uipipelineLayout,
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

	vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{app.vertexBuffer}, []vk.DeviceSize{0})
	vk.CmdBindIndexBuffer(cmd, app.indexBuffer, 0, vk.IndexTypeUint32)
	vk.CmdDrawIndexed(cmd, uint32(len(app.indices)), 1, 0, 0, 0)

	// End render pass
	vk.CmdEndRenderPass(cmd)

	// End command buffer
	ret = vk.EndCommandBuffer(cmd)
	OrPanic(as.NewError(ret))
}

func (app *Application) PrepareUIVulkan() error {
	app.prepareuiDescriptorLayout()
	app.prepareuiRenderPass()
	app.prepareuiPipeline()
	app.prepareuiDescriptorPool()
	app.prepareuiDescriptorSet()
	app.prepareuiFramebuffers()

	uiSwapResources := app.Context().UISwapchainImageResources()
	for i := range uiSwapResources {
		app.updateuiCommandBuffer(uiSwapResources[i], uiSwapResources[i].CommandBuffer())
	}
	return nil
}

func (app *Application) VulkanuiContextCleanup() error {
	c := app.Context()
	dev := c.Device()
	vk.DestroyDescriptorPool(dev, app.uidescPool, nil)
	vk.DestroyPipeline(dev, app.uipipeline, nil)
	vk.DestroyRenderPass(dev, app.uirenderPass, nil)
	vk.DestroyPipelineLayout(dev, app.uipipelineLayout, nil)
	vk.DestroyDescriptorSetLayout(dev, app.uidescLayout, nil)

	// TODO more to destroy here?
	for i := 0; i < len(app.uiTextures); i++ {
		app.uiTextures[i].stagingbuf.Destroy()
		app.uiTextures[i].Destroy(dev)
	}

	// destroy vertex and index buffers
	vk.DestroyBuffer(dev, app.vertexBuffer, nil)
	vk.FreeMemory(dev, app.vertexBufferMemory, nil)

	vk.DestroyBuffer(dev, app.indexBuffer, nil)
	vk.FreeMemory(dev, app.indexBufferMemory, nil)

	return nil
}

func loaduiTextureData(img image.Image, rowPitch int) ([]byte, int, int, error) {
	// Ensure the image is in RGBA format
	rgbaImg, ok := img.(*image.RGBA)
	if !ok {
		// Convert image to RGBA if it's not already in that format
		bounds := img.Bounds()
		rgbaImg = image.NewRGBA(bounds)
		draw.Draw(rgbaImg, bounds, img, image.Point{}, draw.Src)
	}

	// Adjust stride if necessary
	if rowPitch <= 4*rgbaImg.Bounds().Dy() {
		rgbaImg.Stride = rowPitch
	}

	// Extract pixel data
	data := rgbaImg.Pix
	width := rgbaImg.Bounds().Dx()
	height := rgbaImg.Bounds().Dy()

	return data, width, height, nil
}
