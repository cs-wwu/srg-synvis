package renderer

import (
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"
)

type Plane struct {
	Normal   lin.Vec3
	Distance float32
}

type Frustum struct {
	Planes [6]Plane // Near, Far, Left, Right, Top, Bottom
}

type Chunk struct {
	Vertices    []Vertex
	Indices     []Index
	BoundingBox AABB

	VertexBuffer vk.Buffer
	IndexBuffer  vk.Buffer
}

// Axis-Aligned Bounding Box
type AABB struct {
	Min lin.Vec3
	Max lin.Vec3
}


