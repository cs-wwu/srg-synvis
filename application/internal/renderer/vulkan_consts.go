package renderer

import (
	as "github.com/cs-wwu/srg-synvis/asche"

	"github.com/go-gl/glfw/v3.3/glfw"
	// as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
)

// This file is for storing the constants associated with vulkan's rendering

// The Main Application "superstruct" class
type Application struct {
	as.BaseVulkanApp

	width  uint32
	height uint32

	textures          []*Texture
	depth             *Depth
	useStagingBuffers bool

	descPool vk.DescriptorPool

	pipelineLayout vk.PipelineLayout
	descLayout     vk.DescriptorSetLayout
	pipelineCache  vk.PipelineCache
	renderPass     vk.RenderPass
	pipeline       vk.Pipeline

	frameCount int

	ubo UniformBufferObject

	camera       *Camera
	ChunkManager *ChunkManager
	frustum      *Frustum
	WindowHandle *glfw.Window
}

const (
	HGT_PATH         = "assets/maps/n48w123.hgt.png" // TODO hardcoded
	FOV              = 60.0                          // Field Of View in Degrees
	CULLING_INTERVAL = 1                             // Perform culling every x frames
	MAPS_PATH        = "assets/maps/"

	WINDOW_HEIGHT = 480
	WINDOW_WIDTH  = 800

	VIEW_DISTANCE = 6000
)

var texEnabled = []string{
	"assets/textures/checkerboard.png",
}

// Depth Constants
const (
	DepthImageType         = vk.ImageType2d
	DepthFormat            = vk.FormatD16Unorm
	DepthImageDepth        = 1
	DepthMipLevels         = 1
	DepthArrayLayers       = 1
	DepthSamples           = vk.SampleCount1Bit
	DepthImageTiling       = vk.ImageTilingOptimal
	DepthImageUsage        = vk.ImageUsageDepthStencilAttachmentBit
	DepthMemoryProperty    = vk.MemoryPropertyDeviceLocalBit
	DepthViewType          = vk.ImageViewType2d
	DepthAspectMask        = vk.ImageAspectDepthBit
	DepthSubresourceLevels = 1
	DepthSubresourceLayers = 1
)

// Texture constants
const (
	TextureFormat             = vk.FormatR8g8b8a8Unorm
	TextureImageType          = vk.ImageType2d
	TextureImageDepth         = 1
	TextureMipLevels          = 1
	TextureArrayLayers        = 1
	TextureSamples            = vk.SampleCount1Bit
	TextureInitialLayout      = vk.ImageLayoutPreinitialized
	TextureDefaultImageLayout = vk.ImageLayoutGeneral
	TextureAspectMask         = vk.ImageAspectColorBit
	TextureMemoryPropertyHost = vk.MemoryPropertyHostVisibleBit
)

// Image layout constants
const (
	ImageMemoryBarrierSType              = vk.StructureTypeImageMemoryBarrier
	ImageSubresourceLayerCount           = 1
	ImageSubresourceLevelCount           = 1
	ImageBarrierDependencyFlags          = 0
	ImageBarrierMemoryBarrierCount       = 0
	ImageBarrierBufferMemoryBarrierCount = 0
	ImageBarrierImageMemoryBarrierCount  = 1
)

// Texture constants
const (
	// TextureFormat                        = vk.FormatR8g8b8a8Unorm
	TextureImageAspect                    = vk.ImageAspectColorBit
	TextureImageLayoutPreinitialized      = vk.ImageLayoutPreinitialized
	TextureImageLayoutTransferSrcOptimal  = vk.ImageLayoutTransferSrcOptimal
	TextureImageLayoutTransferDstOptimal  = vk.ImageLayoutTransferDstOptimal
	TextureImageTilingLinear              = vk.ImageTilingLinear
	TextureImageTilingOptimal             = vk.ImageTilingOptimal
	TextureImageUsageSampledBit           = vk.ImageUsageSampledBit
	TextureImageUsageTransferSrcBit       = vk.ImageUsageTransferSrcBit
	TextureImageUsageTransferDstBit       = vk.ImageUsageTransferDstBit
	TextureMemoryPropertyHostVisibleBit   = vk.MemoryPropertyHostVisibleBit
	TextureMemoryPropertyHostCoherentBit  = vk.MemoryPropertyHostCoherentBit
	TextureMemoryPropertyDeviceLocalBit   = vk.MemoryPropertyDeviceLocalBit
	TextureAccessHostWriteBit             = vk.AccessHostWriteBit
	TexturePipelineStageHostBit           = vk.PipelineStageHostBit
	TextureAccessTransferWriteBit         = vk.AccessTransferWriteBit
	TexturePipelineStageTopOfPipeBit      = vk.PipelineStageTopOfPipeBit
	TexturePipelineStageFragmentShaderBit = vk.PipelineStageFragmentShaderBit
	TexturePipelineStageTransferBit       = vk.PipelineStageTransferBit
)

// Sampler constants
const (
	SamplerCreateInfoSType             = vk.StructureTypeSamplerCreateInfo
	SamplerFilterNearest               = vk.FilterNearest
	SamplerMipmapModeNearest           = vk.SamplerMipmapModeNearest
	SamplerAddressModeRepeat           = vk.SamplerAddressModeRepeat
	SamplerAnisotropyEnable            = vk.False
	SamplerMaxAnisotropy               = 1
	SamplerCompareOpNever              = vk.CompareOpNever
	SamplerBorderColorFloatOpaqueWhite = vk.BorderColorFloatOpaqueWhite
	SamplerUnnormalizedCoordinates     = vk.False
)

// Image view constants
const (
	ImageViewCreateInfoSType = vk.StructureTypeImageViewCreateInfo
	ImageViewType2D          = vk.ImageViewType2d
	ComponentSwizzleR        = vk.ComponentSwizzleR
	ComponentSwizzleG        = vk.ComponentSwizzleG
	ComponentSwizzleB        = vk.ComponentSwizzleB
	ComponentSwizzleA        = vk.ComponentSwizzleA
)

// Uniform Buffer constants
const (
	UniformBufferUsage = vk.BufferUsageUniformBufferBit
)

// Descriptor Layout constants
const (
	DescriptorSetLayoutCreateInfoSType = vk.StructureTypeDescriptorSetLayoutCreateInfo
	DescriptorSetLayoutBindingCount    = 2
	DescriptorSetLayoutBinding0        = 0
	DescriptorSetLayoutBinding1        = 1
	DescriptorTypeUniformBuffer        = vk.DescriptorTypeUniformBuffer
	DescriptorTypeCombinedImageSampler = vk.DescriptorTypeCombinedImageSampler
	ShaderStageVertexBit               = vk.ShaderStageVertexBit
	ShaderStageFragmentBit             = vk.ShaderStageFragmentBit
)

// Pipeline Layout constants
const (
	PipelineLayoutCreateInfoSType = vk.StructureTypePipelineLayoutCreateInfo
	PipelineLayoutSetCount        = 1
)

// Render Pass constants
const (
	RenderPassCreateInfoSType                = vk.StructureTypeRenderPassCreateInfo
	RenderPassAttachmentCount                = 2
	RenderPassSubpassCount                   = 1
	SampleCount1Bit                          = vk.SampleCount1Bit
	AttachmentLoadOpClear                    = vk.AttachmentLoadOpClear
	AttachmentStoreOpStore                   = vk.AttachmentStoreOpStore
	AttachmentStoreOpDontCare                = vk.AttachmentStoreOpDontCare
	AttachmentLoadOpDontCare                 = vk.AttachmentLoadOpDontCare
	ImageLayoutUndefined                     = vk.ImageLayoutUndefined
	ImageLayoutPresentSrc                    = vk.ImageLayoutPresentSrc
	ImageLayoutDepthStencilAttachmentOptimal = vk.ImageLayoutDepthStencilAttachmentOptimal
	ImageLayoutColorAttachmentOptimal        = vk.ImageLayoutColorAttachmentOptimal
	PipelineBindPointGraphics                = vk.PipelineBindPointGraphics
)

// Pipeline constants
const (
	PipelineCacheCreateInfoSType              = vk.StructureTypePipelineCacheCreateInfo
	VertexInputRateVertex                     = vk.VertexInputRateVertex
	FormatR32g32b32Sfloat                     = vk.FormatR32g32b32Sfloat
	FormatR32g32Sfloat                        = vk.FormatR32g32Sfloat
	GraphicsPipelineCreateInfoSType           = vk.StructureTypeGraphicsPipelineCreateInfo
	PipelineDynamicStateCreateInfoSType       = vk.StructureTypePipelineDynamicStateCreateInfo
	DynamicStateScissor                       = vk.DynamicStateScissor
	DynamicStateViewport                      = vk.DynamicStateViewport
	PipelineVertexInputStateCreateInfoSType   = vk.StructureTypePipelineVertexInputStateCreateInfo
	PipelineInputAssemblyStateCreateInfoSType = vk.StructureTypePipelineInputAssemblyStateCreateInfo
	PrimitiveTopologyTriangleStrip            = vk.PrimitiveTopologyTriangleStrip
	PipelineRasterizationStateCreateInfoSType = vk.StructureTypePipelineRasterizationStateCreateInfo
	PolygonModeFill                           = vk.PolygonModeFill
	CullModeBackBit                           = vk.CullModeBackBit
	FrontFaceCounterClockwise                 = vk.FrontFaceCounterClockwise
	PipelineColorBlendStateCreateInfoSType    = vk.StructureTypePipelineColorBlendStateCreateInfo
	PipelineMultisampleStateCreateInfoSType   = vk.StructureTypePipelineMultisampleStateCreateInfo
	// SampleCount1Bit                           = vk.SampleCount1Bit
	PipelineViewportStateCreateInfoSType     = vk.StructureTypePipelineViewportStateCreateInfo
	PipelineDepthStencilStateCreateInfoSType = vk.StructureTypePipelineDepthStencilStateCreateInfo
	CompareOpLessOrEqual                     = vk.CompareOpLessOrEqual
	StencilOpKeep                            = vk.StencilOpKeep
	CompareOpAlways                          = vk.CompareOpAlways
	PipelineShaderStageCreateInfoSType       = vk.StructureTypePipelineShaderStageCreateInfo
	// ShaderStageVertexBit                      = vk.ShaderStageVertexBit
	// ShaderStageFragmentBit                    = vk.ShaderStageFragmentBit
)

// Descriptor Pool constants
const (
	DescriptorPoolCreateInfoSType = vk.StructureTypeDescriptorPoolCreateInfo
	DescriptorPoolSizeCount       = 2
	// DescriptorTypeUniformBuffer        = vk.DescriptorTypeUniformBuffer
	// DescriptorTypeCombinedImageSampler = vk.DescriptorTypeCombinedImageSampler
)

// Descriptor Set constants
const (
	DescriptorSetAllocateInfoSType = vk.StructureTypeDescriptorSetAllocateInfo
	DescriptorSetCount             = 1
	WriteDescriptorSetSType        = vk.StructureTypeWriteDescriptorSet
	ImageLayoutGeneral             = vk.ImageLayoutGeneral
)

// Framebuffer constants
const (
	FramebufferCreateInfoSType = vk.StructureTypeFramebufferCreateInfo
	FramebufferAttachmentCount = 2
	FramebufferLayers          = 1
)

// Chunkmanager constnts
const (
	BufferUsageVertexBufferBit   = vk.BufferUsageVertexBufferBit
	BufferUsageIndexBufferBit    = vk.BufferUsageIndexBufferBit
	BufferUsageIndirectBufferBit = vk.BufferUsageIndirectBufferBit
)
