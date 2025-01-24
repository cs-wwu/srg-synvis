package test

import (
	"testing"

	"github.com/cs-wwu/srg-synvis/internal/renderer"
	lin "github.com/xlab/linmath"
)

func TestMain(t *testing.T) {
	testCamera := renderer.NewCamera(false, lin.Vec3{-442800, 162.11664, -172800})
	InitCamera(t, testCamera)

	NoFix(t, testCamera)
	WorkingGPS(t, testCamera)

	SenseData(t, testCamera)
	WMM(t, testCamera)

	ProcessInput(t, testCamera)
}

// GPS tests
func NoFix(t *testing.T, c *renderer.Camera) {
	// Run TryGps which should trigger the panic
	err := c.TryGps()

	// Since TryGps calls log.Fatalf, it should cause a panic
	if err == nil {
		t.Errorf("Expected panic due to GPS connection failure, but got error: %v", err)
	}
}

func WorkingGPS(t *testing.T, c *renderer.Camera) {
	// test working gps + buffer
	// test that it updates the correct gps coordinates after getting them
	// check that it runs in a loop
	// check that the buffer works properly

	// check runtime?
}

// Sensehat tests
func SenseData(t *testing.T, c *renderer.Camera) {
	// test if sense fails on bad data
	md := c.TrySense("./badhat.py")
	// want to send [0.596, 0.263] to sense hat and see if that breaks it
	if md != renderer.ParseInput {
		t.Errorf("Ran with bad input, expected renderer.ParseInput mode")
	}

	// test if sense works on good data
	md = c.TrySense("./fakehat.py")
	if md != renderer.Waiting {
		t.Errorf("Ran with good input, expected renderer.Waiting")
	}
}

func WMM(t *testing.T, c *renderer.Camera) {
	c.TrySense("./fakehat2.py")
	// check if bellingham declination is within actualy range (how to determine actual range?)
	// check that front vector is also updated corretly
}

// Camera tests
func InitCamera(t *testing.T, c *renderer.Camera){
	// test pos
	if c.Pos[0] != -442800 || c.Pos[1] != 162.11664 || c.Pos[2] != -172800 {
		t.Errorf("Position vector was not set to startPos values")
	}
	// test decimal_year
	if c.Decimal_year.Year() != 2025 {
		t.Errorf("Date is incorrect")
	}

}

func ProcessInput(t *testing.T, c *renderer.Camera) {
	// test updating position vector if no sense or gps

	// test the validity of a position vector after a sequence of transformations
	// test validity of updating front vector after a sequence of transformations
}
