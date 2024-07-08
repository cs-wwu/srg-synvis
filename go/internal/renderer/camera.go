package renderer

import (
	"github.com/cs-wwu/srg-synvis/go/internal/types"

	ln "github.com/xlab/linmath"
)

var (
	START_CAMERA_POS   = ln.Vec3{1740.0, 30.0, 957.0}
	START_CAMERA_UP    = ln.Vec3{0.0, 1.0, 0.0}
	START_CAMERA_FRONT = ln.Vec3{0.0, -1.0, 0.}
	CAMERA_SPEED       = float32(1.0)
)

// NewCamera creates and initializes a new Camera
func NewCamera() *types.Camera {
	return &types.Camera{
		Pos:   &START_CAMERA_POS,
		Up:    &START_CAMERA_UP,
		Front: &START_CAMERA_FRONT,
		Speed: CAMERA_SPEED,
	}
}

