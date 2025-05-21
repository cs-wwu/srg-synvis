package spatial

import (
	"math"

	lin "github.com/xlab/linmath"
)

// NewFrustumFromMatrices creates a frustum from view and projection matrices
func NewFrustumFromMatrices(view, proj *lin.Mat4x4) Frustum {
	var frustum Frustum

	// Combine view and projection
	var clip lin.Mat4x4
	clip.Mult(proj, view)

	// Extract frustum planes

	// Left plane
	frustum.Planes[0].Normal[0] = clip[0][3] + clip[0][0]
	frustum.Planes[0].Normal[1] = clip[1][3] + clip[1][0]
	frustum.Planes[0].Normal[2] = clip[2][3] + clip[2][0]
	frustum.Planes[0].D = clip[3][3] + clip[3][0]

	// Right plane
	frustum.Planes[1].Normal[0] = clip[0][3] - clip[0][0]
	frustum.Planes[1].Normal[1] = clip[1][3] - clip[1][0]
	frustum.Planes[1].Normal[2] = clip[2][3] - clip[2][0]
	frustum.Planes[1].D = clip[3][3] - clip[3][0]

	// Bottom plane
	frustum.Planes[2].Normal[0] = clip[0][3] + clip[0][1]
	frustum.Planes[2].Normal[1] = clip[1][3] + clip[1][1]
	frustum.Planes[2].Normal[2] = clip[2][3] + clip[2][1]
	frustum.Planes[2].D = clip[3][3] + clip[3][1]

	// Top plane
	frustum.Planes[3].Normal[0] = clip[0][3] - clip[0][1]
	frustum.Planes[3].Normal[1] = clip[1][3] - clip[1][1]
	frustum.Planes[3].Normal[2] = clip[2][3] - clip[2][1]
	frustum.Planes[3].D = clip[3][3] - clip[3][1]

	// Near plane
	frustum.Planes[4].Normal[0] = clip[0][3] + clip[0][2]
	frustum.Planes[4].Normal[1] = clip[1][3] + clip[1][2]
	frustum.Planes[4].Normal[2] = clip[2][3] + clip[2][2]
	frustum.Planes[4].D = clip[3][3] + clip[3][2]

	// Far plane
	frustum.Planes[5].Normal[0] = clip[0][3] - clip[0][2]
	frustum.Planes[5].Normal[1] = clip[1][3] - clip[1][2]
	frustum.Planes[5].Normal[2] = clip[2][3] - clip[2][2]
	frustum.Planes[5].D = clip[3][3] - clip[3][2]

	// Normalize all planes
	for i := range frustum.Planes {
		length := float32(math.Sqrt(float64(
			frustum.Planes[i].Normal[0]*frustum.Planes[i].Normal[0] +
				frustum.Planes[i].Normal[1]*frustum.Planes[i].Normal[1] +
				frustum.Planes[i].Normal[2]*frustum.Planes[i].Normal[2])))

		frustum.Planes[i].Normal[0] /= length
		frustum.Planes[i].Normal[1] /= length
		frustum.Planes[i].Normal[2] /= length
		frustum.Planes[i].D /= length
	}

	return frustum
}

// isChunkInFrustum determines if a chunk is inside the view frustum
func isChunkInFrustum(chunk *ChunkInfo, frustum Frustum) bool {
	// Define the chunk's AABB
	halfSize := float32(0.5) // Assuming normalized chunk size of 1.0

	// Calculate the extents of the chunk's bounding box
	min := lin.Vec3{
		chunk.WorldPos[0] - halfSize,
		chunk.WorldPos[1] - halfSize,
		chunk.WorldPos[2] - halfSize,
	}

	max := lin.Vec3{
		chunk.WorldPos[0] + halfSize,
		chunk.WorldPos[1] + halfSize,
		chunk.WorldPos[2] + halfSize,
	}

	// Check against each frustum plane
	for i := range 6 {
		plane := &frustum.Planes[i]

		// Find the point furthest in the direction of the plane normal (positive vertex)
		p := lin.Vec3{min[0], min[1], min[2]}
		if plane.Normal[0] >= 0 {
			p[0] = max[0]
		}
		if plane.Normal[1] >= 0 {
			p[1] = max[1]
		}
		if plane.Normal[2] >= 0 {
			p[2] = max[2]
		}

		// If the positive vertex is outside, the whole box is outside
		// dot(normal, p) + d < 0
		if plane.Normal[0]*p[0]+plane.Normal[1]*p[1]+plane.Normal[2]*p[2]+plane.D < 0 {
			return false
		}
	}

	return true
}
