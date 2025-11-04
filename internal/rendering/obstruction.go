package rendering

import (
	"unsafe"

	pb "github.com/cs-wwu/srg-synvis/assets/chunks/obstructions"
	"github.com/ungerik/go3d/vec2"
	"github.com/ungerik/go3d/vec3"
)

var second_global int

type ObstructionBuilder struct {
	heightInfo *HeightInfo
	vertices   []ObstructionVertex
	indices    []uint32

	color vec3.T
}

const (
	OBSTRUCTION_BASE_RADIUS = 3.0 // This could (and should) change in the future
)

func toWorldLon(obstruction *pb.FAAPoint) float32 {
	return float32(obstruction.Lon * ARC_DEGREES_TO_SECONDS)
}

func toWorldLat(obstruction *pb.FAAPoint) float32 {
	return float32(-(obstruction.Lat) * ARC_DEGREES_TO_SECONDS)
}

func toWorldHeight(obstruction *pb.FAAPoint) float32 {
	return float32((obstruction.Height * 0.3048) * (ARC_DEGREES_TO_MINUTES / 1852)) // feet to meters to arcseconds
}

func buildObstruction(builder *ObstructionBuilder, obstruction *pb.FAAPoint) {

	builder.color = vec3.T{0.8, 0.5, 0.2} // orange-ish color for now {0.8, 0.5, 0.2}

	// TODO: In future, this could render different shapes for different obstructions
	// For now, it's a rectangle for building or tower, and a pyramid for everything else
	builder.renderObstructionShape(obstruction)
}

func (builder *ObstructionBuilder) renderObstructionShape(obstruction *pb.FAAPoint) {

	x := toWorldLon(obstruction)
	z := toWorldLat(obstruction)
	height := toWorldHeight(obstruction)

	xMin := x - OBSTRUCTION_BASE_RADIUS
	xMax := x + OBSTRUCTION_BASE_RADIUS
	zMin := z - OBSTRUCTION_BASE_RADIUS
	zMax := z + OBSTRUCTION_BASE_RADIUS

	// We make four base points
	b0 := builder.addPoint(vec2.T{float32(xMax), float32(zMax)})
	b1 := builder.addPoint(vec2.T{float32(xMin), float32(zMax)})
	b2 := builder.addPoint(vec2.T{float32(xMin), float32(zMin)})
	b3 := builder.addPoint(vec2.T{float32(xMax), float32(zMin)})

	if obstruction.Type != "TOWER" && obstruction.Type != "BLDG" {

		// Custom height for apex points
		a0 := builder.addVertex(ObstructionVertex{
			Position: vec3.T{float32(x), float32(height), float32(z)},
			Color:    builder.color,
		})

		// Four triangles for pyramid
		builder.addTriangle(a0, b1, b0)
		builder.addTriangle(a0, b2, b1)
		builder.addTriangle(a0, b3, b2)
		builder.addTriangle(a0, b0, b3)

	} else {
		// Four apex points
		a0 := builder.addVertex(ObstructionVertex{
			Position: vec3.T{float32(xMax), float32(height), float32(zMax)},
			Color:    builder.color,
		})
		a1 := builder.addVertex(ObstructionVertex{
			Position: vec3.T{float32(xMin), float32(height), float32(zMax)},
			Color:    builder.color,
		})
		a2 := builder.addVertex(ObstructionVertex{
			Position: vec3.T{float32(xMin), float32(height), float32(zMin)},
			Color:    builder.color,
		})
		a3 := builder.addVertex(ObstructionVertex{
			Position: vec3.T{float32(xMax), float32(height), float32(zMin)},
			Color:    builder.color,
		})
		// Sides of tower
		builder.addTriangle(a0, b1, b0)
		builder.addTriangle(b1, a0, a1)
		builder.addTriangle(a1, b2, b1)
		builder.addTriangle(b2, a1, a2)
		builder.addTriangle(a2, b3, b2)
		builder.addTriangle(b3, a2, a3)
		builder.addTriangle(a3, b0, b3)
		builder.addTriangle(b0, a3, a0)

		// Top of tower
		builder.addTriangle(a1, a3, a2)
		builder.addTriangle(a3, a1, a0)
	}
}

func (builder *ObstructionBuilder) addPoint(point vec2.T) uint32 {
	height := builder.heightInfo.Height(float32(point[0]), float32(point[1]))
	// height += 1.0
	return builder.addVertex(ObstructionVertex{
		Position: vec3.T{float32(point[0]), height, float32(point[1])},
		Color:    builder.color,
	})
}

func (builder *ObstructionBuilder) addVertex(vertex ObstructionVertex) (id uint32) {
	id = uint32(len(builder.vertices))
	builder.vertices = append(builder.vertices, vertex)
	return
}

func (builder *ObstructionBuilder) addTriangle(a, b, c uint32) {
	builder.indices = append(builder.indices, a, b, c)
}

func newObstructionBuilder(heightInfo *HeightInfo) *ObstructionBuilder {
	return &ObstructionBuilder{
		heightInfo: heightInfo,
		vertices:   make([]ObstructionVertex, 0),
		indices:    make([]uint32, 0),
	}
}

func (builder *ObstructionBuilder) vertexBytes() []byte {
	if len(builder.vertices) == 0 {
		return nil
	}

	size := len(builder.vertices) * int(unsafe.Sizeof(ObstructionVertex{}))
	return unsafe.Slice((*byte)(unsafe.Pointer(&builder.vertices[0])), size)
}

func (builder *ObstructionBuilder) indexBytes() []byte {
	if len(builder.indices) == 0 {
		return nil
	}

	size := len(builder.indices) * int(unsafe.Sizeof(uint32(0)))
	return unsafe.Slice((*byte)(unsafe.Pointer(&builder.indices[0])), size)
}
