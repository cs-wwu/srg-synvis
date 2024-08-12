package renderer

import (
	"unsafe"

	lin "github.com/xlab/linmath"
)

type Vertex struct {
	Position lin.Vec3
	TexCoord lin.Vec3
	Normal   lin.Vec3
}

const VertexSize = int(unsafe.Sizeof(Vertex{}))

func (v *Vertex) Data() []byte {
	return (*[VertexSize]byte)(unsafe.Pointer(v))[:]
}

type Index uint32 // Be wary of integer overflow with large mesh

const IndexSize = int(unsafe.Sizeof(Index(0)))

func (u *Index) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:IndexSize]
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

type UIVertex struct {
	Position lin.Vec2
	TexCoord lin.Vec2
}

const UIVertexSize = int(unsafe.Sizeof(UIVertex{}))

func (v *UIVertex) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(v))[:UIVertexSize]
}
