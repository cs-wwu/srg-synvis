package ui

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L./build -lImGuiUI -lstdc++ -Wl,-rpath,'${SRCDIR}/build'

#include "./cpp_ui/ui_c_interface.h"
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>
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

/*
Call BEFORE InitForWindow and then again whenever the swapchain format changes
or the resolution of the window changes
*/
func (ui *UIRenderer) InitRenderPass(swapchainImageResources []*as.SwapchainImageResources, swapchainDimensions *as.SwapchainDimensions) bool {
	swapchainImageCount := len(swapchainImageResources)
	swapchainFormat := swapchainDimensions.Format
	width := swapchainDimensions.Width
	height := swapchainDimensions.Height

	// Extract image views from image resources
	imageViews := make([]unsafe.Pointer, swapchainImageCount)
	swapchainImages := make([]unsafe.Pointer, swapchainImageCount)

	for i, res := range swapchainImageResources {
		imageViews[i] = unsafe.Pointer(res.View())
		swapchainImages[i] = unsafe.Pointer(res.Image())
	}

	// Convert image views to be C compatible
	cImageViews := make([]*C.void, swapchainImageCount)
	cSwapchainImages := make([]*C.void, swapchainImageCount)

	for i, view := range imageViews {
		cImageViews[i] = (*C.void)(view)
		cSwapchainImages[i] = (*C.void)(swapchainImages[i])
	}

	// C array of image views
	imageViewsArray := (*unsafe.Pointer)(unsafe.Pointer(&cImageViews[0]))
	swapchainImagesArray := (*unsafe.Pointer)(unsafe.Pointer(&cSwapchainImages[0]))

	result := C.ui_init_render_pass(ui.cPtr, imageViewsArray, swapchainImagesArray, C.uint32_t(swapchainFormat), C.uint32_t(swapchainImageCount), C.int(width), C.int(height))
	return int(result) != 0
}

/*
Call AFTER InitRenderPass and after the glfw window has been initialized, but before
the main program loop starts
*/
func (ui *UIRenderer) AttachToWindow(window *glfw.Window) bool {
	// The actual C glfw window handle is hidden as a private member of glfw.Window, use reflection to retrieve it
	v := reflect.ValueOf(window).Elem()
	dataField := v.FieldByName("data")
	cGlfwWindow := unsafe.Pointer(dataField.Pointer())

	result := C.ui_attach_to_window(ui.cPtr, cGlfwWindow)
	return int(result) != 0
}

/*
Renders all the UI, call every frame after rendering the main program but before presenting the new image
*/
func (ui *UIRenderer) Render(image_index int) {
	C.ui_render(ui.cPtr, C.int(image_index))
}

func (ui *UIRenderer) DebugFloatSlider(title string, sliderValue *float32, minValue float32, maxValue float32, formatStr string) int {
	cTitle := C.CString(title)
	cFormatStr := C.CString(formatStr)

	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cFormatStr))

	valuePtr := (*C.float)(unsafe.Pointer(sliderValue))

	id := C.ui_debug_float_slider(
		ui.cPtr,
		cTitle,
		valuePtr,
		C.float(minValue),
		C.float(maxValue),
		cFormatStr,
	)

	return int(id)
}

func (ui *UIRenderer) DebugCheckbox(label string, toggled *bool) int {
	cLabel := C.CString(label)
	toggledPtr := (*C.bool)(unsafe.Pointer(toggled))
	defer C.free(unsafe.Pointer(cLabel))

	id := C.ui_debug_checkbox(ui.cPtr, cLabel, toggledPtr)
	return int(id)
}

func (ui *UIRenderer) DebugTextLabel(label string) int {
	cLabel := C.CString(label)
	defer C.free(unsafe.Pointer(cLabel))

	id := C.ui_debug_text_label(ui.cPtr, cLabel)
	return int(id)
}

func (ui *UIRenderer) DebugUpdateLabel(id int, newLabel string) {
	cLabel := C.CString(newLabel)
	defer C.free(unsafe.Pointer(cLabel))

	C.ui_debug_update_label(ui.cPtr, C.int(id), cLabel)
}
