package renderer

import (
	"fmt"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/go/internal/types"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// for window resizing
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

func InitVulkan() {
	// TODO: decide what fctns below to source out - some of these are in as
	// as.NewPlatform(app) handles createInstance(), pickPhysicalDevice(), some of createSurface(), createLogicalDevice()

	// createInstance();
	// setupDebugMessenger();
	// createSurface();
	// pickPhysicalDevice();
	// createLogicalDevice();
	// createSwapChain();
	// createImageViews();
	// createRenderPass();
	// createDescriptorSetLayout();
	// createGraphicsPipeline();
	// createCommandPool();
	// createDepthResources();
	// createFramebuffers();
	// createTextureImage();
	// createTextureImageView();
	// createTextureSampler();
	// loadhm();
	// createVertexBuffer();
	// createIndexBuffer();
	// createUniformBuffers();
	// createDescriptorPool();
	// createDescriptorSets();
	// createCommandBuffers();
	// createSyncObjects();

}
