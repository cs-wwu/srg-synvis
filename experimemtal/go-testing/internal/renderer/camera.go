package renderer

import (
	lin "github.com/xlab/linmath"
)

var (
	// For Heightmap
	// START_CAMERA_POS   = ln.Vec3{1740.0, 30.0, 957.0}
	// START_CAMERA_UP    = ln.Vec3{0.0, 1.0, 0.0}
	// START_CAMERA_FRONT = ln.Vec3{0.0, -1.0, 0.}
	START_CAMERA_POS   = lin.Vec3{0.0, 2.5, 5.0}
	START_CAMERA_UP    = lin.Vec3{0.0, 1.0, 0.0}
	START_CAMERA_FRONT = lin.Vec3{0.0, -0.5, -0.86}
	CAMERA_SPEED       = float32(1.0)
)

type Camera struct {
	Pos   *lin.Vec3 // (x, y, z)
	Up    *lin.Vec3 // Front of the camera
	Front *lin.Vec3 // Above the camera

	Speed float32 // Camera speed
}

// NewCamera creates and initializes a new Camera
func NewCamera() *Camera {
	pos := START_CAMERA_POS
	up := START_CAMERA_UP
	front := START_CAMERA_FRONT

	return &Camera{
		Pos:   &pos,
		Up:    &up,
		Front: &front,
		Speed: CAMERA_SPEED,
	}
}

// GetViewMatrix returns the camera's view matrix
func (c *Camera) GetViewMatrix() lin.Mat4x4 {
	center := lin.Vec3{}
	center.Add(c.Pos, c.Front)

	result := lin.Mat4x4{}
	result.LookAt(c.Pos, &center, c.Up)
	return result
}
