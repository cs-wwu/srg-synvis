package rendering

import (
	"time"

	"github.com/cs-wwu/srg-synvis/internal/camera"
	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/streaming"
	"github.com/elokore/cimgui-go-vulkan/backend"
	glfwvulkanbackend "github.com/elokore/cimgui-go-vulkan/backend/glfwvulkan-backend"
	as "github.com/vulkan-go/asche"

	"github.com/go-gl/glfw/v3.4/glfw"

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

	// Airspaces
	AirspaceStreamer          *streaming.DegreeChunkStreamer[*Airspace]
	airspacePipeline          vk.Pipeline
	airspaceStencilPipeline   vk.Pipeline
	fullscreenOverlayPipeline vk.Pipeline
	CurrentAirspace           *Feature
	lastAirspaceCheck         time.Time

	// Roads
	roadPipeline vk.Pipeline

	// Obstructions
	obstructionPipeline vk.Pipeline

	// Skybox resources
	skyboxPipeline     vk.Pipeline
	skyboxVertexBuffer vk.Buffer
	skyboxVertexMemory vk.DeviceMemory
	skyboxIndexBuffer  vk.Buffer
	skyboxIndexMemory  vk.DeviceMemory
	skyboxIndexCount   uint32

	frameCount int

	ubo UniformBufferObject

	Cfg models.AppConfig

	Camera        *camera.Camera
	StreamManager *StreamManager
	WindowHandle  *glfw.Window
	CimguiBackend backend.Backend[glfwvulkanbackend.GLFWWindowFlags]

	RoadStreamer *streaming.DegreeChunkStreamer[*RoadSquare]

	ObstructionStreamer *streaming.DegreeChunkStreamer[*ObstructionSquare]

	Gps bool
	Hat bool

	statsTracker *StatsTracker
}

const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0
)

const (
	MAPS_PATH             = "assets/maps/"
	DEFAULT_WINDOW_WIDTH  = 800
	DEFAULT_WINDOW_HEIGHT = 480
)

var texEnabled = []string{
	"assets/textures/Default.png",
}

const (
	DepthFormat   = vk.FormatD24UnormS8Uint
	TextureFormat = vk.FormatR8g8b8a8Unorm
)

var airspaceColors = map[string][4]float32{
	"A": {0.8, 0.0, 0.0, 0.4},  // Red - highest priority controlled airspace
	"B": {0.0, 0.0, 0.5, 0.3},  // Dark blue - major airport terminal areas
	"C": {0.8, 0.0, 0.8, 0.3},  // Magenta - medium airport areas
	"D": {0.0, 0.0, 1.0, 0.25}, // Blue - smaller controlled airports
	"E": {0.5, 0.5, 0.5, 0.25}, // Gray - controlled airspace (transparent)
	"F": {0.0, 0.5, 0.0, 0.2},  // Green - special use (rare in US)
	"G": {0.3, 0.3, 0.3, 0.1},  // Light gray - uncontrolled (very transparent)
}
