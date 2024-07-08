package renderer

import (
	lin "github.com/xlab/linmath"
)

var (
	START_CAMERA_POS   = lin.Vec3{1740.0, 30.0, 957.0}
	START_CAMERA_UP    = lin.Vec3{0.0, 1.0, 0.0}
	START_CAMERA_FRONT = lin.Vec3{0.0, -1.0, 0.}
	CAMERA_SPEED       = float32(1.0)
)

type Camera struct {
	pos   *lin.Vec3 // (x, y, z)
	up    *lin.Vec3 // Front of the camera
	front *lin.Vec3 // Above the camera

	speed float32	// Camera speed
}

// NewCamera creates and initializes a new Camera
func NewCamera() *Camera {
	return &Camera{
		pos:   &START_CAMERA_POS,
		up:    &START_CAMERA_UP,
		front: &START_CAMERA_FRONT,
		speed: CAMERA_SPEED,
	}
}

// GetViewMatrix returns the camera's view matrix
func (c *Camera) GetViewMatrix() lin.Mat4x4 {
	result := lin.Mat4x4{}
	center := lin.Vec3{}
	center.Add(c.pos, c.front)
	result.LookAt(c.pos, &center, c.up)
	return result
}
