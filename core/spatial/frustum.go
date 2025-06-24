package spatial

import (
	"math"

	lin "github.com/xlab/linmath"
)

// NewFrustumFromCamera creates a 2D frustum from camera parameters
func NewFrustumFromCamera(position lin.Vec2, direction float32, fov float32, near float32, far float32) Frustum {
	var frustum Frustum

	// Calculate half FOV angle
	halfFOV := fov * 0.5

	// Calculate direction vectors
	// Front direction
	frontX := float32(math.Cos(float64(direction)))
	frontZ := float32(math.Sin(float64(direction)))
	front := lin.Vec2{frontX, frontZ}

	// Left plane - normal points inward (toward right)
	leftAngle := direction - halfFOV
	leftDirX := float32(math.Cos(float64(leftAngle)))
	leftDirZ := float32(math.Sin(float64(leftAngle)))
	// Normal is perpendicular to left edge, pointing inward
	frustum.Planes[0].Normal = lin.Vec2{-leftDirZ, leftDirX}
	frustum.Planes[0].D = -(frustum.Planes[0].Normal[0]*position[0] + frustum.Planes[0].Normal[1]*position[1])

	// Right plane - normal points inward (toward left)
	rightAngle := direction + halfFOV
	rightDirX := float32(math.Cos(float64(rightAngle)))
	rightDirZ := float32(math.Sin(float64(rightAngle)))
	// Normal is perpendicular to right edge, pointing inward
	frustum.Planes[1].Normal = lin.Vec2{rightDirZ, -rightDirX}
	frustum.Planes[1].D = -(frustum.Planes[1].Normal[0]*position[0] + frustum.Planes[1].Normal[1]*position[1])

	// Near plane - normal points away from camera
	nearCenter := lin.Vec2{position[0] + front[0]*near, position[1] + front[1]*near}
	frustum.Planes[2].Normal = front
	frustum.Planes[2].D = -(frustum.Planes[2].Normal[0]*nearCenter[0] + frustum.Planes[2].Normal[1]*nearCenter[1])

	// Far plane - normal points toward camera
	farCenter := lin.Vec2{position[0] + front[0]*far, position[1] + front[1]*far}
	frustum.Planes[3].Normal = lin.Vec2{-front[0], -front[1]}
	frustum.Planes[3].D = -(frustum.Planes[3].Normal[0]*farCenter[0] + frustum.Planes[3].Normal[1]*farCenter[1])

	return frustum
}

// NewFrustumFromVectors creates a 2D frustum from camera position and front vector
func NewFrustumFromVectors(position lin.Vec2, front lin.Vec2, fov float32, near float32, far float32) Frustum {
	// Calculate direction angle from front vector
	direction := float32(math.Atan2(float64(front[1]), float64(front[0])))
	return NewFrustumFromCamera(position, direction, fov, near, far)
}

// IsChunkInFrustum determines if a chunk is inside the 2D view frustum
func IsChunkInFrustum(chunk *ChunkInfo, frustum Frustum) bool {
	// Define the chunk's 2D AABB
	halfSize := float32(0.5) // Assuming normalized chunk size of 1.0

	// Calculate the extents of the chunk's 2D bounding box
	minX := chunk.WorldPos[0] - halfSize
	maxX := chunk.WorldPos[0] + halfSize
	minZ := chunk.WorldPos[1] - halfSize
	maxZ := chunk.WorldPos[1] + halfSize

	// Check against each frustum plane
	for i := range 4 {
		plane := &frustum.Planes[i]

		// Find the point furthest in the direction of the plane normal (positive vertex)
		var pX, pZ float32
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
func IsPointInFrustum(point lin.Vec2, frustum Frustum) bool {
	for i := range 4 {
		plane := &frustum.Planes[i]

		// If point is outside this plane, it's outside the frustum
		if plane.Normal[0]*point[0]+plane.Normal[1]*point[1]+plane.D < 0 {
			return false
		}
	}
	return true
}
