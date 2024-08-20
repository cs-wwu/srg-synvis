package main

import (
	"log"

	"github.com/cs-wwu/srg-synvis/internal/renderer"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func testCoords(coord string, xValues, yValues []int) {
	resultX, resultY := renderer.CoordinateMapToInt(coord)
	resultCoord := renderer.CoordinateIntToMap(resultX, resultY)

	if resultCoord != coord {
		log.Printf("Failed test (%s != %s): %s -> (%d, %d) -> %s\n", coord, resultCoord, coord, resultX, resultY, resultCoord)
	}

	for _, x := range xValues {
		for _, y := range yValues {
			resultCoord = renderer.CoordinateIntToMap(x, y)
			if resultCoord != coord {
				log.Printf("Failed test CoordinateIntToMap(%d, %d) -> %s, should be %s\n", x, y, resultCoord, coord)
			}
		}
	}
}

func VulkanMathTest() {
	n48w123_coord := "n48w123"

	w123_xValues := []int{-442800, -440000, -439201}
	n48_yValues := []int{176400, 174000, 172801}

	n00w001_coord := "n00w001"
	w001_xValues := []int{-3600, -1000, -1}
	n00_yValues := []int{3600, 1000, 1}

	n00e000_coord := "n00e000"
	e000_xValues := []int{0, 1000, 3599}

	s01w001_coord := "s01w001"
	s01_yValues := []int{0, -1000, -3599}

	s01e000_coord := "s01e000"

	testCoords(n48w123_coord, w123_xValues, n48_yValues)
	testCoords(n00w001_coord, w001_xValues, n00_yValues)
	testCoords(n00e000_coord, e000_xValues, n00_yValues)
	testCoords(s01w001_coord, w001_xValues, s01_yValues)
	testCoords(s01e000_coord, e000_xValues, s01_yValues)
}

func main() {
	VulkanMathTest()
}
