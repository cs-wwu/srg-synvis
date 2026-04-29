package rendering

import (
	"unsafe"

	"github.com/ungerik/go3d/vec3"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

// NB: This gets cast to a byte slice, so it must have the exact same bytes
// that the shader expects.
type RoadVertex struct {
	Position vec3.T
	Color    vec3.T
}

type ObstructionVertex struct {
	Position vec3.T
	Color    vec3.T
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
	_pad            [2]float32
	OverlayColor    [4]float32 // Overlay color for being inside airspace
}

// creating a byte slice array of size UniformBufferObject
const UBOSize = int(unsafe.Sizeof(UniformBufferObject{}))

func (u *UniformBufferObject) Data() []byte {
	const m = 0x7fffffff
	return (*[m]byte)(unsafe.Pointer(u))[:UBOSize]
}

func (app *Application) Destroy() {}

type RoadSquare struct {
	vertices   *as.Buffer
	indices    *as.Buffer
	indexCount uint32
}

type ObstructionSquare struct {
	vertices   *as.Buffer
	indices    *as.Buffer
	indexCount uint32
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
