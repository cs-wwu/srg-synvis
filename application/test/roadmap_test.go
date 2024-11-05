package test

import (
	"testing"

	"github.com/cs-wwu/srg-synvis/internal/processor"
)

func TestMain(m *testing.M) {
	m.Run()
}

func TestNoMaps(t *testing.T) {
	err := processor.GenerateOsmPng(nil, "testdir")
	if err == nil {
		t.Errorf("ran on nil, wanted err, got nil")
	}
}

func TestEmptyString(t *testing.T) {
	maps := [][]int{{0, 0}}
	err := processor.GenerateOsmPng(maps, "")
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

}
func TestMakeApiRequest(t *testing.T) {

}
func TestIsPointInPolygon(t *testing.T) {

}
