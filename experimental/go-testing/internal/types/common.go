// Package types provides universal types used across packages
package types

import (
	lin "github.com/xlab/linmath"
	"unsafe"
)

type Vertexp struct {
	Position [12 * 3]lin.Vec4
	// _ 		 [12 * 3]float32				// alignas(16)
	Attr [12 * 3][4]float32
}

type UBOp struct {
	MVP lin.Mat4x4
}

const UBOSizep = int(unsafe.Sizeof(UBOp{}))

func (u *UBOp) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:UBOSizep]
}

const VertexpSize = int(unsafe.Sizeof(Vertexp{}))

func (u *Vertexp) Data() []byte {
	return (*[VertexpSize]byte)(unsafe.Pointer(u))[:]
}

var GVertexData = []lin.Vec4{
	{-1.0, -1.0, -1.0, 1.0}, // -X side
	{-1.0, -1.0, 1.0, 1.0},
	{-1.0, 1.0, 1.0, 1.0},
	{-1.0, 1.0, 1.0, 1.0},
	{-1.0, 1.0, -1.0, 1.0},
	{-1.0, -1.0, -1.0, 1.0},

	{-1.0, -1.0, -1.0, 1.0}, // -Z side
	{1.0, 1.0, -1.0, 1.0},
	{1.0, -1.0, -1.0, 1.0},
	{-1.0, -1.0, -1.0, 1.0},
	{-1.0, 1.0, -1.0, 1.0},
	{1.0, 1.0, -1.0, 1.0},

	{-1.0, -1.0, -1.0, 1.0}, // -Y side
	{1.0, -1.0, -1.0, 1.0},
	{1.0, -1.0, 1.0, 1.0},
	{-1.0, -1.0, -1.0, 1.0},
	{1.0, -1.0, 1.0, 1.0},
	{-1.0, -1.0, 1.0, 1.0},

	{-1.0, 1.0, -1.0, 1.0}, // +Y side
	{-1.0, 1.0, 1.0, 1.0},
	{1.0, 1.0, 1.0, 1.0},
	{-1.0, 1.0, -1.0, 1.0},
	{1.0, 1.0, 1.0, 1.0},
	{1.0, 1.0, -1.0, 1.0},

	{1.0, 1.0, -1.0, 1.0}, // +X side
	{1.0, 1.0, 1.0, 1.0},
	{1.0, -1.0, 1.0, 1.0},
	{1.0, -1.0, 1.0, 1.0},
	{1.0, -1.0, -1.0, 1.0},
	{1.0, 1.0, -1.0, 1.0},

	{-1.0, 1.0, 1.0, 1.0}, // +Z side
	{-1.0, -1.0, 1.0, 1.0},
	{1.0, 1.0, 1.0, 1.0},
	{-1.0, -1.0, 1.0, 1.0},
	{1.0, -1.0, 1.0, 1.0},
	{1.0, 1.0, 1.0, 1.0},
}

var GUVBufferData = []float32{
	0.0, 1.0, // -X side
	1.0, 1.0,
	1.0, 0.0,
	1.0, 0.0,
	0.0, 0.0,
	0.0, 1.0,

	1.0, 1.0, // -Z side
	0.0, 0.0,
	0.0, 1.0,
	1.0, 1.0,
	1.0, 0.0,
	0.0, 0.0,

	1.0, 0.0, // -Y side
	1.0, 1.0,
	0.0, 1.0,
	1.0, 0.0,
	0.0, 1.0,
	0.0, 0.0,

	1.0, 0.0, // +Y side
	0.0, 0.0,
	0.0, 1.0,
	1.0, 0.0,
	0.0, 1.0,
	1.0, 1.0,

	1.0, 0.0, // +X side
	0.0, 0.0,
	0.0, 1.0,
	0.0, 1.0,
	1.0, 1.0,
	1.0, 0.0,

	0.0, 0.0, // +Z side
	0.0, 1.0,
	1.0, 0.0,
	0.0, 1.0,
	1.0, 1.0,
	1.0, 0.0,
}

type Vertex struct {
	// possible error here: define number of vertices
	Pos      lin.Vec3
	Color    lin.Vec3
	Normal   lin.Vec3
	TexCoord lin.Vec2
}

type Index int32

type UniformBufferObject struct {
	Model   *lin.Mat4x4 // object to world space
	View    *lin.Mat4x4 // world to camera space
	Proj    *lin.Mat4x4 // camera to clip and screen space
	NormalM *lin.Mat4x4 // for lighting

	LightDir lin.Vec3 // Need to initialize this as
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
	{lin.Vec3{-0.5, -0.5, 0.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{1.0, 0.0}}, // 0
	{lin.Vec3{0.5, -0.5, 0.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{0.0, 1.0}},  // 1
	{lin.Vec3{0.5, 0.5, 0.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{1.0, 0.0}},   // 2
	{lin.Vec3{-0.5, 0.5, 0.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{0.0, 1.0}},  // 3

	{lin.Vec3{-0.5, -0.5, -0.5}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{0.0, 0.0}}, // 4
	{lin.Vec3{0.5, -0.5, -0.5}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{1.0, 1.0}},  // 5
	{lin.Vec3{0.5, 0.5, -0.5}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{0.0, 0.0}},   // 6
	{lin.Vec3{-0.5, 0.5, -0.5}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec3{1.0, 1.0, 1.0}, lin.Vec2{1.0, 1.0}},  // 7
}

var GIndexBufferData = []Index{
	0, 1, 2, 2, 3, 0,
	4, 7, 6, 6, 5, 4,
	5, 6, 2, 2, 1, 5,
	3, 2, 6, 6, 7, 3, // top
	4, 0, 3, 3, 7, 4,
	0, 4, 5, 5, 1, 0,
}
