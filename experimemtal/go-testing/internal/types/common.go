// Package types provides universal types used across packages
package types

import (
	ln "github.com/xlab/linmath"
)

type Vertex struct {
	Pos      ln.Vec3
	Color    ln.Vec3
	Normal   ln.Vec3
	TexCoord ln.Vec2
}

type Index int32

type UniformBufferObject struct {
	Model*   ln.Mat4x4 // object to world space
	View*    ln.Mat4x4 // world to camera space
	Proj*    ln.Mat4x4 // camera to clip and screen space
	NormalM* ln.Mat4x4 // for lighting

	LightDir ln.Vec3 // Need to initialize this as
}
