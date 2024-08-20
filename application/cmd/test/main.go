package main

import (
	"fmt"
	"log"

	"github.com/cs-wwu/srg-synvis/internal/renderer"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func printTest(failed bool, failures []string, successMsg string) {
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

	return failed, failures , successMsg
}

func VulkanMathTest() {
	n48w123_coord := "n48w123"

	w123_xValues := []int{-442800, -440000, -439201}
	n48_yValues := []int{176400, 174000, 172801}

	printTest(testCoords(n48w123_coord, w123_xValues, n48_yValues))

	// Inner quadrants
	n00w001_coord := "n00w001"
	w001_xValues := []int{-3600, -1000, -1}
	n00_yValues := []int{3600, 1000, 1}

	n00e000_coord := "n00e000"
	e000_xValues := []int{0, 1000, 3599}

	s01w001_coord := "s01w001"
	s01_yValues := []int{0, -1000, -3599}

	s01e000_coord := "s01e000"

	// Outer quadrants
	n01w002_coord := "n01w002"
	w002_xValues := []int{-7200, -4000, -3601}
	n01_yValues := []int{3601, 4000, 7200}

	n01e001_coord := "n01e001"
	e001_xValues := []int{3600, 4000, 7199}

	s02w002_coord := "s02w002"
	s02_yValues := []int{-7199, -4000, -3600}

	s02e001_coord := "s02e001"

	printTest(testCoords(n00w001_coord, w001_xValues, n00_yValues))
	printTest(testCoords(n00e000_coord, e000_xValues, n00_yValues))
	printTest(testCoords(s01w001_coord, w001_xValues, s01_yValues))
	printTest(testCoords(s01e000_coord, e000_xValues, s01_yValues))

	printTest(testCoords(n01w002_coord, w002_xValues, n01_yValues))
	printTest(testCoords(n01e001_coord, e001_xValues, n01_yValues))
	printTest(testCoords(s02w002_coord, w002_xValues, s02_yValues))
	printTest(testCoords(s02e001_coord, e001_xValues, s02_yValues))
}

func main() {
	VulkanMathTest()
}
