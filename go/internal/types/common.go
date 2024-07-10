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