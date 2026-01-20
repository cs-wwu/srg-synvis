package rendering

import (
	"math"
	"unsafe"

	pb "github.com/cs-wwu/srg-synvis/assets/chunks/roads"
	dvec2 "github.com/ungerik/go3d/float64/vec2"
	dvec3 "github.com/ungerik/go3d/float64/vec3"
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

	ROAD_OFFSET            = 1.0
	SIMPLIFICATION_EPSILON = 0.2 // 0.2 arcseconds (20 feet at the equator)
)

func toWorld(point *pb.Point) dvec2.T {
	return dvec2.T{
		point.Lon * ARC_DEGREES_TO_SECONDS,
		-point.Lat * ARC_DEGREES_TO_SECONDS,
	}
}

type RoadType struct {
	Width float64
	Color vec3.T
}

var ROAD_TYPES = map[string]RoadType{
	// Highway types:
	"motorway":  {Width: 8, Color: vec3.T{0.5, 0.8, 0.9}},
	"trunk":     {Width: 6, Color: vec3.T{0.5, 0.8, 0.9}},
	"primary":   {Width: 4, Color: vec3.T{0.5, 0.8, 0.9}},
	"secondary": {Width: 2, Color: vec3.T{0.5, 0.7, 0.8}},
	"tertiary":  {Width: 1, Color: vec3.T{0.5, 0.6, 0.7}},
	// Railway types:
	"rail":       {Width: 3, Color: vec3.T{0.8, 0.6, 0.5}},
	"light_rail": {Width: 2, Color: vec3.T{0.8, 0.6, 0.5}},
	"tram":       {Width: 1, Color: vec3.T{0.8, 0.6, 0.5}},
	// Aeroway types:
	"runway":   {Width: 6, Color: vec3.T{0.9, 0.5, 0.8}},
	"airstrip": {Width: 5, Color: vec3.T{0.9, 0.5, 0.8}},
	"taxiway":  {Width: 2, Color: vec3.T{0.9, 0.5, 0.8}},
}

func renderRoad(builder *RoadBuilder, road *pb.Road) {
	points := make([]dvec2.T, len(road.Points))
	for i, point := range road.Points {
		points[i] = toWorld(point)
	}

	// Roads from OSM are quite complex, which make them difficult to render from
	// far away. So, we simplify them.
	points = simplify(points, SIMPLIFICATION_EPSILON)

	if len(points) < 2 {
		return
	}

	ty, exists := ROAD_TYPES[road.Type]
	if !exists {
		return
	}

	builder.color = ty.Color
	width := ty.Width
	half_w := width / 2.0
	distance := 2.0
	min_angle := math.Pi / 12 // 15 degrees

	var prev_left, prev_right uint32

	{
		p0 := points[0]
		p1 := points[1]
		dir := p1.Subed(&p0)
		dir.Normalize()
		normal := perpendicular(dir)
		normal_half_w := normal.Scaled(half_w)
		left := p0.Subed(&normal_half_w)
		right := p0.Added(&normal_half_w)
		prev_left = builder.addPoint(left)
		prev_right = builder.addPoint(right)
	}

	for i := range points {
		if i == 0 || i == len(points)-1 {
			continue
		}

		p0 := points[i-1]
		p1 := points[i]
		p2 := points[i+1]
		dir1 := p1.Subed(&p0)
		dir1.Normalize()
		dir2 := p2.Subed(&p1)
		dir2.Normalize()
		norm1 := perpendicular(dir1)
		norm2 := perpendicular(dir2)

		lefthanded := dir1[0]*dir2[1]-dir1[1]*dir2[0] < 0
		if lefthanded {
			norm1.Scale(-1.0)
			norm2.Scale(-1.0)
		}

		angle := math.Acos(dvec2.Dot(&dir1, &dir2))
		half_angle := (math.Pi - angle) / 2
		radius := distance / math.Tan(half_angle)
		if radius > distance*2 {
			radius = distance * 2
			distance = radius * math.Tan(half_angle)
		}
		origin := p1
		dir1_scaled := dir1.Scaled(radius)
		origin.Sub(&dir1_scaled)
		norm1_scaled := norm1.Scaled(distance)
		origin.Add(&norm1_scaled)

		steps := int(math.Ceil(angle / min_angle))
		per_step := 1.0 / float64(steps)

		t := 0.0
		for range steps + 1 {
			norm := slerp(norm1, norm2, t)

			norm_scaled := norm.Scaled(distance + width/2)
			outside := origin.Subed(&norm_scaled)
			norm_scaled = norm.Scaled(distance - width/2)
			inside := origin.Subed(&norm_scaled)

			var left, right dvec2.T
			if lefthanded {
				left = inside
				right = outside
			} else {
				left = outside
				right = inside
			}

			curr_left := builder.addPoint(left)
			curr_right := builder.addPoint(right)

			builder.addTriangle(prev_left, prev_right, curr_left)
			builder.addTriangle(curr_left, prev_right, curr_right)

			prev_left = curr_left
			prev_right = curr_right

			t += per_step
		}
	}

	{
		p0 := points[len(points)-2]
		p1 := points[len(points)-1]
		dir := p1.Subed(&p0)
		dir.Normalize()
		normal := perpendicular(dir)
		normal_half_w := normal.Scaled(half_w)
		left := p1.Subed(&normal_half_w)
		right := p1.Added(&normal_half_w)
		curr_left := builder.addPoint(left)
		curr_right := builder.addPoint(right)

		builder.addTriangle(prev_left, prev_right, curr_left)
		builder.addTriangle(curr_left, prev_right, curr_right)
	}
}

func perpendicular(v dvec2.T) dvec2.T {
	return dvec2.T{-v[1], v[0]}
}

func slerp(a, b dvec2.T, t float64) dvec2.T {
	angle := math.Acos(dvec2.Dot(&a, &b))
	f1 := math.Sin(angle*(1-t)) / math.Sin(angle)
	f2 := math.Sin(angle*t) / math.Sin(angle)
	return dvec2.T{a[0]*f1 + b[0]*f2, a[1]*f1 + b[1]*f2}
}

// This is the Ramer-Douglas-Peucker algorithm for line simplification.
func simplify(points []dvec2.T, epsilon float64) []dvec2.T {
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

	return []dvec2.T{points[0], points[len(points)-1]}
}

func perpendicularDistance(pt, lineStart, lineEnd dvec2.T) float64 {
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

func (r *RoadBuilder) addPoint(point dvec2.T) uint32 {
	height := r.heightInfo.Height(point[0], point[1])
	height += ROAD_OFFSET
	return r.addVertex(RoadVertex{
		Position: dvec3.T{point[0], height, point[1]},
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
