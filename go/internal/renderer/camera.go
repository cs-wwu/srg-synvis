package renderer

import (
	ln "github.com/xlab/linmath"
)

var (
	START_CAMERA_POS   = ln.Vec3{1740.0, 30.0, 957.0}
	START_CAMERA_UP    = ln.Vec3{0.0, 1.0, 0.0}
	START_CAMERA_FRONT = ln.Vec3{0.0, -1.0, 0.}
	CAMERA_SPEED       = float32(1.0)
)

type Camera struct {
	Pos   *ln.Vec3 // (x, y, z)
	Up    *ln.Vec3 // Front of the camera
	Front *ln.Vec3 // Above the camera

	Speed float32 // Camera speed
}

// NewCamera creates and initializes a new Camera
func NewCamera() *Camera {
	return &Camera{
		Pos:   &START_CAMERA_POS,
		Up:    &START_CAMERA_UP,
		Front: &START_CAMERA_FRONT,
		Speed: CAMERA_SPEED,
	}
}

// GetViewMatrix returns the camera's view matrix
func (c *Camera) GetViewMatrix() ln.Mat4x4 {
	center := ln.Vec3{}
	center.Add(c.Pos, c.Front)

	result := ln.Mat4x4{}
	result.LookAt(c.Pos, &center, c.Up)
	return result
}
