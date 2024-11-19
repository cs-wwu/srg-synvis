package main

import (
	"fmt"
	"reflect"
	"unsafe"

	mo "github.com/cs-wwu/srg-synvis/meshopt/meshoptimizer"
)

func main() {
	fmt.Printf("MESHOPTIMIZER_VERSION: %d\n", mo.VERSION)

	runTests()
}

func runTests() {
	testDecodeIndexBuffer()
	testDecodeIndexSequence()
	testDecodeVertexBuffer()
	testSimplify()
	testOptimizeVertexCache()
	testOptimizeOverdraw()
	testBuildMeshlets()
}

func testDecodeIndexBuffer() {
	fmt.Println("Testing Index Buffer Decoding...")

	indexBuffer := []uint32{0, 1, 2, 2, 1, 3, 4, 6, 5, 7, 8, 9}
	encodedData := []byte{
		0xe0, 0xf0, 0x10, 0xfe, 0xff, 0xf0, 0x0c, 0xff, 0x02, 0x02, 0x02, 0x00,
		0x76, 0x87, 0x56, 0x67, 0x78, 0xa9, 0x86, 0x65, 0x89, 0x68, 0x98, 0x01, 0x69, 0x00, 0x00,
	}

	decoded := make([]uint32, len(indexBuffer))
	result := mo.DecodeIndexBuffer(unsafe.Pointer(&decoded[0]), uint64(len(indexBuffer)), uint64(4), encodedData, uint64(len(encodedData)))

	if result != 0 {
		fmt.Println("DecodeIndexBuffer failed")
	} else if !reflect.DeepEqual(decoded, indexBuffer) {
		fmt.Println("DecodeIndexBuffer output doesn't match expected")
	} else {
		fmt.Println("DecodeIndexBuffer passed")
	}
}

func testDecodeIndexSequence() {
	fmt.Println("Testing Index Sequence Decoding...")

	indexSequence := []uint32{0, 1, 51, 2, 49, 1000}
	encodedData := []byte{
		0xd1, 0x00, 0x04, 0xcd, 0x01, 0x04, 0x07, 0x98, 0x1f, 0x00, 0x00, 0x00, 0x00,
	}

	decoded := make([]uint32, len(indexSequence))
	result := mo.DecodeIndexSequence(unsafe.Pointer(&decoded[0]), uint64(len(indexSequence)), uint64(4), encodedData, uint64(len(encodedData)))

	if result != 0 {
		fmt.Println("DecodeIndexSequence failed")
	} else if !reflect.DeepEqual(decoded, indexSequence) {
		fmt.Println("DecodeIndexSequence output doesn't match expected")
	} else {
		fmt.Println("DecodeIndexSequence passed")
	}
}

func testDecodeVertexBuffer() {
	fmt.Println("Testing Vertex Buffer Decoding...")

	type Vertex struct {
		px, py, pz uint16
		nu, nv     uint8
		tx, ty     uint16
	}

	vertexBuffer := []Vertex{
		{0, 0, 0, 0, 0, 0, 0},
		{300, 0, 0, 0, 0, 500, 0},
		{0, 300, 0, 0, 0, 0, 500},
		{300, 300, 0, 0, 0, 500, 500},
	}

	encodedData := []byte{
		0xa0, 0x01, 0x3f, 0x00, 0x00, 0x00, 0x58, 0x57, 0x58, 0x01, 0x26, 0x00, 0x00, 0x00, 0x01,
		0x0c, 0x00, 0x00, 0x00, 0x58, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x3f, 0x00, 0x00, 0x00, 0x17, 0x18, 0x17, 0x01, 0x26, 0x00, 0x00, 0x00, 0x01, 0x0c, 0x00,
		0x00, 0x00, 0x17, 0x01, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	decoded := make([]Vertex, len(vertexBuffer))
	result := mo.DecodeVertexBuffer(unsafe.Pointer(&decoded[0]), uint64(len(vertexBuffer)), uint64(unsafe.Sizeof(Vertex{})), encodedData, uint64(len(encodedData)))

	if result != 0 {
		fmt.Println("DecodeVertexBuffer failed")
	} else if !reflect.DeepEqual(decoded, vertexBuffer) {
		fmt.Println("DecodeVertexBuffer output doesn't match expected")
	} else {
		fmt.Println("DecodeVertexBuffer passed")
	}
}

func testSimplify() {
	fmt.Println("Testing Simplify Flip...")

	vb := []float32{
		1.000000, 1.000000, -1.000000,
		1.000000, 1.000000, 1.000000,
		1.000000, -1.000000, 1.000000,
		1.000000, -0.200000, -0.200000,
		1.000000, 0.200000, -0.200000,
		1.000000, -0.200000, 0.200000,
		1.000000, 0.200000, 0.200000,
		1.000000, 0.500000, -0.500000,
		1.000000, -1.000000, 0.000000,
	}

	ib := []uint32{
		7, 4, 3,
		1, 2, 5,
		7, 1, 6,
		7, 8, 0, // gets removed
		7, 6, 4,
		8, 5, 2,
		8, 7, 3,
		8, 3, 5,
		5, 6, 1,
		7, 0, 1, // gets removed
	}

	expected := []uint32{
		0, 4, 3,
		1, 2, 5,
		0, 1, 6,
		0, 6, 4,
		8, 5, 2,
		8, 0, 3,
		8, 3, 5,
		5, 6, 1,
	}

	result := mo.Simplify(ib, ib, uint64(len(ib)), vb, uint64(len(vb)/3), 12, 3, 1e-3, 0, nil)

	if result != 24 {
		fmt.Printf("Simplify Flip failed: expected 24 indices, got %d\n", result)
	} else if !reflect.DeepEqual(ib[:24], expected) {
		fmt.Println("Simplify Flip failed: output doesn't match expected")
		fmt.Printf("Got:      %v\n", ib[:24])
		fmt.Printf("Expected: %v\n", expected)
	} else {
		fmt.Println("Simplify Flip passed")
	}
}

func testOptimizeVertexCache() {
	fmt.Println("Testing Vertex Cache Optimization...")

	indices := []uint32{0, 1, 2, 0, 2, 3}
	optimized := make([]uint32, len(indices))

	mo.OptimizeVertexCache(optimized, indices, uint64(len(indices)), uint64(4))

	fmt.Println("OptimizeVertexCache completed")
}

func testOptimizeOverdraw() {
	fmt.Println("Testing Overdraw Optimization...")

	vertices := []float32{0, 0, 0, 1, 0, 0, 0, 1, 0, 1, 1, 0}
	indices := []uint32{0, 1, 2, 1, 3, 2}
	optimized := make([]uint32, len(indices))

	mo.OptimizeOverdraw(optimized, indices, uint64(len(indices)), vertices, uint64(len(vertices)/3), uint64(3*4), 1.05)

	fmt.Println("OptimizeOverdraw completed")
}

func testBuildMeshlets() {
	fmt.Println("Testing Meshlet Generation...")

	vertices := []float32{0, 0, 0, 1, 0, 0, 0, 1, 0, 1, 1, 0}
	indices := []uint32{0, 1, 2, 1, 3, 2}

	maxMeshlets := mo.BuildMeshletsBound(uint64(len(indices)), 64, 124)
	meshlets := make([]mo.Meshlet, maxMeshlets)
	meshletVertices := make([]uint32, maxMeshlets*64)
	meshletTriangles := make([]byte, maxMeshlets*124*3)

	meshletCount := mo.BuildMeshlets(meshlets, meshletVertices, meshletTriangles, indices, uint64(len(indices)), vertices, uint64(len(vertices)/3), uint64(3*4), 64, 124, 0.5)

	fmt.Printf("BuildMeshlets generated %d meshlets\n", meshletCount)
}
