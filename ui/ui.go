package ui

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L./build -lImGuiUI -lstdc++ -Wl,-rpath,'${SRCDIR}/build'

#include "ui_c_interface.h"
#include <stdlib.h>
#include <stdint.h>
*/
import "C"
import (
	"unsafe"

	as "github.com/cs-wwu/srg-synvis/pkg/asche"
)

type UIRenderer struct {
	cPtr *C.UIRenderer
}

func NewUIRenderer(vkInstance, vkDevice, vkPhysicalDevice, vkGraphicsQueue unsafe.Pointer, graphicsQueueFamily uint32, vkPipelineCache unsafe.Pointer) *UIRenderer {
	cPtr := C.ui_create(
		vkInstance,
		vkDevice,
		vkPhysicalDevice,
		vkGraphicsQueue,
		C.uint32_t(graphicsQueueFamily),
		vkPipelineCache)
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

func (ui *UIRenderer) InitForWindow(window unsafe.Pointer) bool {
	result := C.ui_init_for_window(ui.cPtr, window)
	return int(result) != 0
}

func (ui *UIRenderer) NewFrame() {
	C.ui_new_frame(ui.cPtr)
}

func (ui *UIRenderer) Render(vkCommandBuffer unsafe.Pointer, image_index int) {
	C.ui_render(ui.cPtr, vkCommandBuffer, C.int(image_index))
}

func (ui *UIRenderer) SetWindowTitle(title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.ui_set_window_title(ui.cPtr, cTitle)
}

func (ui *UIRenderer) AddText(text string) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	C.ui_add_text(ui.cPtr, cText)
}
