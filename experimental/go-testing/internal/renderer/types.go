package renderer

import (
	lin "github.com/xlab/linmath"
	"unsafe"
)

type Vertex struct {
	// possible error here: define number of vertices
	Pos      lin.Vec3
	TexCoord lin.Vec2
}

const VertexSize = int(unsafe.Sizeof(Vertex{}))

func (v *Vertex) Data() []byte {
	return (*[VertexSize]byte)(unsafe.Pointer(v))[:]
}

type Index int16

const IndexSize = int(unsafe.Sizeof(Index(0)))

func (u *Index) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:IndexSize]
}

type UBOp struct {
	MVP lin.Mat4x4
}

const UBOSizep = int(unsafe.Sizeof(UBOp{}))

func (u *UBOp) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:UBOSizep]
}

type UniformBufferObject struct {
	Model      lin.Mat4x4 // object to world space
	View       lin.Mat4x4 // world to camera space
	Projection lin.Mat4x4 // camera to clip and screen space

	NormalM  lin.Mat4x4 // for lighting
	LightDir lin.Vec3   // Need to initialize this as
}

// creating a byte slice array of size UniformBufferObject
const UBOSize = int(unsafe.Sizeof(UniformBufferObject{}))

func (u *UniformBufferObject) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:UBOSize]
}
