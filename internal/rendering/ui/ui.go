package ui

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L./build -lImGuiUI -lstdc++ -Wl,-rpath,'${SRCDIR}/build'

#include "./cpp_ui/ui_c_interface.h"
#include <stdlib.h>
#include <stdint.h>
*/
import "C"
import (
	"reflect"
	"unsafe"

	as "github.com/cs-wwu/srg-synvis/pkg/asche"
	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

type UIRenderer struct {
	cPtr *C.UIRenderer
}

func NewUIRenderer(vkInstance vk.Instance, vkDevice vk.Device, vkPhysicalDevice vk.PhysicalDevice, vkGraphicsQueue vk.Queue, vkPipelineCache vk.PipelineCache, graphicsQueueFamily uint32) *UIRenderer {
	cPtr := C.ui_create(
		unsafe.Pointer(vkInstance),
		unsafe.Pointer(vkDevice),
		unsafe.Pointer(vkPhysicalDevice),
		unsafe.Pointer(vkGraphicsQueue),
		unsafe.Pointer(vkPipelineCache),
		C.uint32_t(graphicsQueueFamily),
	)
	return &UIRenderer{cPtr: cPtr}
}

func (ui *UIRenderer) Destroy() {
	if ui.cPtr != nil {
		C.ui_destroy(ui.cPtr)
		ui.cPtr = nil
	}
}

func (ui *UIRenderer) CreateFrameBuffers(swapchainImageResources []*as.SwapchainImageResources, swapchainDimensions *as.SwapchainDimensions) bool {
	swapchainImageCount := len(swapchainImageResources)
	swapchainFormat := swapchainDimensions.Format
	width := swapchainDimensions.Width
	height := swapchainDimensions.Height

	// Extract image views from image resources
	imageViews := make([]unsafe.Pointer, swapchainImageCount)
	for i, res := range swapchainImageResources {
		imageViews[i] = unsafe.Pointer(res.View())
	}

	// Convert imageViews to be C compatible
	cImageViews := make([]*C.void, len(imageViews))
	for i, view := range imageViews {
		cImageViews[i] = (*C.void)(view)
	}

	imageViewsArray := (*unsafe.Pointer)(unsafe.Pointer(&cImageViews[0]))

	result := C.ui_create_frame_buffers(ui.cPtr, imageViewsArray, C.uint32_t(swapchainFormat), C.uint32_t(swapchainImageCount), C.int(width), C.int(height))
	return int(result) != 0
}

func (ui *UIRenderer) InitForWindow(window *glfw.Window) bool {
	v := reflect.ValueOf(window).Elem()
	dataField := v.FieldByName("data")
	cGlfwWindow := unsafe.Pointer(dataField.Pointer())

	result := C.ui_init_for_window(ui.cPtr, cGlfwWindow)
	return int(result) != 0
}

func (ui *UIRenderer) NewFrame() {
	C.ui_new_frame(ui.cPtr)
}

func (ui *UIRenderer) Render(vkCommandBuffer unsafe.Pointer, image_index int) {
	C.ui_render(ui.cPtr, vkCommandBuffer, C.int(image_index))
}
