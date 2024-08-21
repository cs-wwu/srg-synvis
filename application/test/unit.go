// Package test for testing the multiple aspects of the svs application
package test

import (
	"fmt"

	"github.com/cs-wwu/srg-synvis/internal/renderer"
)

// This file is for unit tests
func printTestCoords(failed bool, failures []string, successMsg string) {
	if failed {
		for _, failure := range failures {
			fmt.Println(failure)
		}
	} else {
		fmt.Println(successMsg)
	}
	fmt.Println()
}

func testCoords(coord string, xValues, yValues []int) (bool, []string, string) {
	failures := []string{}
	var failure string
	failed := false

	resultX, resultY := renderer.CoordinateMapToInt(coord)
	resultCoord := renderer.CoordinateIntToMap(resultX, resultY)

	if resultCoord != coord {
		failure = fmt.Sprintf("Failed test (%s != %s): %s -> (%d, %d) -> %s", coord, resultCoord, coord, resultX, resultY, resultCoord)
		failures = append(failures, failure)
		failed = true
	}

	for _, x := range xValues {
		for _, y := range yValues {
			resultCoord = renderer.CoordinateIntToMap(x, y)
			if resultCoord != coord {
				failure = fmt.Sprintf("Failed test CoordinateIntToMap(%-7d, %-7d) -> %s, should be %s", x, y, resultCoord, coord)
				failures = append(failures, failure)
				failed = true
			}
		}
	}

	successMsg := ""
	if !failed {
		successMsg = fmt.Sprintf("Passed test: %s", coord)
	}

	return failed, failures, successMsg
}

func printTestPixel(failed bool, failure, successMsg string) {
	if failed {
		fmt.Println(failure)
	} else {
		fmt.Println(successMsg)
	}
	fmt.Println()
}

func testCoordinatePixel(xIn, zIn, xOut, zOut int) (bool, string, string) {
	failed := false
	failure := ""
	successMsg := ""

	xRes, zRes := renderer.CoordinateIntToPixel(xIn, zIn)
	if (xRes != xOut) || (zRes != zOut) {
		failed = true
		failure = fmt.Sprintf("Failed test CoordinateIntToPixel(%-7d, %-7d) -> (%-7d, %-7d) should be (%-7d, %-7d)", xIn, zIn, xRes, zRes, xOut, zOut)
	} else {
		successMsg = fmt.Sprintf("Passed test: (%-7d, %-7d)", xIn, zIn)
	}

	return failed, failure, successMsg
}

func VulkanMathTest() {
	n48w123Coord := "n48w123"

	w123xValues := []int{-442800, -440000, -439201}
	n48yValues := []int{176400, 174000, 172801}

	printTestCoords(testCoords(n48w123Coord, w123xValues, n48yValues))

	// Inner quadrants
	n00w001Coord := "n00w001"
	w001xValues := []int{-3600, -1000, -1}
	n00yValues := []int{3600, 1000, 1}

	n00e000Coord := "n00e000"
	e000xValues := []int{0, 1000, 3599}

	s01w001Coord := "s01w001"
	s01yValues := []int{0, -1000, -3599}

	s01e000Coord := "s01e000"

	// Outer quadrants
	n01w002Coord := "n01w002"
	w002xValues := []int{-7200, -4000, -3601}
	n01yValues := []int{3601, 4000, 7200}

	n01e001Coord := "n01e001"
	e001xValues := []int{3600, 4000, 7199}

	s02w002Coord := "s02w002"
	s02yValues := []int{-7199, -4000, -3600}

	s02e001Coord := "s02e001"

	printTestCoords(testCoords(n00w001Coord, w001xValues, n00yValues))
	printTestCoords(testCoords(n00e000Coord, e000xValues, n00yValues))
	printTestCoords(testCoords(s01w001Coord, w001xValues, s01yValues))
	printTestCoords(testCoords(s01e000Coord, e000xValues, s01yValues))

	printTestCoords(testCoords(n01w002Coord, w002xValues, n01yValues))
	printTestCoords(testCoords(n01e001Coord, e001xValues, n01yValues))
	printTestCoords(testCoords(s02w002Coord, w002xValues, s02yValues))
	printTestCoords(testCoords(s02e001Coord, e001xValues, s02yValues))

	printTestPixel(testCoordinatePixel(-3600, 3600, 0, 0))
	printTestPixel(testCoordinatePixel(-3600, 0, 0, 0))
	printTestPixel(testCoordinatePixel(0, 0, 0, 0))
	printTestPixel(testCoordinatePixel(0, 3600, 0, 0))

	printTestPixel(testCoordinatePixel(-3580, 3580, 20, 3580))
	printTestPixel(testCoordinatePixel(-3580, -20, 20, 3580))
	printTestPixel(testCoordinatePixel(20, 3580, 20, 3580))
	printTestPixel(testCoordinatePixel(20, -20, 20, 3580))

	printTestPixel(testCoordinatePixel(-442800, 176400, 0, 0))
	printTestPixel(testCoordinatePixel(-442800, 172800, 0, 0))
	printTestPixel(testCoordinatePixel(-446400, 176400, 0, 0))
	printTestPixel(testCoordinatePixel(-446400, 172800, 0, 0))

	printTestPixel(testCoordinatePixel(-446399, 172801, 1, 1))

	printTestPixel(testCoordinatePixel(100, -100, 100, 3500))
	printTestPixel(testCoordinatePixel(0, -100, 0, 3500))
	printTestPixel(testCoordinatePixel(100, 100, 100, 100))
	printTestPixel(testCoordinatePixel(0, 100, 0, 100))

	printTestPixel(testCoordinatePixel(-100, 100, 3500, 100))
	printTestPixel(testCoordinatePixel(-100, 0, 3500, 0))
}
