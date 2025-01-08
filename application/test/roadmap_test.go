package test

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/cs-wwu/srg-synvis/internal/processor"
)

func TestMain(m *testing.M) {
	m.Run()
}

func TestNoMaps(t *testing.T) {
	err := processor.GenerateOsmPng(nil, "testdir", processor.OSMURL)
	if err == nil {
		t.Errorf("ran on nil, wanted err, got nil")
	}
}

func TestEmptyString(t *testing.T) {
	maps := [][]int{{0, 0}}
	err := processor.GenerateOsmPng(maps, "", processor.OSMURL)
	if err == nil {
		t.Errorf("ran on targetDir='', wanted err, got nil")
	}
}

func TestConvertCoordsToPngIndex(t *testing.T) {

}

func TestItoa(t *testing.T) {
	retA := processor.Itoa(-1)
	if retA != "-1" {
		t.Errorf("ran on -1, wanted '-1', got %s", retA)
	}

	retB := processor.Itoa(0)
	if retB != "0" {
		t.Errorf("ran on 0, wanted '0', got %s", retB)
	}

	retC := processor.Itoa(1)
	if retC != "1" {
		t.Errorf("ran on 1, wanted '1', got %s", retC)
	}
}

func TestCoordToString(t *testing.T) {
	retA := processor.CoordToString(-49, -20)
	if retA != "s49w20" {
		t.Errorf("ran on (-49, -20), wanted 's49w20', got %s", retA)
	}

	retB := processor.CoordToString(49, -20)
	if retB != "n49w20" {
		t.Errorf("ran on (49, -20), wanted 'n49w20', got %s", retB)
	}

	retC := processor.CoordToString(-49, 20)
	if retC != "s49e20" {
		t.Errorf("ran on (-49, 20), wanted 's49e20', got %s", retC)
	}

	retD := processor.CoordToString(49, 20)
	if retD != "n49e20" {
		t.Errorf("ran on (49, 20), wanted 'n49e20', got %s", retD)
	}
}
func TestDrawLine(t *testing.T) {

}
func TestDrawFilledPolygon(t *testing.T) {
	baseImage := image.NewGray(image.Rect(0, 0, 100, 100))
	testImage := image.NewGray(image.Rect(0, 0, 100, 100))
	for y := 1; y <= 11; y++ {
		for x := 1; x <= 11; x++ {
			baseImage.Set(x, y, color.White)
		}
	}
	corners := []processor.Point{
		{X: 1, Y: 1},
		{X: 11, Y: 1},
		{X: 1, Y: 11},
		{X: 11, Y: 11},
	}
	processor.DrawFilledPolygon(testImage, corners, color.White)

	for p := range baseImage.Pix {
		if testImage.Pix[p] == 255 {
			fmt.Printf("%d == %d\n", p, testImage.Pix[p])
		}
		if baseImage.Pix[p] != testImage.Pix[p] {
			t.Errorf("Pixel: %d, %d != %d", p, baseImage.Pix[p], testImage.Pix[p])
		}
	}
}
func TestMakeApiRequest(t *testing.T) {

}
func TestIsPointInPolygon(t *testing.T) {
	polygon := []processor.Point{{X: 0.0, Y: 0.0}, {X: 1.0, Y: 0.0}, {X: 1.0, Y: 1.0}, {X: 0.0, Y: 1.0}}

	retA := processor.IsPointInPolygon(-1, -1, polygon)
	if retA {
		t.Errorf("ran on (-1, -1), wanted false, got true")
	}

	retB := processor.IsPointInPolygon(0, 0, polygon)
	if !retB {
		t.Errorf("ran on (0, 0), wanted true, got false")
	}

	retC := processor.IsPointInPolygon(0.5, 0.5, polygon)
	if !retC {
		t.Errorf("ran on (0.5, 0.5), wanted true, got false")
	}

	retD := processor.IsPointInPolygon(-1.0, 0.5, polygon)
	if retD {
		t.Errorf("ran on (-1.0, 0.5), wanted false, got true")
	}

	retF := processor.IsPointInPolygon(-1.0, 1.0, polygon)
	if retF {
		t.Errorf("ran on (-1.0, 1.0), wanted false got true")
	}
}

func TestDrawRoads(t *testing.T) {

}
