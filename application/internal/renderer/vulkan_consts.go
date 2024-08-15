package renderer

import (
	"image"

	"github.com/go-gl/glfw/v3.3/glfw"
	as "github.com/vulkan-go/asche"
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

	// uniformBuffersMapped []unsafe.Pointer
	chunks       map[string]*Chunk
	loadedChunks []*Chunk
	heightmap    *image.Gray16
	ubo          UniformBufferObject

	camera        *Camera
	frustum       *Frustum
	visibleChunks []*Chunk
	WindowHandle  *glfw.Window
}

const (
	HGT_PATH         = "assets/heightmaps/n48w123.png" // TODO hardcoded
	FOV              = 60.0                            // Field Of View in Degrees
	CULLING_INTERVAL = 2                               // Perform culling every x frames

	WINDOW_HEIGHT = 480
	WINDOW_WIDTH  = 800
)

var texEnabled = []string{
	"assets/textures/n48w123_google.png",
}

// Depth
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
