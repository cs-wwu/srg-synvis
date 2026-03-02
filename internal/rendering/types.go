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

type ObstructionVertex struct {
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

func (app *Application) Destroy() {
	dev := app.Context().Device()
	vk.DeviceWaitIdle(dev)

	// Pipelines
	vk.DestroyPipeline(dev, app.pipeline, nil)
	vk.DestroyPipeline(dev, app.airspacePipeline, nil)
	vk.DestroyPipeline(dev, app.airspaceStencilPipeline, nil)
	vk.DestroyPipeline(dev, app.obstructionPipeline, nil)
	vk.DestroyPipeline(dev, app.planePipeline, nil)
	vk.DestroyPipeline(dev, app.roadPipeline, nil)
	vk.DestroyPipeline(dev, app.skyboxPipeline, nil)

	// Set to null so asche doesnt mess with it
	app.pipeline = vk.NullPipeline
	app.airspacePipeline = vk.NullPipeline
	app.airspaceStencilPipeline = vk.NullPipeline
	app.obstructionPipeline = vk.NullPipeline
	app.planePipeline = vk.NullPipeline
	app.roadPipeline = vk.NullPipeline
	app.skyboxPipeline = vk.NullPipeline
	println("Cleanup pipelines")

	// Render pass and pipeline layout
	vk.DestroyPipelineLayout(dev, app.pipelineLayout, nil)
	vk.DestroyRenderPass(dev, app.renderPass, nil)
	vk.DestroyDescriptorSetLayout(dev, app.descLayout, nil)

	app.pipelineLayout = vk.NullPipelineLayout
	app.renderPass = vk.NullRenderPass
	app.descLayout = vk.NullDescriptorSetLayout
	println("Cleanup Layouts and Render pass")

	// Buffers
	vk.DestroyBuffer(dev, app.planeVertexBuffer, nil)
	vk.DestroyBuffer(dev, app.planeIndexBuffer, nil)
	vk.DestroyBuffer(dev, app.skyboxIndexBuffer, nil)
	vk.DestroyBuffer(dev, app.skyboxVertexBuffer, nil)

	app.planeVertexBuffer = vk.NullBuffer
	app.planeIndexBuffer = vk.NullBuffer
	app.skyboxIndexBuffer = vk.NullBuffer
	app.skyboxVertexBuffer = vk.NullBuffer
	println("Cleanup Buffers")

	// Memory
	vk.FreeMemory(dev, app.planeIndexMemory, nil)
	vk.FreeMemory(dev, app.planeVertexMemory, nil)
	vk.FreeMemory(dev, app.skyboxIndexMemory, nil)
	vk.FreeMemory(dev, app.skyboxVertexMemory, nil)

	app.planeIndexMemory = vk.NullDeviceMemory
	app.planeVertexMemory = vk.NullDeviceMemory
	app.skyboxIndexMemory = vk.NullDeviceMemory
	app.skyboxVertexMemory = vk.NullDeviceMemory

	println("Freed vulkan device memory")

	app.Context().Platform().Destroy()

}

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
