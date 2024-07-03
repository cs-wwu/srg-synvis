package utils

import (
	vk "github.com/vulkan-go/vulkan"
)

// Global util for panicing on different errors
func orPanic(err interface{}) {
	switch v := err.(type) {
	case error:
		if v != nil {
			panic(err)
		}
	case vk.Result:
		if err := vk.Error(v); err != nil {
			panic(err)
		}
	case bool:
		if !v {
			panic("condition failed: != true")
		}
	default:
		panic("unsupported type")
	}
}
