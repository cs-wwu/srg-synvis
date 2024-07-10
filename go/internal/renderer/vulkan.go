package renderer

// Note that most of this code was pulled from the vulkancube demo at
// https://github.com/vulkan-go/demos/
import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"unsafe"
	"os"

	"github.com/cs-wwu/srg-synvis/go/internal/types"
	"github.com/cs-wwu/srg-synvis/go/pkg/utils"
	"github.com/go-gl/glfw/v3.3/glfw"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

var texEnabled = []string{ //TODO check if this path works
	"assets/textures/checkers.png",
}

var (
	fragPath = "assets/shaders/shader.frag.spv"
	vertPath = "assets/shaders/shader.vert.spv"
)

// will be expanded
// here instead of types/common.go because "you cannot define a method on a type that is not declared in the same package where the method is being defined"
type Application struct {
	as.BaseVulkanApp

	Window             *glfw.Window
	FramebufferResized bool

	// swapchain extent values
	width  uint32
	height uint32

	// using example
	format     vk.Format
	colorSpace vk.ColorSpace

	textures          []*Texture
	depth             *Depth
	useStagingBuffers bool

	descPool vk.DescriptorPool

	pipelineLayout vk.PipelineLayout
	descLayout     vk.DescriptorSetLayout
	pipelineCache  vk.PipelineCache
	renderPass     vk.RenderPass
	pipeline       vk.Pipeline

	frameIndex int
	
	vertexBuffer   vk.Buffer
	indexBuffer    vk.Buffer
	indices 	   []uint32

	projectionMatrix lin.Mat4x4
	viewMatrix       lin.Mat4x4
	modelMatrix      lin.Mat4x4

	eyeVec    *lin.Vec3
	originVec *lin.Vec3
	upVec     *lin.Vec3

	spinAngle float32

	WindowHandle *glfw.Window // TODO may be unecessary
}

type Texture struct {
	sampler vk.Sampler

	image       vk.Image
	imageLayout vk.ImageLayout

	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView

	texWidth  int32
	texHeight int32
}

type Depth struct {
	format   vk.Format
	image    vk.Image
	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView
}

// Creates a new Application and returns a pointer to it
func NewApplication(spinAngle float32) *Application {
	app := &Application{
		spinAngle: spinAngle,
		eyeVec:    &lin.Vec3{0.0, 3.0, 5.0},
		originVec: &lin.Vec3{0.0, 0.0, 0.0},
		upVec:     &lin.Vec3{0.0, 1.0, 0.0},
	}

	app.projectionMatrix.Perspective(lin.DegreesToRadians(45.0), 1.0, 0.1, 100.0)
	app.viewMatrix.LookAt(app.eyeVec, app.originVec, app.upVec)
	app.modelMatrix.Identity()
	app.projectionMatrix[1][1] *= -1 // Flip projection matrix from GL to Vulkan orientation.
	
	return app
}

// for window resizing (not necessary on rpi)
func framebufferSizeCallback(window *glfw.Window, width int, height int) {
	appPtr := window.GetUserPointer()
	if appPtr == nil {
		return
	}
	app := (*types.Application)(appPtr)
	app.FramebufferResized = true
}

func InitWindow(windowWidth, windowHeight int) *types.Application {
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Vulkan", nil, nil)
	if err != nil {
		panic(fmt.Errorf("glfw window creation failed: %v", err))
	}
	// window is resizeable + detection of resize
	app := &types.Application{Window: window}

	ptr := unsafe.Pointer(app)
	window.SetUserPointer(ptr)

	window.SetFramebufferSizeCallback(framebufferSizeCallback)

	return app
}

// main.cpp createDepthResources -> findDepthFormat -> findSupportedFormat -> createImage -> createImageView
func (s *Application) prepareDepth() {
	dev := s.Context().Device()
	// note: main.cpp calls findSupportedFormat for this
	depthFormat := vk.FormatD16Unorm
	s.depth = &Depth{
		format: depthFormat,
	}

	ret := vk.CreateImage(dev, &vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    depthFormat,
		Extent: vk.Extent3D{
			Width:  s.width,
			Height: s.height,
			Depth:  1,
		},
		MipLevels:   1,
		ArrayLayers: 1,
		Samples:     vk.SampleCount1Bit,
		Tiling:      vk.ImageTilingOptimal, // difference
		Usage:       vk.ImageUsageFlags(vk.ImageUsageDepthStencilAttachmentBit),
		// imageInfo.samples = vk.SampleCount1Bit,
		// imageInfo.sharingMode = vk.SharingModeExclusive;
	}, nil, &s.depth.image)
	utils.OrPanic(as.NewError(ret))

	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(dev, s.depth.image, &memReqs)
	memReqs.Deref() // used for unsafe ptrs...

	memProps := s.Context().Platform().MemoryProperties()
	memTypeIndex, _ := as.FindRequiredMemoryTypeFallback(memProps,
		vk.MemoryPropertyFlagBits(memReqs.MemoryTypeBits), vk.MemoryPropertyDeviceLocalBit)
	s.depth.memAlloc = &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memTypeIndex,
	}

	var mem vk.DeviceMemory
	ret = vk.AllocateMemory(dev, s.depth.memAlloc, nil, &mem)
	utils.OrPanic(as.NewError(ret))
	s.depth.mem = mem

	ret = vk.BindImageMemory(dev, s.depth.image, s.depth.mem, 0)
	utils.OrPanic(as.NewError(ret))

	var view vk.ImageView
	ret = vk.CreateImageView(dev, &vk.ImageViewCreateInfo{
		SType:  vk.StructureTypeImageViewCreateInfo,
		Format: depthFormat,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectDepthBit),
			LevelCount: 1,
			LayerCount: 1,
			// baseMipLevel == 0,
			// baseArrayLayer = 0,
		},
		ViewType: vk.ImageViewType2d,
		Image:    s.depth.image,
	}, nil, &view)
	utils.OrPanic(as.NewError(ret))
	s.depth.view = view
}

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
	draw.Draw(newImg, newImg.Bounds(), img, image.ZP, draw.Src)
	size := newImg.Bounds().Size()
	return []byte(newImg.Pix), size.X, size.Y, nil
}

func (app *Application) prepareTextureImage(path string, tiling vk.ImageTiling,
	usage vk.ImageUsageFlagBits, memoryProps vk.MemoryPropertyFlagBits) *Texture {

	dev := app.Context().Device()
	texFormat := vk.FormatR8g8b8a8Unorm
	_, width, height, err := loadTextureData(path, 0)
	if err != nil {
		utils.OrPanic(err)
	}
	tex := &Texture{
		texWidth:    int32(width),
		texHeight:   int32(height),
		imageLayout: vk.ImageLayoutShaderReadOnlyOptimal,
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
	utils.OrPanic(as.NewError(ret))
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
	utils.OrPanic(as.NewError(ret))
	tex.mem = mem
	ret = vk.BindImageMemory(dev, tex.image, tex.mem, 0)
	utils.OrPanic(as.NewError(ret))

	hostVisible := memoryProps&vk.MemoryPropertyHostVisibleBit != 0
	if hostVisible {
		var layout vk.SubresourceLayout
		vk.GetImageSubresourceLayout(dev, tex.image, &vk.ImageSubresource{
			AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
		}, &layout)
		layout.Deref()

		data, _, _, err := loadTextureData(path, int(layout.RowPitch))
		utils.OrPanic(err)
		if len(data) > 0 {
			var pData unsafe.Pointer
			ret = vk.MapMemory(dev, tex.mem, 0, vk.DeviceSize(len(data)), 0, &pData)
			if isError(ret) {
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
		utils.OrPanic(errors.New("vulkan: command buffer not initialized"))
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

func (app *Application) prepareTextures() {
	dev := app.Context().Device()
	texFormat := vk.FormatR8g8b8a8Unorm
	var props vk.FormatProperties
	gpu := app.Context().Platform().PhysicalDevice()
	vk.GetPhysicalDeviceFormatProperties(gpu, texFormat, &props)
	props.Deref()

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
				vk.PipelineStageTopOfPipeBit, vk.PipelineStageFragmentShaderBit)

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
				vk.PipelineStageTopOfPipeBit, vk.PipelineStageTransferBit)

			app.setImageLayout(tex.image, vk.ImageAspectColorBit,
				vk.ImageLayoutPreinitialized, vk.ImageLayoutTransferDstOptimal,
				vk.AccessHostWriteBit,
				vk.PipelineStageTopOfPipeBit, vk.PipelineStageTransferBit)

			cmd := app.Context().CommandBuffer()
			if cmd == nil {
				utils.OrPanic(errors.New("vulkan: command buffer not initialized"))
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
			utils.OrPanic(errors.New("vulkan: R8G8B8A8_UNORM not supported as texture image format"))
		}

		var sampler vk.Sampler
		ret := vk.CreateSampler(dev, &vk.SamplerCreateInfo{
			SType:                   vk.StructureTypeSamplerCreateInfo,
			MagFilter:               vk.FilterNearest,
			MinFilter:               vk.FilterNearest,
			MipmapMode:              vk.SamplerMipmapModeNearest,
			AddressModeU:            vk.SamplerAddressModeClampToEdge,
			AddressModeV:            vk.SamplerAddressModeClampToEdge,
			AddressModeW:            vk.SamplerAddressModeClampToEdge,
			AnisotropyEnable:        vk.False,
			MaxAnisotropy:           1,
			CompareOp:               vk.CompareOpNever,
			BorderColor:             vk.BorderColorFloatOpaqueWhite,
			UnnormalizedCoordinates: vk.False,
		}, nil, &sampler)
		utils.OrPanic(as.NewError(ret))
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
		utils.OrPanic(as.NewError(ret))
		tex.view = view

		return tex
	}

	app.textures = make([]*Texture, 0, len(texEnabled))
	for _, texFile := range texEnabled {
		app.textures = append(app.textures, prepareTex(texFile))
	}
}

func (app *Application) prepareCubeDataBuffers() {
	dev := app.Context().Device()

	var VP lin.Mat4x4
	var MVP lin.Mat4x4
	VP.Mult(&app.projectionMatrix, &app.viewMatrix)
	MVP.Mult(&VP, &app.modelMatrix)

	data := vkTexCubeUniform{
		mvp: MVP,
	}
	for i := 0; i < 12*3; i++ {
		data.position[i][0] = gVertexBufferData[i*3]
		data.position[i][1] = gVertexBufferData[i*3+1]
		data.position[i][2] = gVertexBufferData[i*3+2]
		data.position[i][3] = 1.0
		data.attr[i][0] = gUVBufferData[2*i]
		data.attr[i][1] = gUVBufferData[2*i+1]
		data.attr[i][2] = 0
		data.attr[i][3] = 0
	}

	dataRaw := data.Data()
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
	utils.OrPanic(as.NewError(ret))
	app.descLayout = descLayout

	var pipelineLayout vk.PipelineLayout
	ret = vk.CreatePipelineLayout(dev, &vk.PipelineLayoutCreateInfo{
		SType:          vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount: 1,
		PSetLayouts: []vk.DescriptorSetLayout{
			app.descLayout,
		},
	}, nil, &pipelineLayout)
	utils.OrPanic(as.NewError(ret))
	app.pipelineLayout = pipelineLayout
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

	// TODO no dependecy struct
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
	utils.OrPanic(as.NewError(ret))
	app.renderPass = renderPass
}

func (app *Application) preparePipeline() {
	dev := app.Context().Device()

	// Shaders MUST be compiled before this is called
	vs, err := as.LoadShaderModule(dev, fileToBytes("shaders/cube.vert.spv"))
	utils.OrPanic(err)
	fs, err := as.LoadShaderModule(dev, fileToBytes("shaders/cube.frag.spv"))
	utils.OrPanic(err)

	var pipelineCache vk.PipelineCache
	ret := vk.CreatePipelineCache(dev, &vk.PipelineCacheCreateInfo{
		SType: vk.StructureTypePipelineCacheCreateInfo,
	}, nil, &pipelineCache)
	utils.OrPanic(as.NewError(ret))
	app.pipelineCache = pipelineCache

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
			SType: vk.StructureTypePipelineVertexInputStateCreateInfo,
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
	ret = vk.CreateGraphicsPipelines(dev, app.pipelineCache, 1, pipelineCreateInfos, nil, pipeline)
	utils.OrPanic(as.NewError(ret))
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
	utils.OrPanic(as.NewError(ret))
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
		utils.OrPanic(as.NewError(ret))

		res.SetDescriptorSet(set)

		vk.UpdateDescriptorSets(dev, 2, []vk.WriteDescriptorSet{{
			SType:           vk.StructureTypeWriteDescriptorSet,
			DstSet:          set,
			DescriptorCount: 1,
			DescriptorType:  vk.DescriptorTypeUniformBuffer,
			PBufferInfo: []vk.DescriptorBufferInfo{{
				Offset: 0,
				Range:  vk.DeviceSize(vkTexCubeUniformSize),
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
			PAttachments: []vk.ImageView{
				res.View(),
				app.depth.view,
			},
			Width:  app.width,
			Height: app.height,
			Layers: 1,
		}, nil, &fb)
		utils.OrPanic(as.NewError(ret))

		res.SetFramebuffer(fb)
	}
}

func (app *Application) VulkanContextPrepare() error {
	dim := app.Context().SwapchainDimensions()
	app.height = dim.Height
	app.width = dim.Width

	app.prepareDepth()
	app.prepareTextures()
	app.prepareCubeDataBuffers() // TODO Only used for cubes?
	app.prepareDescriptorLayout()
	app.prepareRenderPass()
	app.preparePipeline()
	app.prepareDescriptorPool()
	app.prepareDescriptorSet()
	app.prepareFramebuffers()

	swapchainImageResources := app.Context().SwapchainImageResources()
	for _, res := range swapchainImageResources {
		app.drawBuildCommandBuffer(res, res.CommandBuffer())
	}
	return nil
}

// main.cpp recordComandBuffer -> as called by drawFrame?
func (app *Application) drawBuildCommandBuffer(res *as.SwapchainImageResources, cmd vk.CommandBuffer) {
	ret := vk.BeginCommandBuffer(cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit),	// different: no flags
	})
	utils.OrPanic(as.NewError(ret))

	clearValues := make([]vk.ClearValue, 2)
	clearValues[1].SetDepthStencil(1, 0)
	clearValues[0].SetColor([]float32{
		0.2, 0.2, 0.2, 0.2,	// 0.0, 0.0, 0.0, 1.0 for background color
	})
	vk.CmdBeginRenderPass(cmd, &vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  app.renderPass,
		Framebuffer: res.Framebuffer(), // swapChainFramebuffers[imageIndex];
		RenderArea: vk.Rect2D{
			Offset: vk.Offset2D{
				X: 0, Y: 0,
			},
			Extent: vk.Extent2D{
				Width:  app.width,
				Height: app.height,
			},
		},
		ClearValueCount: 2,
		PClearValues:    clearValues,
	}, vk.SubpassContentsInline)
	
	// vkCmdClearAttachments() for background color only?
	
	vk.CmdBindPipeline(cmd, vk.PipelineBindPointGraphics, app.pipeline)
	
	vk.CmdBindDescriptorSets(cmd, vk.PipelineBindPointGraphics, app.pipelineLayout,
		0, 1, []vk.DescriptorSet{res.DescriptorSet()}, 0, nil)
	
	vk.CmdSetViewport(cmd, 0, 1, []vk.Viewport{{
		// x = 0.0,
		// y = 0.0,
		Width:    float32(app.width),
		Height:   float32(app.height),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}})
	vk.CmdSetScissor(cmd, 0, 1, []vk.Rect2D{{
		Offset: vk.Offset2D{
			X: 0, Y: 0,
		},
		Extent: vk.Extent2D{
			Width:  app.width,
			Height: app.height,
		},
	}})
	
	// TODO .cpp code below
	// var vertexBuffers []vk.Buffer
	// vertexBuffers = append(vertexBuffers, s.vertexBuffer)
	// var offsets []vk.DeviceSize
	// offsets = append(offsets, 0)
	// vk.CmdBindVertexBuffers(cmd, 0, 1, vertexBuffers, offsets)
	// vk.CmdBindIndexBuffer(cmd, s.indexBuffer, 0, vk.IndexTypeUint32)

	// vk.CmdDrawIndexed(cmd, uint32(len(s.indices)), 1, 0, 0, 0)
	// end cpp code
	vk.CmdDraw(cmd, 12*3, 1, 0, 0)
	
	// Note that ending the renderpass changes the image's layout from
	// vk.ImageLayoutColorAttachmentOptimal to vk.ImageLayoutPresentSrc
	vk.CmdEndRenderPass(cmd)

	// below code is not in main.cpp -> unless transitionImageLayout? (called in createTextureImage)
	graphicsQueueIndex := app.Context().Platform().GraphicsQueueFamilyIndex()
	presentQueueIndex := app.Context().Platform().PresentQueueFamilyIndex()
	if graphicsQueueIndex != presentQueueIndex {
		// Separate Present Queue Case
		// We have to transfer ownership from the graphics queue family to the
		// present queue family to be able to present.  Note that we don't have
		// to transfer from present queue family back to graphics queue family at
		// the start of the next frame because we don't care about the image's
		// contents at that point.
		vk.CmdPipelineBarrier(cmd,
			vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
			vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit),
			0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{{
				SType:               vk.StructureTypeImageMemoryBarrier,
				SrcAccessMask:       0,
				DstAccessMask:       vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
				OldLayout:           vk.ImageLayoutPresentSrc,
				NewLayout:           vk.ImageLayoutPresentSrc,
				SrcQueueFamilyIndex: graphicsQueueIndex,
				DstQueueFamilyIndex: presentQueueIndex,
				SubresourceRange: vk.ImageSubresourceRange{
					AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
					LayerCount: 1,
					LevelCount: 1,
				},
				Image: res.Image(),
			}})
	}
	ret = vk.EndCommandBuffer(cmd)
	utils.OrPanic(as.NewError(ret))
}

func (app *Application) VulkanContextCleanup() error {
	dev := app.Context().Device()
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
	var Model lin.Mat4x4
	Model.Dup(&app.modelMatrix)
	// Rotate around the Y axis
	app.modelMatrix.Rotate(&Model, 0.0, 1.0, 0.0, lin.DegreesToRadians(app.spinAngle))
}

func (app *Application) VulkanContextInvalidate(imageIdx int) error {
	dev := app.Context().Device()
	res := app.Context().SwapchainImageResources()[imageIdx]

	var MVP, VP lin.Mat4x4
	VP.Mult(&app.projectionMatrix, &app.viewMatrix)
	MVP.Mult(&VP, &app.modelMatrix)

	data := MVP.Data()
	var pData unsafe.Pointer
	ret := vk.MapMemory(dev, res.UniformMemory(), 0, vk.DeviceSize(len(data)), 0, &pData)
	utils.OrPanic(as.NewError(ret))

	n := vk.Memcopy(pData, data)
	if n != len(data) {
		log.Printf("vulkan warning: failed to copy data, %d != %d", n, len(data))
	}
	vk.UnmapMemory(dev, res.UniformMemory())
	return nil
}

func (app *Application) VulkanDeviceExtensions() []string {
	return []string{
		"VK_KHR_swapchain",
	}
}

func (app *Application) VulkanSwapchainDimensions() *as.SwapchainDimensions {
	return &as.SwapchainDimensions{
		Width: 500, Height: 500, Format: vk.FormatB8g8r8a8Unorm,
	}
}

func (app *Application) VulkanInstanceExtensions() []string {
	extensions := app.WindowHandle.GetRequiredInstanceExtensions()
	return extensions
}

func (app *Application) Destroy() {/* TODO Empty when we found it? */}

func (t *Texture) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, t.view, nil)
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroyImage(dev, t.image, nil)
	vk.DestroySampler(dev, t.sampler, nil)
}

func (d *Depth) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, d.view, nil)
	vk.DestroyImage(dev, d.image, nil)
	vk.FreeMemory(dev, d.mem, nil)
}

// func InitVulkan() {
	// TODO: decide what fctns below to source out - some of these are in as
	// as.NewPlatform(app) handles createInstance(), pickPhysicalDevice(), some of createSurface(), createLogicalDevice()

	// createInstance(); D
	// setupDebugMessenger(); D
	// createSurface(); D
	// pickPhysicalDevice(); D
	// createLogicalDevice(); D
	// createSwapChain(); D VulkanContextPrepare() and others?
	// createImageViews(); D
	// createRenderPass(); D
	// createDescriptorSetLayout(); D
	// createGraphicsPipeline(); D
	// createCommandPool(); D
	// createDepthResources(); D
	// createFramebuffers(); D
	// createTextureImage(); D
	// createTextureImageView(); D
	// createTextureSampler(); D
	// loadhm(); D In internal/heightmap
	// createVertexBuffer(); These are represented by the cube dims at the bottom
	// createIndexBuffer();  ^^^^^
	// createUniformBuffers(); D nextFrame()
	// createDescriptorPool(); D
	// createDescriptorSets(); D
	// createCommandBuffers(); drawBuildCommandBuffer?
	// createSyncObjects(); 

// }

func isError(ret vk.Result) bool {
	return ret != vk.Success
}

// Converts the given file to a byte array to be used
func fileToBytes(filename string) []byte {
	file, err := os.Open(filename)
	utils.OrPanic(err)
	defer file.Close()

	// Read the file into data
	fileInfo, err := file.Stat()
	utils.OrPanic(err)
	fileSize := fileInfo.Size()
	data := make([]byte, fileSize)
	_, err = file.Read(data)
	utils.OrPanic(err)

	return data
}

// TODO remove, these are structs for rendering the cube
type vkTexCubeUniform struct {
	mvp      lin.Mat4x4
	position [12 * 3][4]float32
	attr     [12 * 3][4]float32
}

const vkTexCubeUniformSize = int(unsafe.Sizeof(vkTexCubeUniform{}))

func (u *vkTexCubeUniform) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:vkTexCubeUniformSize]
}

var gVertexBufferData = []float32{
	-1.0, -1.0, -1.0, // -X side
	-1.0, -1.0, 1.0,
	-1.0, 1.0, 1.0,
	-1.0, 1.0, 1.0,
	-1.0, 1.0, -1.0,
	-1.0, -1.0, -1.0,

	-1.0, -1.0, -1.0, // -Z side
	1.0, 1.0, -1.0,
	1.0, -1.0, -1.0,
	-1.0, -1.0, -1.0,
	-1.0, 1.0, -1.0,
	1.0, 1.0, -1.0,

	-1.0, -1.0, -1.0, // -Y side
	1.0, -1.0, -1.0,
	1.0, -1.0, 1.0,
	-1.0, -1.0, -1.0,
	1.0, -1.0, 1.0,
	-1.0, -1.0, 1.0,

	-1.0, 1.0, -1.0, // +Y side
	-1.0, 1.0, 1.0,
	1.0, 1.0, 1.0,
	-1.0, 1.0, -1.0,
	1.0, 1.0, 1.0,
	1.0, 1.0, -1.0,

	1.0, 1.0, -1.0, // +X side
	1.0, 1.0, 1.0,
	1.0, -1.0, 1.0,
	1.0, -1.0, 1.0,
	1.0, -1.0, -1.0,
	1.0, 1.0, -1.0,

	-1.0, 1.0, 1.0, // +Z side
	-1.0, -1.0, 1.0,
	1.0, 1.0, 1.0,
	-1.0, -1.0, 1.0,
	1.0, -1.0, 1.0,
	1.0, 1.0, 1.0,
}

var gUVBufferData = []float32{
	0.0, 1.0, // -X side
	1.0, 1.0,
	1.0, 0.0,
	1.0, 0.0,
	0.0, 0.0,
	0.0, 1.0,

	1.0, 1.0, // -Z side
	0.0, 0.0,
	0.0, 1.0,
	1.0, 1.0,
	1.0, 0.0,
	0.0, 0.0,

	1.0, 0.0, // -Y side
	1.0, 1.0,
	0.0, 1.0,
	1.0, 0.0,
	0.0, 1.0,
	0.0, 0.0,

	1.0, 0.0, // +Y side
	0.0, 0.0,
	0.0, 1.0,
	1.0, 0.0,
	0.0, 1.0,
	1.0, 1.0,

	1.0, 0.0, // +X side
	0.0, 0.0,
	0.0, 1.0,
	0.0, 1.0,
	1.0, 1.0,
	1.0, 0.0,

	0.0, 0.0, // +Z side
	0.0, 1.0,
	1.0, 0.0,
	0.0, 1.0,
	1.0, 1.0,
	1.0, 0.0,
}

// TODO end of cube functions 
