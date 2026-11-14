package rendering

import (
	"log"
	"math"
	"unsafe"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/chunk"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"google.golang.org/protobuf/proto"

	pb "github.com/cs-wwu/srg-synvis/assets/chunks/roads"
	"github.com/ungerik/go3d/float64/vec2"
	"github.com/ungerik/go3d/vec3"
)

// RoadVertex is one edge vertex of a road ribbon. Instead of storing the final
// edge position, it stores the road's centerline point plus the direction and
// distance to push sideways. The vertex shader does the pushing so that it can
// widen the road to a minimum on-screen width when the camera is far away.
//
// Memory layout (40 bytes, no padding) must match the pipeline's vertex input:
//
//	location 0: vec3 Position   offset 0
//	location 1: vec3 Color      offset 12
//	location 2: vec2 Normal     offset 24  (NormalX, NormalZ)
//	location 3: vec2 SideHalfW  offset 32  (Side, HalfW)
//
// NOTE: this replaces the existing RoadVertex definition. Delete the old one.
type RoadVertex struct {
	Position vec3.T  // road centerline point (x, height, z)
	Color    vec3.T  //
	NormalX  float32 // unit sideways direction in the xz plane
	NormalZ  float32 //
	Side     float32 // -1 or +1: which side of the centerline this vertex is on
	HalfW    float32 // real half-width of the road in world units
}

type RoadBuilder struct {
	heightInfo *chunk.HeightInfo
	vertices   []RoadVertex
	indices    []uint32

	color vec3.T
}

func toWorld(point *pb.Point) vec2.T {
	return vec2.T{
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

const (
	ROAD_OFFSET            = float32(1.0)
	SIMPLIFICATION_EPSILON = 0.2 // 0.2 arcseconds (20 feet at the equator)
	CORNER_DISTANCE        = 2.0 // base corner arc distance, reset for every corner
)

func (app *Application) buildRoad(pos spatial.MapCoord) *RoadSquare {
	roadFile := "assets/chunks/roads/data/" + pos.String() + ".pb"
	roadSquare := pb.RoadSquare{}
	file, err := FileToBytesChecked(roadFile)
	if err != nil {
		log.Printf("vulkan: error loading road square %s: %v", roadFile, err)
		return nil
	}
	err = proto.Unmarshal(file, &roadSquare)
	if err != nil {
		log.Printf("vulkan: error unmarshaling road square %s: %v", roadFile, err)
		return nil
	}

	mapData, err := app.StreamManager.assetManager.LoadMapData(pos)
	if err != nil {
		log.Printf("vulkan: error loading map data for road square %s: %v", roadFile, err)
		return nil
	}
	heightInfo := chunk.NewHeightInfo(mapData.HeightData)
	builder := newRoadBuilder(heightInfo)

	for _, road := range roadSquare.Roads {
		renderRoad(builder, road)
	}

	// NB: These buffers must be created while `builder` is still referenced,
	// so that the `vertexBytes` and `indexBytes` methods return valid slices.
	vertices := as.CreateBuffer(
		app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		builder.vertexBytes(),
		vk.BufferUsageVertexBufferBit,
	)

	indices := as.CreateBuffer(
		app.Context().Device(),
		app.Context().Platform().MemoryProperties(),
		builder.indexBytes(),
		vk.BufferUsageIndexBufferBit,
	)

	return &RoadSquare{
		vertices:   vertices,
		indices:    indices,
		indexCount: uint32(len(builder.indices)),
	}
}

func renderRoad(builder *RoadBuilder, road *pb.Road) {
	points := make([]vec2.T, len(road.Points))
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
	min_angle := math.Pi / 12 // 15 degrees

	var prev_left, prev_right uint32

	// Start cap. The left vertex sits on the -normal side, the right on +normal.
	{
		p0 := points[0]
		p1 := points[1]
		dir := p1.Subed(&p0)
		dir.Normalize()
		normal := perpendicular(dir)
		prev_left, prev_right = builder.addPair(p0, normal, half_w, -1)
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

		// Clamp: floating point error can push the dot product slightly outside
		// [-1, 1], which would make Acos return NaN.
		dot := math.Max(-1.0, math.Min(1.0, vec2.Dot(&dir1, &dir2)))
		angle := math.Acos(dot)
		half_angle := (math.Pi - angle) / 2

		// Per-corner distance. This used to be shared across the whole road, so
		// one sharp corner changed the arc radius of every later corner.
		distance := CORNER_DISTANCE
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
		if steps < 1 {
			steps = 1
		}
		per_step := 1.0 / float64(steps)

		// With the normal flipped for left-handed turns, the centerline point is
		// origin - norm*distance. The old "outside" edge was that point pushed by
		// -norm*width/2 (side -1), and the old "inside" edge by +norm*width/2
		// (side +1). Which of those is the left vertex depends on handedness.
		leftSide := float32(-1)
		if lefthanded {
			leftSide = 1
		}

		t := 0.0
		for range steps + 1 {
			norm := slerp(norm1, norm2, t)

			norm_scaled := norm.Scaled(distance)
			center := origin.Subed(&norm_scaled)

			curr_left, curr_right := builder.addPair(center, norm, half_w, leftSide)

			builder.addTriangle(prev_left, prev_right, curr_left)
			builder.addTriangle(curr_left, prev_right, curr_right)

			prev_left = curr_left
			prev_right = curr_right

			t += per_step
		}
	}

	// End cap.
	{
		p0 := points[len(points)-2]
		p1 := points[len(points)-1]
		dir := p1.Subed(&p0)
		dir.Normalize()
		normal := perpendicular(dir)
		curr_left, curr_right := builder.addPair(p1, normal, half_w, -1)

		builder.addTriangle(prev_left, prev_right, curr_left)
		builder.addTriangle(curr_left, prev_right, curr_right)
	}
}

func perpendicular(v vec2.T) vec2.T {
	return vec2.T{-v[1], v[0]}
}

func slerp(a, b vec2.T, t float64) vec2.T {
	dot := math.Max(-1.0, math.Min(1.0, vec2.Dot(&a, &b)))
	angle := math.Acos(dot)
	s := math.Sin(angle)
	// Nearly parallel vectors: sin(angle) is ~0 and the formula divides by it.
	if math.Abs(s) < 1e-6 {
		return a
	}
	f1 := math.Sin(angle*(1-t)) / s
	f2 := math.Sin(angle*t) / s
	return vec2.T{a[0]*f1 + b[0]*f2, a[1]*f1 + b[1]*f2}
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

func newRoadBuilder(heightInfo *chunk.HeightInfo) *RoadBuilder {
	return &RoadBuilder{
		heightInfo: heightInfo,
		vertices:   make([]RoadVertex, 0),
		indices:    make([]uint32, 0),
	}
}

// addPair emits the left and right edge vertices for one centerline point and
// returns their indices. Both share the same centerline position (and height
// sampled at it); the vertex shader pushes them apart along `normal`.
//
// leftSide is the sign (-1 or +1) of the side the LEFT vertex sits on along
// `normal`; the right vertex gets the opposite sign.
func (r *RoadBuilder) addPair(center vec2.T, normal vec2.T, halfW float64, leftSide float32) (left, right uint32) {
	height := r.heightInfo.Height(float32(center[0]), float32(center[1]))
	height += ROAD_OFFSET

	emit := func(side float32) uint32 {
		return r.addVertex(RoadVertex{
			Position: vec3.T{float32(center[0]), height, float32(center[1])},
			Color:    r.color,
			NormalX:  float32(normal[0]),
			NormalZ:  float32(normal[1]),
			Side:     side,
			HalfW:    float32(halfW),
		})
	}
	left = emit(leftSide)
	right = emit(-leftSide)
	return
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
