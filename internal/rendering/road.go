package rendering

import (
	"math"
	"unsafe"

	pb "github.com/cs-wwu/srg-synvis/assets/chunks/roads"
	"github.com/ungerik/go3d/float64/vec2"
	"github.com/ungerik/go3d/vec3"
)

type RoadBuilder struct {
	heightInfo *HeightInfo
	vertices   []RoadVertex
	indices    []uint32

	color vec3.T
}

// TODO: Consolidate? Maybe move this conversion around?
const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0

	ROAD_OFFSET            = float32(1.0)
	SIMPLIFICATION_EPSILON = 0.5 // 0.5 arcseconds (50 feet at the equator)
)

func toWorld(point *pb.Point) vec2.T {
	return vec2.T{
		point.Lon * ARC_DEGREES_TO_SECONDS,
		-point.Lat * ARC_DEGREES_TO_SECONDS,
	}
}

func renderRoad(builder *RoadBuilder, road *pb.Road) {
	builder.color = vec3.T{255, 0, 0}

	points := make([]vec2.T, len(road.Points))
	for i, point := range road.Points {
		points[i] = toWorld(point)
	}

	// Roads from OSM are quite complex, which make them difficult to render from
	// far away. So, we simplify them.
	points = simplify(points, SIMPLIFICATION_EPSILON)

	for i := range points {
		if i == 0 {
			continue
		}

		from := points[i-1]
		to := points[i]

		builder.drawLine(
			from,
			to,
			5.0,
		)
	}
}

// This is the Ramer-Douglas-Peucker algorithm for line simplification.
func simplify(points []vec2.T, epsilon float64) []vec2.T {
	if len(points) < 3 {
		return points
	}

	// Find the point with the maximum distance from the line
	maxDist := 0.0
	index := 0
	for i := 1; i < len(points)-1; i++ {
		dist := perpendicularDistance(points[i], points[0], points[len(points)-1])
		if dist > maxDist {
			index = i
			maxDist = dist
		}
	}

	if maxDist > epsilon {
		left := simplify(points[:index+1], epsilon)
		right := simplify(points[index:], epsilon)

		// Combine results, remove duplicate at index
		return append(left[:len(left)-1], right...)
	}

	return []vec2.T{points[0], points[len(points)-1]}
}

func perpendicularDistance(pt, lineStart, lineEnd vec2.T) float64 {
	if lineStart == lineEnd {
		dx := pt[0] - lineStart[0]
		dy := pt[1] - lineStart[1]
		return math.Hypot(dx, dy)
	}

	num := math.Abs((lineEnd[1]-lineStart[1])*pt[0] - (lineEnd[0]-lineStart[0])*pt[1] + lineEnd[0]*lineStart[1] - lineEnd[1]*lineStart[0])
	den := math.Hypot(lineEnd[0]-lineStart[0], lineEnd[1]-lineStart[1])
	return num / den
}

func newRoadBuilder(heightInfo *HeightInfo) *RoadBuilder {
	return &RoadBuilder{
		heightInfo: heightInfo,
		vertices:   make([]RoadVertex, 0),
		indices:    make([]uint32, 0),
	}
}

func (r *RoadBuilder) drawLine(from, to vec2.T, width float64) {
	dir := to.Subed(&from)
	dir.Normalize()

	perp := vec2.T{-dir[1], dir[0]}

	a := r.addPoint(vec2.T{from[0] - perp[0]*width, from[1] - perp[1]*width})
	b := r.addPoint(vec2.T{from[0] + perp[0]*width, from[1] + perp[1]*width})
	c := r.addPoint(vec2.T{to[0] - perp[0]*width, to[1] - perp[1]*width})
	d := r.addPoint(vec2.T{to[0] + perp[0]*width, to[1] + perp[1]*width})

	r.addTriangle(a, b, c)
	r.addTriangle(b, d, c)
}

func (r *RoadBuilder) addPoint(point vec2.T) uint32 {
	height := r.heightInfo.Height(float32(point[0]), float32(point[1]))
	height += ROAD_OFFSET
	return r.addVertex(RoadVertex{
		Position: vec3.T{float32(point[0]), height, float32(point[1])},
		Color:    r.color,
	})
}

func (r *RoadBuilder) addVertex(vertex RoadVertex) (id uint32) {
	id = uint32(len(r.vertices))
	r.vertices = append(r.vertices, vertex)
	return
}

func (r *RoadBuilder) addTriangle(a, b, c uint32) {
	r.indices = append(r.indices, a, b, c)
}

// SAFETY: The returned slice will become invalid after the `RoadBuilder` is freed.
func (r *RoadBuilder) vertexBytes() []byte {
	if len(r.vertices) == 0 {
		return nil
	}

	size := len(r.vertices) * int(unsafe.Sizeof(RoadVertex{}))
	return unsafe.Slice((*byte)(unsafe.Pointer(&r.vertices[0])), size)
}

// SAFETY: The returned slice will become invalid after the `RoadBuilder` is freed.
func (r *RoadBuilder) indexBytes() []byte {
	if len(r.indices) == 0 {
		return nil
	}

	size := len(r.indices) * int(unsafe.Sizeof(uint32(0)))
	return unsafe.Slice((*byte)(unsafe.Pointer(&r.indices[0])), size)
}
