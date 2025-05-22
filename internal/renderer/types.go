package renderer

import (
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
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
	NormalM    lin.Mat4x4 // for lighting
	LightDir   lin.Vec3   // Need to initialize this as
	_          float32    // padding to align to 16 bytes
	CameraPos  lin.Vec3   // camera position for terrain warnings
	_          float32    // padding to align to 16 bytes

	RenderDistance         float32 // Maximum render distanceu
	TerrainWarningsEnabled float32 // 1.0 = enabled, 0.0 = disabled
	Time                   float32
	_                      float32 // (Can remove in the future)
}

// creating a byte slice array of size UniformBufferObject
const UBOSize = int(unsafe.Sizeof(UniformBufferObject{}))

func (u *UniformBufferObject) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:UBOSize]
}

func (app *Application) Destroy() {}

type Texture struct {
	sampler vk.Sampler

	image       vk.Image
	imageLayout vk.ImageLayout

	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView

	texWidth  int32
	texHeight int32
}

func (t *Texture) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, t.view, nil)
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroyImage(dev, t.image, nil)
	vk.DestroySampler(dev, t.sampler, nil)
}

func (t *Texture) DestroyImage(dev vk.Device) {
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroyImage(dev, t.image, nil)
}

type Depth struct {
	format   vk.Format
	image    vk.Image
	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView
}

func (d *Depth) Destroy(dev vk.Device) {
	vk.DestroyImageView(dev, d.view, nil)
	vk.DestroyImage(dev, d.image, nil)
	vk.FreeMemory(dev, d.mem, nil)
}
