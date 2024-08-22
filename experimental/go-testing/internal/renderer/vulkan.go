package renderer

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strconv"
	"time"
	"unsafe"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"

	// as "github.com/vulkan-go/asche"
	as "go-testing/asche2"

	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

const (
	HGT_PATH         = "assets/heightmaps/n48w123.png"     // TODO hardcoded
	WATER_PATH       = "assets/heightmaps/n48w123.swb.png" // TODO hardoced
	FOV              = 60.0                                // Field Of View in Degrees
	CULLING_INTERVAL = 2                                   // Perform culling every x frames
)

var texEnabled = []string{
	"assets/textures/test.png",
}

func NewApplication() *Application {
	camera := NewCamera()
	frustum := &Frustum{}

	app := &Application{
		camera:        camera,
		frustum:       frustum,
		visibleChunks: make([]*Chunk, 1000),
		chunks:        make(map[string]*Chunk),
	}

	return app
}

type Application struct {
	as.BaseVulkanApp

	width      uint32
	height     uint32
	format     vk.Format
	colorSpace vk.ColorSpace

	textures          []*Texture
	uiTextures        []*Texture
	depth             *Depth
	useStagingBuffers bool

	descPool   vk.DescriptorPool
	uidescPool vk.DescriptorPool

	pipelineLayout   vk.PipelineLayout
	uipipelineLayout vk.PipelineLayout
	descLayout       vk.DescriptorSetLayout
	uidescLayout     vk.DescriptorSetLayout
	pipelineCache    vk.PipelineCache
	renderPass       vk.RenderPass
	uirenderPass     vk.RenderPass
	pipeline         vk.Pipeline
	uipipeline       vk.Pipeline

	frameIndex int
	frameCount int

	chunks       map[string]*Chunk
	loadedChunks []*Chunk
	heightmap    *image.Gray16
	watermap     *image.Gray
	ubo          UniformBufferObject

	vertices           []UIVertex
	indices            []Index
	vertexBuffer       vk.Buffer
	vertexBufferMemory vk.DeviceMemory
	indexBuffer        vk.Buffer
	indexBufferMemory  vk.DeviceMemory
	fontImages         []image.Image
	font               *truetype.Font
	click              bool
	lastClickTime      time.Time

	camera        *Camera
	frustum       *Frustum
	visibleChunks []*Chunk
	WindowHandle  *glfw.Window
}

func (app *Application) initUBO() {
	aspectRatio := float32(app.width) / float32(app.height)
	// 45 FOV, Aspect 1.0, Near plane, Far Plane
	app.ubo.Projection.Perspective(lin.DegreesToRadians(FOV), aspectRatio, 0.1, 4000.0)
	app.ubo.View = app.camera.GetViewMatrix()
	app.ubo.Model.Identity()
	app.ubo.Projection[1][1] *= -1
	temp := lin.Vec3{0, 0.1, 0.1}
	app.ubo.LightDir.Norm(&temp)
}

func (app *Application) prepareDepth() {
	dev := app.Context().Device()
	depthFormat := vk.FormatD16Unorm
	app.depth = &Depth{
		format: depthFormat,
	}
	ret := vk.CreateImage(dev, &vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    depthFormat,
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
		Format: depthFormat,
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

func (app *Application) prepareTextureImage(path string, img image.Image, tiling vk.ImageTiling,
	usage vk.ImageUsageFlagBits, memoryProps vk.MemoryPropertyFlagBits) *Texture {

	dev := app.Context().Device()
	texFormat := vk.FormatR8g8b8a8Unorm
	var width, height int
	var err error
	if path != "" {
		_, width, height, err = loadTextureData(path, 0)
	} else {
		_, width, height, err = loaduiTextureData(img, 0)
	}
	if err != nil {
		OrPanic(err)
	}
	tex := &Texture{
		texWidth:    int32(width),
		texHeight:   int32(height),
		imageLayout: vk.ImageLayoutShaderReadOnlyOptimal, // FIXME: Possible mismatch with ImageLayout: vk.ImageLayoutGeneral
	}

	var image vk.Image
	ret := vk.CreateImage(dev, &vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    texFormat,
		Extent: vk.Extent3D{
			Width:  uint32(width),
			Height: uint32(height),
			Depth:  1,
		},
		MipLevels:     1,
		ArrayLayers:   1,
		Samples:       vk.SampleCount1Bit,
		Tiling:        tiling,
		Usage:         vk.ImageUsageFlags(usage),
		InitialLayout: vk.ImageLayoutPreinitialized,
	}, nil, &image)
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

		var data []byte
		var err error
		if path != "" {
			data, _, _, err = loadTextureData(path, int(layout.RowPitch))
		} else {
			data, _, _, err = loaduiTextureData(img, int(layout.RowPitch))
		}
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
		// make sure anything that was copying from this image has completed
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
		0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{imageMemoryBarrier})
}

func (app *Application) getGPUName(gpu vk.PhysicalDevice) string {
	var devProperties vk.PhysicalDeviceProperties
	vk.GetPhysicalDeviceProperties(gpu, &devProperties)
	devProperties.Deref()

	return vk.ToString(devProperties.DeviceName[:])
}

func (app *Application) prepareTextures() {
	dev := app.Context().Device()
	texFormat := vk.FormatR8g8b8a8Unorm
	var props vk.FormatProperties
	gpu := app.Context().Platform().PhysicalDevice()
	vk.GetPhysicalDeviceFormatProperties(gpu, texFormat, &props)
	props.Deref()

	fmt.Printf("GPU: %v\n", app.getGPUName(gpu))
	prepareTex := func(path string, img image.Image) *Texture {
		var tex *Texture

		if (props.LinearTilingFeatures&vk.FormatFeatureFlags(vk.FormatFeatureSampledImageBit) != 0) &&
			!app.useStagingBuffers {
			// -> device can texture using linear textures

			tex = app.prepareTextureImage(path, img, vk.ImageTilingLinear, vk.ImageUsageSampledBit,
				vk.MemoryPropertyHostVisibleBit|vk.MemoryPropertyHostCoherentBit)

			// Nothing in the pipeline needs to be complete to start, and don't allow fragment
			// shader to run until layout transition completes
			app.setImageLayout(tex.image, vk.ImageAspectColorBit,
				vk.ImageLayoutPreinitialized, tex.imageLayout,
				vk.AccessHostWriteBit,
				vk.PipelineStageTopOfPipeBit, vk.PipelineStageFragmentShaderBit)

		} else if props.OptimalTilingFeatures&vk.FormatFeatureFlags(vk.FormatFeatureSampledImageBit) != 0 {
			//  Must use staging buffer to copy linear texture to optimized
			log.Println("vulkan warn: using staging buffers")

			staging := app.prepareTextureImage(path, img, vk.ImageTilingLinear, vk.ImageUsageTransferSrcBit,
				vk.MemoryPropertyHostVisibleBit|vk.MemoryPropertyHostCoherentBit)
			tex = app.prepareTextureImage(path, img, vk.ImageTilingOptimal,
				vk.ImageUsageTransferDstBit|vk.ImageUsageSampledBit, vk.MemoryPropertyDeviceLocalBit)

			app.setImageLayout(staging.image, vk.ImageAspectColorBit,
				vk.ImageLayoutPreinitialized, vk.ImageLayoutTransferSrcOptimal,
				vk.AccessHostWriteBit,
				vk.PipelineStageTopOfPipeBit, vk.PipelineStageTransferBit)

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
			// todo fix this???
		} else {
			OrPanic(errors.New("vulkan: R8G8B8A8_UNORM not supported as texture image format"))
		}

		var sampler vk.Sampler // nearest -> linear?
		ret := vk.CreateSampler(dev, &vk.SamplerCreateInfo{
			SType:                   vk.StructureTypeSamplerCreateInfo,
			MagFilter:               vk.FilterNearest,
			MinFilter:               vk.FilterNearest,
			MipmapMode:              vk.SamplerMipmapModeNearest,
			AddressModeU:            vk.SamplerAddressModeRepeat,
			AddressModeV:            vk.SamplerAddressModeRepeat,
			AddressModeW:            vk.SamplerAddressModeRepeat,
			AnisotropyEnable:        vk.False,
			MaxAnisotropy:           1,
			CompareOp:               vk.CompareOpNever,
			BorderColor:             vk.BorderColorFloatOpaqueWhite,
			UnnormalizedCoordinates: vk.False,
			// minLod, maxLod?
		}, nil, &sampler)
		OrPanic(as.NewError(ret))
		tex.sampler = sampler

		var view vk.ImageView
		ret = vk.CreateImageView(dev, &vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			Image:    tex.image,
			ViewType: vk.ImageViewType2d,
			Format:   texFormat,
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

		if img != nil {
			memProps := app.Context().Platform().MemoryProperties()
			tex.stagingbuf = as.CreateBuffer(dev, memProps, img.(*image.RGBA).Pix, vk.BufferUsageTransferSrcBit)
		}
		return tex
	}

	app.textures = make([]*Texture, 0, len(texEnabled))
	for _, texFile := range texEnabled {
		app.textures = append(app.textures, prepareTex(texFile, nil))
	}

	// app.uiTextures = make([]*Texture, 0, len(uitexEnabled))
	// for _, texFile := range uitexEnabled {
	// 	app.uiTextures = append(app.uiTextures, prepareTex(texFile, nil))
	// }
	app.uiTextures = make([]*Texture, 0, len(app.fontImages))
	for _, img := range app.fontImages {
		app.uiTextures = append(app.uiTextures, prepareTex("", img))
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
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit),
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
		PSetLayouts: []vk.DescriptorSetLayout{
			app.descLayout,
		},
	}, nil, &pipelineLayout)
	OrPanic(as.NewError(ret))
	app.pipelineLayout = pipelineLayout
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

func (app *Application) prepareRenderPass() {
	dev := app.Context().Device()
	// The initial layout for the color and depth attachments will be vk.LayoutUndefined
	// because at the start of the renderpass, we don't care about their contents.
	// At the start of the subpass, the color attachment's layout will be transitioned
	// to vk.LayoutColorAttachmentOptimal and the depth stencil attachment's layout
	// will be transitioned to vk.LayoutDepthStencilAttachmentOptimal. At the end of
	// the renderpass, the color attachment's layout will be transitioned to
	// vk.LayoutPresentSrc to be ready to present. This is all done as part of
	// the renderpass, no barriers are necessary.
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
			FinalLayout:    vk.ImageLayoutColorAttachmentOptimal,
		}, {
			Format:         app.depth.format,
			Samples:        vk.SampleCount1Bit,
			LoadOp:         vk.AttachmentLoadOpClear,
			StoreOp:        vk.AttachmentStoreOpDontCare,
			StencilLoadOp:  vk.AttachmentLoadOpDontCare,
			StencilStoreOp: vk.AttachmentStoreOpDontCare,
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

func (app *Application) preparePipeline() {
	dev := app.Context().Device()

	vs, err := as.LoadShaderModule(dev, fileToBytes("assets/shaders/cube.vert.spv"))
	OrPanic(err)
	fs, err := as.LoadShaderModule(dev, fileToBytes("assets/shaders/cube.frag.spv"))
	OrPanic(err)

	// note .cpp does not have a pipelinecache
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
			Stride:    uint32(VertexSize),
			InputRate: vk.VertexInputRateVertex,
		},
	}

	// These are correct
	attributeDescriptions := []vk.VertexInputAttributeDescription{
		{
			Binding:  0,
			Location: 0,
			Format:   vk.FormatR32g32b32Sfloat, // vec3
			Offset:   uint32(unsafe.Offsetof(Vertex{}.Position)),
		},
		{
			Binding:  0,
			Location: 1,
			Format:   vk.FormatR32g32b32Sfloat, // vec3
			Offset:   uint32(unsafe.Offsetof(Vertex{}.TexCoord)),
		},
		{
			Binding:  0,
			Location: 2,
			Format:   vk.FormatR32g32b32Sfloat, // vec3
			Offset:   uint32(unsafe.Offsetof(Vertex{}.Normal)),
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
			SType: vk.StructureTypePipelineInputAssemblyStateCreateInfo,
			// Topology: vk.PrimitiveTopologyTriangleList,
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
			SType: vk.StructureTypePipelineMultisampleStateCreateInfo,
			// SampleShadingEnable: vk.False,
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
	ret = vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, pipelineCreateInfos, nil, pipeline)
	OrPanic(as.NewError(ret))
	app.pipeline = pipeline[0]

	vk.DestroyShaderModule(dev, vs, nil)
	vk.DestroyShaderModule(dev, fs, nil)
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

	uiswapchainImageResources := app.Context().UISwapchainImageResources()
	var uidescPool vk.DescriptorPool
	ret = vk.CreateDescriptorPool(dev, &vk.DescriptorPoolCreateInfo{
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

func (app *Application) prepareDescriptorSet() {
	dev := app.Context().Device()
	swapchainImageResources := app.Context().SwapchainImageResources()
	texInfos := make([]vk.DescriptorImageInfo, 0, len(app.textures))
	for _, tex := range app.textures {
		texInfos = append(texInfos, vk.DescriptorImageInfo{
			Sampler:     tex.sampler,
			ImageView:   tex.view,
			ImageLayout: vk.ImageLayoutGeneral, // FIXME: Possible mismatch with tex.imageLayout = vk.ImageLayoutShaderReadOnlyOptimal
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

	// and for ui
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

func (app *Application) prepareFramebuffers() {
	dev := app.Context().Device()
	swapchainImageResources := app.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		var fb vk.Framebuffer
		ret := vk.CreateFramebuffer(dev, &vk.FramebufferCreateInfo{
			SType:           vk.StructureTypeFramebufferCreateInfo,
			RenderPass:      app.renderPass,
			AttachmentCount: 2,
			PAttachments: []vk.ImageView{
				res.View(),
				app.depth.view,
			},
			Width:  app.width,
			Height: app.height,
			Layers: 1,
		}, nil, &fb)
		OrPanic(as.NewError(ret))
		res.SetFramebuffer(fb)
	}

	// and for ui
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

func (app *Application) LoadAllChunks() {
	if app.heightmap == nil {
		panic("Heightmap not loaded")
	}

	totalChunks := (length / ChunkSize) * (length / ChunkSize)
	fmt.Printf("Loading %d chunks...\n", totalChunks)

	for z := 0; z < length/ChunkSize; z++ {
		for x := 0; x < length/ChunkSize; x++ {
			app.LoadChunk(x, z)
			if (z*length/ChunkSize+x+1)%100 == 0 {
				fmt.Printf("Loaded %d/%d chunks\n", z*length/ChunkSize+x+1, totalChunks)
			}
		}
	}

	fmt.Printf("All %d chunks loaded.\n", totalChunks)
}

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
	// bgc := color.RGBA{0, 0, 0, 128}
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

func (app *Application) updateText(str string, size float64, width, height, quad int) {
	dev := app.Context().Device()
	bgc := color.RGBA{0, 0, 0, 128}
	if app.click && quad == 1 {
		bgc = color.RGBA{0, 0, 0, 1}
	}
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

func (app *Application) prepareuiQuad() {
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

func (app *Application) VulkanContextPrepare() error {
	dim := app.Context().SwapchainDimensions()
	app.height = dim.Height
	app.width = dim.Width

	app.initUBO()
	heightmap, err := GetHM(HGT_PATH)
	OrPanic(err)
	app.heightmap = heightmap
	watermap, err := GetWatermap(WATER_PATH)
	OrPanic(err)
	app.watermap = watermap

	app.font = prepareFont("assets/SourceSansPro-Regular.ttf")
	text := renderTextToBitmap(app.font, "X", 72, 800, 600, color.RGBA{0, 0, 0, 128})
	app.fontImages = append(app.fontImages, text)
	text2 := renderTextToBitmap(app.font, "click", 72, 200, 125, color.RGBA{0, 0, 0, 128})
	app.fontImages = append(app.fontImages, text2)
	// look at
	//	descriptorTypeStorageBufferDynamic
	//	descriptorTypeUniformBufferDynamic
	//	GraphicsPipelineCreateInfo.PDynamicState
	//	PipelineDynamicStateCreateFlags

	app.prepareDepth()
	app.prepareTextures()

	app.initUIDimensions()
	app.prepareuiQuad()

	// Load all available chunks for now
	app.LoadAllChunks()

	app.frustum.Update(&app.ubo.View, &app.ubo.Projection)
	app.determineVisibleChunks()

	app.prepareUniformBuffers() // need to be called after loadhm()
	app.prepareDescriptorLayout()
	app.prepareuiDescriptorLayout()
	app.prepareRenderPass()
	app.prepareuiRenderPass()
	app.preparePipeline()
	app.prepareuiPipeline()
	app.prepareDescriptorPool()
	app.prepareDescriptorSet()
	app.prepareFramebuffers()

	swapchainImageResources := app.Context().SwapchainImageResources()
	uiSwapResources := app.Context().UISwapchainImageResources()
	for i, res := range swapchainImageResources {
		app.updateCommandBuffer(res, res.CommandBuffer())
		app.updateuiCommandBuffer(uiSwapResources[i], uiSwapResources[i].CommandBuffer())
	}
	fmt.Printf("done preparing\n")
	return nil
}

func (app *Application) VulkanContextCleanup() error {
	app.click = false
	app.lastClickTime = time.Now()
	c := app.Context()
	dev := c.Device()
	vk.DestroyDescriptorPool(dev, app.descPool, nil)
	vk.DestroyDescriptorPool(dev, app.uidescPool, nil)
	vk.DestroyPipeline(dev, app.pipeline, nil)
	vk.DestroyPipelineCache(dev, app.pipelineCache, nil)
	vk.DestroyRenderPass(dev, app.renderPass, nil)
	vk.DestroyRenderPass(dev, app.uirenderPass, nil)
	vk.DestroyPipelineLayout(dev, app.pipelineLayout, nil)
	vk.DestroyPipelineLayout(dev, app.uipipelineLayout, nil)
	vk.DestroyDescriptorSetLayout(dev, app.descLayout, nil)
	vk.DestroyDescriptorSetLayout(dev, app.uidescLayout, nil)

	// TODO more to destroy here?

	for i := 0; i < len(app.textures); i++ {
		app.textures[i].Destroy(dev)
	}
	for i := 0; i < len(app.uiTextures); i++ {
		app.uiTextures[i].stagingbuf.Destroy()
		app.uiTextures[i].Destroy(dev)
	}

	for _, chunk := range app.chunks {
		vk.DestroyBuffer(dev, chunk.vertexBuffer, nil)
		vk.FreeMemory(dev, chunk.vertexMemory, nil)
		vk.DestroyBuffer(dev, chunk.indexBuffer, nil)
		vk.FreeMemory(dev, chunk.indexMemory, nil)
	}

	// destroy vertex and index buffers
	vk.DestroyBuffer(dev, app.vertexBuffer, nil)
	vk.FreeMemory(dev, app.vertexBufferMemory, nil)

	vk.DestroyBuffer(dev, app.indexBuffer, nil)
	vk.FreeMemory(dev, app.indexBufferMemory, nil)

	app.depth.Destroy(dev)
	return nil
}

func (app *Application) NextFrame(fps int) {
	app.frameCount++

	// Process Input and update the camera
	app.camera.ProcessInput(app.WindowHandle)
	app.ubo.View = app.camera.GetViewMatrix()
	app.CheckMouse(app.WindowHandle)

	// Update the view frustum and visibleChunks
	if app.frameCount%CULLING_INTERVAL == 0 {
		app.frustum.Update(&app.ubo.View, &app.ubo.Projection)
		app.determineVisibleChunks()
	}
	app.updateText("fps: "+strconv.Itoa(fps), 72, 800, 600, 0)
	if app.click {
		app.updateText("on", 72, 200, 125, 1)
	} else {
		app.updateText("off", 72, 200, 125, 1)
	}
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

	// update ui state

	// New command buffer update logic
	cmd := res.CommandBuffer()
	app.updateCommandBuffer(res, cmd)

	res2 := app.Context().UISwapchainImageResources()[imageIdx]
	cmd2 := res2.CommandBuffer()
	app.updateuiCommandBuffer(res2, cmd2)

	return nil
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

func (app *Application) updateCommandBuffer(res *as.SwapchainImageResources, cmd vk.CommandBuffer) {
	// Begin command buffer
	ret := vk.BeginCommandBuffer(cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit),
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

	// Draw visible chunks
	for _, chunk := range app.visibleChunks {
		vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{chunk.vertexBuffer}, []vk.DeviceSize{0})
		vk.CmdBindIndexBuffer(cmd, chunk.indexBuffer, 0, vk.IndexTypeUint32)
		vk.CmdDrawIndexed(cmd, uint32(len(chunk.indices)), 1, 0, 0, 0)
	}

	// End render pass
	vk.CmdEndRenderPass(cmd)

	// End command buffer
	ret = vk.EndCommandBuffer(cmd)
	OrPanic(as.NewError(ret))
}

func (app *Application) Destroy() {}

type Texture struct {
	sampler vk.Sampler

	image       vk.Image
	imageLayout vk.ImageLayout

	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView

	texWidth  int32
	texHeight int32

	stagingbuf *as.Buffer
}

func (t *Texture) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, t.view, nil)
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroyImage(dev, t.image, nil)
	vk.DestroySampler(dev, t.sampler, nil)
}

func (t *Texture) DestroyImage(dev vk.Device) {
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroyImage(dev, t.image, nil)
}

type Depth struct {
	format   vk.Format
	image    vk.Image
	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView
}

func (d *Depth) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, d.view, nil)
	vk.DestroyImage(dev, d.image, nil)
	vk.FreeMemory(dev, d.mem, nil)
}

// func loadTextureSize(name string) (w int, h int, err error) {
// 	data := MustAsset(name)
// 	r := bytes.NewReader(data)
// 	ppmCfg, err := ppm.DecodeConfig(r)
// 	if err != nil {
// 		return 0, 0, err
// 	}
// 	return ppmCfg.Width, ppmCfg.Height, nil
// }

// func loadTextureData(name string, layout vk.SubresourceLayout) ([]byte, error) {
// 	data := MustAsset(name)
// 	r := bytes.NewReader(data)
// 	img, err := ppm.Decode(r)
// 	if err != nil {
// 		return nil, err
// 	}
// 	newImg := image.NewRGBA(img.Bounds())
// 	newImg.Stride = int(layout.RowPitch)
// 	draw.Draw(newImg, newImg.Bounds(), img, image.ZP, draw.Src)
// 	return []byte(newImg.Pix), nil
// }

func loadTextureData(name string, rowPitch int) ([]byte, int, int, error) {
	data := fileToBytes(name)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	newImg := image.NewRGBA(img.Bounds())
	if rowPitch <= 4*img.Bounds().Dy() {
		// apply the proposed row pitch only if supported,
		// as we're using only optimal textures.
		newImg.Stride = rowPitch
	}
	draw.Draw(newImg, newImg.Bounds(), img, image.Point{}, draw.Src)
	size := newImg.Bounds().Size()
	return []byte(newImg.Pix), size.X, size.Y, nil
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

// Converts the given file to a byte array to be used
func fileToBytes(filename string) []byte {
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
