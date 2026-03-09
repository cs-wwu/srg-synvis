package spatial

import (
	"math"

	"github.com/ungerik/go3d/float64/vec2"
)

// NewFrustumFromCamera creates a 2D frustum from camera parameters
func NewFrustumFromCamera(position vec2.T, direction float64, fov float64, near float64, far float64) Frustum {
	var frustum Frustum

	// Calculate half FOV angle
	halfFOV := fov * 0.5

	// Calculate direction vectors
	// Front direction
	frontX := math.Cos(direction)
	frontZ := math.Sin(direction)
	front := vec2.T{frontX, frontZ}

	// Left plane - normal points inward (toward right)
	leftAngle := direction - halfFOV
	leftDirX := math.Cos(leftAngle)
	leftDirZ := math.Sin(leftAngle)
	// Normal is perpendicular to left edge, pointing inward
	frustum.Planes[0].Normal = vec2.T{-leftDirZ, leftDirX}
	frustum.Planes[0].D = -(frustum.Planes[0].Normal[0]*position[0] + frustum.Planes[0].Normal[1]*position[1])

	// Right plane - normal points inward (toward left)
	rightAngle := direction + halfFOV
	rightDirX := math.Cos(rightAngle)
	rightDirZ := math.Sin(rightAngle)
	// Normal is perpendicular to right edge, pointing inward
	frustum.Planes[1].Normal = vec2.T{rightDirZ, -rightDirX}
	frustum.Planes[1].D = -(frustum.Planes[1].Normal[0]*position[0] + frustum.Planes[1].Normal[1]*position[1])

	// Near plane - normal points away from camera
	nearCenter := vec2.T{position[0] + front[0]*near, position[1] + front[1]*near}
	frustum.Planes[2].Normal = front
	frustum.Planes[2].D = -(frustum.Planes[2].Normal[0]*nearCenter[0] + frustum.Planes[2].Normal[1]*nearCenter[1])

	// Far plane - normal points toward camera
	farCenter := vec2.T{position[0] + front[0]*far, position[1] + front[1]*far}
	frustum.Planes[3].Normal = vec2.T{-front[0], -front[1]}
	frustum.Planes[3].D = -(frustum.Planes[3].Normal[0]*farCenter[0] + frustum.Planes[3].Normal[1]*farCenter[1])

	return frustum
}

// NewFrustumFromVectors creates a 2D frustum from camera position and front vector
func NewFrustumFromVectors(position vec2.T, front vec2.T, fov float64, near float64, far float64) Frustum {
	// Calculate direction angle from front vector
	direction := math.Atan2(front[1], front[0])
	return NewFrustumFromCamera(position, direction, fov, near, far)
}

// IsChunkInFrustum determines if a chunk is inside the 2D view frustum
func IsChunkInFrustum(chunk *ChunkInfo, frustum Frustum) bool {
	// Define the chunk's 2D AABB
	halfSize := 0.5 // Assuming normalized chunk size of 1.0

	// Calculate the extents of the chunk's 2D bounding box
	minX := float64(chunk.WorldPos[0]) - halfSize
	maxX := float64(chunk.WorldPos[0]) + halfSize
	minZ := float64(chunk.WorldPos[1]) - halfSize
	maxZ := float64(chunk.WorldPos[1]) + halfSize

	// Check against each frustum plane
	for i := range 4 {
		plane := &frustum.Planes[i]

		// Find the point furthest in the direction of the plane normal (positive vertex)
		var pX, pZ float64
		if plane.Normal[0] >= 0 {
			pX = maxX
		} else {
			pX = minX
		}
		if plane.Normal[1] >= 0 {
			pZ = maxZ
		} else {
			pZ = minZ
		}

		// If the positive vertex is outside, the whole box is outside
		// dot(normal, p) + d < 0 means outside
		if plane.Normal[0]*pX+plane.Normal[1]*pZ+plane.D < 0 {
			return false
		}
	}

	return true
}

// IsPointInFrustum tests if a 2D point is inside the frustum
func IsPointInFrustum(point vec2.T, frustum Frustum) bool {
	for i := range 4 {
		plane := &frustum.Planes[i]

		// If point is outside this plane, it's outside the frustum
		if plane.Normal[0]*point[0]+plane.Normal[1]*point[1]+plane.D < 0 {
			return false
		}
	}
	return true
}
