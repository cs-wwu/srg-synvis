// Package types provides universal types used across packages
package types

import (
	"unsafe"
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

// creating a byte slice array of size UniformBufferObject
const UBOSize = int(unsafe.Sizeof(UniformBufferObject{}))
func (u *UniformBufferObject) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:UBOSize]
}

// creating a byte slice array of size Vertex
const VertexSize = int(unsafe.Sizeof(Vertex{}))
func (u *Vertex) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:VertexSize]
}

const IndexSize = int(unsafe.Sizeof(int32(0)))
func (u *Index) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:IndexSize]
}

var GVertexBufferData = []Vertex{
    {ln.Vec3{-0.5, -0.5, 0.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{1.0, 0.0}},   // 0
    {ln.Vec3{0.5, -0.5, 0.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{0.0, 1.0}},    // 1
    {ln.Vec3{0.5, 0.5, 0.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{1.0, 0.0}},     // 2
    {ln.Vec3{-0.5, 0.5, 0.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{0.0, 1.0}},    // 3

    {ln.Vec3{-0.5, -0.5, -0.5}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{0.0, 0.0}},  // 4
    {ln.Vec3{0.5, -0.5, -0.5}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{1.0, 1.0}},   // 5
    {ln.Vec3{0.5, 0.5, -0.5}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{0.0, 0.0}},    // 6
    {ln.Vec3{-0.5, 0.5, -0.5}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec3{1.0, 1.0, 1.0}, ln.Vec2{1.0, 1.0}},   // 7
}

var GIndexBufferData = []uint32{
    0, 1, 2, 2, 3, 0,
    4, 7, 6, 6, 5, 4,
    5, 6, 2, 2, 1, 5,
    3, 2, 6, 6, 7, 3,   // top
    4, 0, 3, 3, 7, 4,
    0, 4, 5, 5, 1, 0,
}