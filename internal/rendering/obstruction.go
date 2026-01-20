package rendering

import (
	"log"
	"math"
	"unsafe"

	pb "github.com/cs-wwu/srg-synvis/assets/chunks/obstructions"
	dvec2 "github.com/ungerik/go3d/float64/vec2"
	dvec3 "github.com/ungerik/go3d/float64/vec3"
	"github.com/ungerik/go3d/vec3"
)

type Shape int

const (
	Pyramid Shape = iota
	Building
	PyramidSmall
	Unmapped
)

var obsShapesMap = map[string]Shape{
	"AG EQUIP":           Unmapped,
	"AMUSEMENT PARK":     Unmapped,
	"ANTENNA":            PyramidSmall,
	"ARCH":               Unmapped,
	"BLDG-TWR":           Unmapped,
	"BRIDGE":             Building,
	"BLDG":               Building,
	"CABLE CAR":          Unmapped, // Cannot find a single existence of this tag in the data - thus, do not know if "CABLE CAR" is accurate string
	"CATENARY":           Unmapped,
	"CTRL TWR":           Pyramid,
	"COOL TWR":           Pyramid,
	"CRANE":              PyramidSmall,
	"DAM":                Building,
	"DOME":               Unmapped,
	"ELEC SYS":           Unmapped,
	"ELEVATOR":           Unmapped,
	"FENCE":              Unmapped,
	"GATE":               Unmapped,
	"GEN UTIL":           Unmapped,
	"GRAIN ELEVATOR":     Unmapped,
	"HANGAR":             Building,
	"HEAT COOL SYSTEM":   Unmapped,
	"LANDFILL":           Building,
	"LIGHTHOUSE":         Unmapped, // Not found in data
	"MET":                Pyramid,
	"MONUMENT":           Unmapped,
	"NATURAL GAS SYSTEM": Unmapped,
	"NAVAID":             Unmapped,
	"PIPELINE PIPE":      Unmapped,
	"PLANT":              Unmapped,
	"POLE":               Pyramid,
	"POWER PLANT":        Building,
	"REFINERY":           Building,
	"RIG":                Building,
	"SHIP":               Unmapped,
	"SIGN":               Unmapped,
	"SILO":               Pyramid,
	"SOLAR PANELS":       Unmapped,
	"SPIRE":              Pyramid,
	"STACK":              Pyramid,
	"STADIUM":            Building,
	"TANK":               Building,
	"BALLOON":            Unmapped,
	"TETRAHEDRON":        Unmapped, // Not found in data
	"TOWER":              Pyramid,
	"TRAMWAY":            Building,
	"TRAMWAY PYLON":      Unmapped, // Not found in data
	"T-L TWR":            Pyramid,
	"UTILITY POLE":       PyramidSmall,
	"VERTICAL STRUCTURE": Building,
	"WALL":               Unmapped,
	"WIND INDICATOR":     PyramidSmall,
	"WINDMILL":           Unmapped,
	"WINDSOCK":           PyramidSmall,
}

type ObstructionBuilder struct {
	heightInfo *HeightInfo
	vertices   []ObstructionVertex
	indices    []uint32

	color vec3.T
}

const (
	OBSTRUCTION_BASE_RADIUS = 3.0
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

	builder.color = vec3.T{0.8, 0.5, 0.2} // orange-ish color

	// Get obstruction shape
	obsShape, ok := obsShapesMap[obstruction.Type]
	if !ok {
		log.Printf("Unaccounted for shape found: %s", obstruction.Type)
	}

	// Get obstruction quantity
	obsNum := obstruction.Quantity

	// Assign radius based on quantity
	obsRadius := OBSTRUCTION_BASE_RADIUS / obsNum

	// If obstruction small pyramid: make radius smaller
	if obsShape == PyramidSmall {
		obsRadius /= 2
		obsShape = Pyramid
	}

	x := float64(toWorldLon(obstruction))
	z := float64(toWorldLat(obstruction))
	height := float64(toWorldHeight(obstruction))
	if int(obsNum) == 1 {
		// Render as usual
		builder.renderObstructionShape(x, z, height, obsShape, obsRadius)
	} else {
		// Otherwise, we must account for quantity
		chunkRadians := 2. / obsNum
		for i := 0; i < int(obsNum); i++ {
			// Split a fictionalized 'circle' around the central point into a number of lines appropriate to how many obstructions there are
			// Obstructions are rendered at the points along this line (with smaller radii)
			radian := chunkRadians * float64(i)
			xAdjustment := OBSTRUCTION_BASE_RADIUS * math.Cos(radian*math.Pi)
			zAdjustment := OBSTRUCTION_BASE_RADIUS * math.Sin(radian*math.Pi)
			if radian > 0.5 && radian < 1 || radian > 1.5 && radian < 2 {
				// switch x and z adjustments
				temp := xAdjustment
				xAdjustment = zAdjustment
				zAdjustment = temp
			}
			if radian > 0.5 && radian < 1.5 {
				// we are subtracting from x
				x -= xAdjustment
			} else {
				// we are adding to x
				x += xAdjustment
			}
			if radian < 1. {
				// we are adding to z
				z += zAdjustment
			} else {
				// we are subtracting from z
				z -= zAdjustment
			}

			// Build obstruction with adjusted location vals
			builder.renderObstructionShape(x, z, height, obsShape, obsRadius)

		}
	}

}

func (builder *ObstructionBuilder) renderObstructionShape(x float64, z float64, height float64, shape Shape, radius float64) {
	xMin := x - radius
	xMax := x + radius
	zMin := z - radius
	zMax := z + radius

	// We make four base points
	b0 := builder.addPoint(dvec2.T{xMax, zMax})
	b1 := builder.addPoint(dvec2.T{xMin, zMax})
	b2 := builder.addPoint(dvec2.T{xMin, zMin})
	b3 := builder.addPoint(dvec2.T{xMax, zMin})

	// Unmapped shapes are now pyramids
	if shape == Unmapped {
		shape = Pyramid
	}

	switch shape {
	case Pyramid:
		// This is a typical pyramid
		// Custom height for apex points
		a0 := builder.addVertex(ObstructionVertex{
			Position: dvec3.T{x, height, z},
			Color:    builder.color,
		})

		// Four triangles for pyramid
		builder.addTriangle(a0, b1, b0)
		builder.addTriangle(a0, b2, b1)
		builder.addTriangle(a0, b3, b2)
		builder.addTriangle(a0, b0, b3)
	case Building:
		// This is a rectangular building
		// Four apex points
		a0 := builder.addVertex(ObstructionVertex{
			Position: dvec3.T{xMax, height, zMax},
			Color:    builder.color,
		})
		a1 := builder.addVertex(ObstructionVertex{
			Position: dvec3.T{xMin, height, zMax},
			Color:    builder.color,
		})
		a2 := builder.addVertex(ObstructionVertex{
			Position: dvec3.T{xMin, height, zMin},
			Color:    builder.color,
		})
		a3 := builder.addVertex(ObstructionVertex{
			Position: dvec3.T{xMax, height, zMin},
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
	default:
		log.Fatalf("Error: got shape type of %d", shape)
	}
}

func (builder *ObstructionBuilder) addPoint(point dvec2.T) uint32 {
	height := builder.heightInfo.Height(point[0], point[1])
	return builder.addVertex(ObstructionVertex{
		Position: dvec3.T{point[0], height, point[1]},
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
