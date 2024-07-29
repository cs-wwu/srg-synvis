package renderer

import (
	"fmt"

	lin "github.com/xlab/linmath"
)

const (
	MAX_HEIGHT = 32768
)

type Plane struct {
	Normal   lin.Vec3
	Distance float32
}

type Frustum struct {
	Planes [6]Plane // Left, Right, Top, Bottom, Near, Far
}

func (f *Frustum) Update(view, proj *lin.Mat4x4) {
	// Combine view and projection matrices
	var clipMatrix lin.Mat4x4
	clipMatrix.Mult(proj, view)

	// Extract frustum planes
	f.Planes[0] = extractPlane(&clipMatrix, 0, 3, -1) // Left
	f.Planes[1] = extractPlane(&clipMatrix, 0, 3, 1)  // Right
	f.Planes[2] = extractPlane(&clipMatrix, 1, 3, -1) // Bottom
	f.Planes[3] = extractPlane(&clipMatrix, 1, 3, 1)  // Top
	f.Planes[4] = extractPlane(&clipMatrix, 2, 3, -1) // Near
	f.Planes[5] = extractPlane(&clipMatrix, 2, 3, 1)  // Far

	// Normalize all plane normals
	for i := range f.Planes {
		f.Planes[i].normalize()
	}
}

func extractPlane(m *lin.Mat4x4, row, col int, sign float32) Plane {
	plane := Plane{
		Normal: lin.Vec3{
			m[0][col] + sign*m[0][row],
			m[1][col] + sign*m[1][row],
			m[2][col] + sign*m[2][row],
		},
		Distance: m[3][col] + sign*m[3][row],
	}
	return plane
}

func (p *Plane) normalize() {
	length := p.Normal.Len()
	p.Normal[0] /= length
	p.Normal[1] /= length
	p.Normal[2] /= length
	p.Distance /= length
}

func (f *Frustum) IsChunkVisible(chunk *Chunk) bool {
	corners := [8]lin.Vec3{
		{float32(chunk.X), 0, float32(chunk.Z)},
		{float32(chunk.X + ChunkSize), 0, float32(chunk.Z)},
		{float32(chunk.X), 0, float32(chunk.Z + ChunkSize)},
		{float32(chunk.X + ChunkSize), 0, float32(chunk.Z + ChunkSize)},
		{float32(chunk.X), MAX_HEIGHT, float32(chunk.Z)},
		{float32(chunk.X + ChunkSize), MAX_HEIGHT, float32(chunk.Z)},
		{float32(chunk.X), MAX_HEIGHT, float32(chunk.Z + ChunkSize)},
		{float32(chunk.X + ChunkSize), MAX_HEIGHT, float32(chunk.Z + ChunkSize)},
	}

	for _, plane := range f.Planes {
		inside := false
		for _, corner := range corners {
			if plane.Normal[0]*corner[0]+plane.Normal[1]*corner[1]+plane.Normal[2]*corner[2]+plane.Distance > 0 {
				inside = true
				break
			}
		}
		if !inside {
			return false
		}
	}
	return true
}

func (app *Application) determineVisibleChunks() {
	// Clear the slice while keeping the underlying array for size
	app.visibleChunks = app.visibleChunks[:0] // Clear the slice while keeping the underlying array

	for _, chunk := range app.loadedChunks {
		if app.frustum.IsChunkVisible(chunk) {
			app.visibleChunks = append(app.visibleChunks, chunk)
		}
	}

	fmt.Printf("Visible chunks: %d\n", len(app.visibleChunks))
}
