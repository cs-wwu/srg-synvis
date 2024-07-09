// Package types provides universal types used across packages
package types

import (
	"github.com/go-gl/glfw/v3.3/glfw"

	as "github.com/vulkan-go/asche"
	ln "github.com/xlab/linmath"
)

type Vertex struct {
	Pos      ln.Vec3
	Color    ln.Vec3
	Normal   ln.Vec3
	TexCoord ln.Vec2
}

// will be expanded
type Application struct {
	as.BaseVulkanApp

	Window             *glfw.Window
	FramebufferResized bool

	Width  uint32
	Height uint32
}
