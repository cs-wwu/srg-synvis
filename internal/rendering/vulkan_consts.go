package rendering

import (
	"github.com/cs-wwu/srg-synvis/internal/camera"
	"github.com/cs-wwu/srg-synvis/internal/models"
	ui "github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/cs-wwu/srg-synvis/internal/streaming"
	as "github.com/cs-wwu/srg-synvis/pkg/asche"
	"github.com/elokore/cimgui-go-vulkan/backend"
	glfwvulkanbackend "github.com/elokore/cimgui-go-vulkan/backend/glfwvulkan-backend"

	"github.com/elokore/glfw/v3.4/glfw"

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

	// Plane model resources
	planePipeline     vk.Pipeline
	planeVertexBuffer vk.Buffer
	planeVertexMemory vk.DeviceMemory
	planeIndexBuffer  vk.Buffer
	planeIndexMemory  vk.DeviceMemory
	planeIndexCount   uint32

	// Roads
	roadPipeline vk.Pipeline

	// Skybox resources
	skyboxPipeline     vk.Pipeline
	skyboxVertexBuffer vk.Buffer
	skyboxVertexMemory vk.DeviceMemory
	skyboxIndexBuffer  vk.Buffer
	skyboxIndexMemory  vk.DeviceMemory
	skyboxIndexCount   uint32

	frameCount int

	ubo UniformBufferObject

	cfg models.AppConfig

	Camera        camera.Camera
	StreamManager *StreamManager
	WindowHandle  *glfw.Window
	UIRenderer    *ui.UIRenderer
	CimguiBackend backend.Backend[glfwvulkanbackend.GLFWWindowFlags]

	RoadStreamer *streaming.DegreeChunkStreamer[*RoadSquare]

	Gps bool
	Hat bool

	statsTracker *StatsTracker
}

const (
	MAPS_PATH     = "assets/maps/"
	WINDOW_HEIGHT = 480
	WINDOW_WIDTH  = 800
)

var texEnabled = []string{
	"assets/textures/Default.png",
}

const (
	DepthFormat   = vk.FormatD16Unorm
	TextureFormat = vk.FormatR8g8b8a8Unorm
)
