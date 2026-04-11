package chunk

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

type Index uint32

const IndexSize = int(unsafe.Sizeof(Index(0)))

func (v *Vertex) Data() []byte {
	return (*[VertexSize]byte)(unsafe.Pointer(v))[:]
}

func (u *Index) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:IndexSize]
}
