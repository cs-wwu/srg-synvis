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

type Camera struct {
	Pos   *ln.Vec3 // (x, y, z)
	Up    *ln.Vec3 // Front of the camera
	Front *ln.Vec3 // Above the camera

	Speed float32 // Camera speed
}

// GetViewMatrix returns the camera's view matrix
func (c *Camera) GetViewMatrix() ln.Mat4x4 {
	result := ln.Mat4x4{}
	center := ln.Vec3{}
	center.Add(c.Pos, c.Front)
	result.LookAt(c.Pos, &center, c.Up)
	return result
}
