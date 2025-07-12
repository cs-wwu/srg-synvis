package ui

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L../build -lImGuiUI -lstdc++

#include "ui_c_interface.h"
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"
)

type UIRenderer struct {
	cPtr *C.UIRenderer
}

func NewUIRenderer(vulkanInstance, vulkanDevice, vulkanPhysicalDevice unsafe.Pointer) *UIRenderer {
	cPtr := C.ui_create(vulkanInstance, vulkanDevice, vulkanPhysicalDevice)
	return &UIRenderer{cPtr: cPtr}
}

func (ui *UIRenderer) Destroy() {
	if ui.cPtr != nil {
		C.ui_destroy(ui.cPtr)
		ui.cPtr = nil
	}
}

func (ui *UIRenderer) InitForWindow(window unsafe.Pointer) bool {
	result := C.ui_init_for_window(ui.cPtr, window)
	return int(result) != 0
}

func (ui *UIRenderer) NewFrame() {
	C.ui_new_frame(ui.cPtr)
}

func (ui *UIRenderer) Render() {
	C.ui_render(ui.cPtr)
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
