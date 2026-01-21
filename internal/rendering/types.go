package rendering

import (
	"unsafe"

	"github.com/ungerik/go3d/vec3"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

type Vertex struct {
	Position lin.Vec3
	TexCoord lin.Vec3
	Normal   lin.Vec3
}

// NB: This gets cast to a byte slice, so it must have the exact same bytes
// that the shader expects.
type RoadVertex struct {
	Position vec3.T
	Color    vec3.T
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
	Model           lin.Mat4x4 // object to world space
	View            lin.Mat4x4 // world to camera space
	Projection      lin.Mat4x4 // camera to clip and screen space
	NormalM         lin.Mat4x4 // for lighting
	PlaneDirection  lin.Mat4x4
	LightDir        lin.Vec3 // Light direction
	RenderDistance  float32  // Maximum render distance
	CameraPos       lin.Vec3 // camera position for terrain warnings
	Time            float32  // Animation time
	DisableWarnings float32  // 1.0 = disabled, 0.0 = enabled (match shader)
	DisableFog      float32
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

type RoadSquare struct {
	vertices   *as.Buffer
	indices    *as.Buffer
	indexCount uint32
}

func (t *Texture) Destroy(dev vk.Device) {
	if t.view != vk.NullImageView {
		vk.DestroyImageView(dev, t.view, nil)
		t.view = vk.NullImageView
	}
	if t.mem != vk.NullDeviceMemory {
		vk.FreeMemory(dev, t.mem, nil)
		t.mem = vk.NullDeviceMemory
	}
	if t.image != vk.NullImage {
		vk.DestroyImage(dev, t.image, nil)
		t.image = vk.NullImage
	}
	if t.sampler != vk.NullSampler {
		vk.DestroySampler(dev, t.sampler, nil)
		t.sampler = vk.NullSampler
	}
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
