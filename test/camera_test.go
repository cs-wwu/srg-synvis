package test

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/renderer"
	"github.com/stratoberry/go-gpsd"
	lin "github.com/xlab/linmath"
)

// MockGPSD is a test double for the GPSD service
type MockGPSD struct {
	listener    net.Listener
	done        chan bool
	connections []net.Conn
	mu          sync.Mutex
	closed      bool
}

// NewMockGPSD creates and starts a mock GPSD server
func NewMockGPSD(t *testing.T) *MockGPSD {
	listener, err := net.Listen("tcp", "localhost:2947")
	if err != nil {
		t.Fatalf("Failed to start mock GPSD: %v", err)
	}

	mock := &MockGPSD{
		listener:    listener,
		done:        make(chan bool),
		connections: make([]net.Conn, 0),
		closed:      false,
	}

	go mock.accept()
	return mock
}

// Accepts a MockGPSD function and adds it to the connections array
func (m *MockGPSD) accept() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			m.mu.Lock()
			if m.closed {
				m.mu.Unlock()
				return
			}
			m.mu.Unlock()
			continue
		}

		m.mu.Lock()
		m.connections = append(m.connections, conn)
		m.mu.Unlock()

		go m.handleConnection(conn)
	}
}

// Handles starting the MockGPSD connection
func (m *MockGPSD) handleConnection(conn net.Conn) {
	// Send initial GPSD protocol messages
	initMessages := []string{
		`{"class":"VERSION","release":"3.17","rev":"3.17","proto_major":3,"proto_minor":12}`,
		`{"class":"DEVICES","devices":[{"class":"DEVICE","path":"/dev/ttyACM0","driver":"u-blox","subtype":"NEO-M8N"}]}`,
		`{"class":"WATCH","enable":true,"json":true,"nmea":false,"raw":0,"scaled":false,"timing":false,"split24":false,"pps":false}`,
	}

	for _, msg := range initMessages {
		conn.Write([]byte(msg + "\n"))
	}

	// Wait for done signal
	<-m.done
}

// SendTPV sends a full TPV report to all connected clients
func (m *MockGPSD) SendTPV(lat, lon, alt float64, mode gpsd.Mode) {
	tpv := map[string]interface{}{
		"class":  "TPV",
		"device": "/dev/ttyACM0",
		"mode":   mode,
		"time":   time.Now().UTC().Format(time.RFC3339),
		"lat":    lat,
		"lon":    lon,
		"alt":    alt,
		"epx":    10.0,
		"epy":    10.0,
		"epv":    15.0,
		"track":  0.0,
		"speed":  0.0,
		"climb":  0.0,
	}

	jsonData, _ := json.Marshal(tpv)
	msg := string(jsonData) + "\n"

	m.mu.Lock()
	for _, conn := range m.connections {
		conn.Write([]byte(msg))
	}
	m.mu.Unlock()
}

// Close a GPSD connection
func (m *MockGPSD) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	close(m.done)

	for _, conn := range m.connections {
		conn.Close()
	}
	m.connections = nil
	m.listener.Close()
	m.mu.Unlock()
}

// Test functions

// Sensehat tests:
// SenseData tests processing sensehat data correctly
func TestSenseData(t *testing.T) {
	testCamera := renderer.NewCamera(false, lin.Vec3{-442800, 162.11664, -172800})

	// test if sense fails on bad data
	md := testCamera.TrySense("./badhat.py")
	// want to send [0.596, 0.263] to sense hat and see if that breaks it
	if md != renderer.ParseInput {
		t.Errorf("Ran with bad input, expected renderer.ParseInput mode")
	}

	// test if sense works on good data
    // test roll
    fmt.Println("starting roll test")
    testCamera.TrySense("./fakehat.py")
	if !(equal(float32(testCamera.Roll), 0.263)) {
		t.Errorf("Expected rollAngle to be 0.263, roll was %f", testCamera.Roll)
	}
}

// WMM tests if correct declination is found and applied to yaw
func TestWMM(t *testing.T) {
	// Initialize camera to 48.0, -123.0, 1300
	c := renderer.NewCamera(false, lin.Vec3{-442800, 162.11664, -172800})

	c.TrySense("./fakehat2.py")
	// Check if starting coordinate declination is within actual range ~15.30
	
	if !(equal(float32(c.Yaw), 15.30)) {
		t.Errorf("Expected yaw to be updated with declination to 15.30, was %f", c.Yaw)
	}
}

// GPS tests:
// NoFix tests if TryGPS handles when the gps has no fix (position should not change)
func NoFix(t *testing.T) {
	testCamera := renderer.NewCamera(false, lin.Vec3{-442800, 162.11664, -172800})

	mock := NewMockGPSD(t)
	defer mock.Close()
	initialPos := testCamera.Pos

	done := make(chan bool)
	go func() {
		testCamera.TryGps()
		done <- true
	}()

	time.Sleep(500 * time.Millisecond)
	mock.SendTPV(0, 0, 0, gpsd.NoFix)
	time.Sleep(500 * time.Millisecond)

	currentPos := testCamera.Pos
	if !equal(currentPos[0], initialPos[0]) ||
		!equal(currentPos[1], initialPos[1]) ||
		!equal(currentPos[2], initialPos[2]) {
		t.Errorf("Camera position should not change with NoFix. Got %v, expected %v",
			currentPos, initialPos)
	}
}

// WorkingGPS tests that data is correctly parsed and the position vector is updated
func WorkingGPS(t *testing.T) {
	testCamera := renderer.NewCamera(false, lin.Vec3{-442800, 162.11664, -172800})

	mock := NewMockGPSD(t)
	defer mock.Close()

	done := make(chan bool)
	go func() {
		testCamera.TryGps()
		done <- true
	}()

	time.Sleep(500 * time.Millisecond)

	expectedLat := 45.5
	expectedLon := -122.6
	expectedAlt := 100.0

	mock.SendTPV(expectedLat, expectedLon, expectedAlt, gpsd.Mode3D)
	time.Sleep(500 * time.Millisecond)

	expectedX, expectedZ := renderer.CoordinateGPSFloatToFloat(float32(expectedLat), float32(expectedLon))
	expectedY := float32(expectedAlt) * renderer.HEIGHT_CONVERSION

	pos := testCamera.Pos
	if !equal(pos[0], expectedX) || !equal(pos[1], expectedY) || !equal(pos[2], expectedZ) {
		t.Errorf("Camera position not updated correctly.\nGot: %v\nExpected: (%f, %f, %f)",
			pos, expectedX, expectedY, expectedZ)
	}
}

// Camera tests:

func TestInitCamera(t *testing.T) {
	c := renderer.NewCamera(false, lin.Vec3{-442800, 162.11664, -172800})

	// test pos
	if c.Pos[0] != -442800 || c.Pos[1] != 162.11664 || c.Pos[2] != -172800 {
		t.Errorf("Position vector was not set to startPos values")
	}
	// test decimal_year
	if c.Decimal_year.Year() != 2025 {
		t.Errorf("Date is incorrect")
	}
}

func QuatRot(t *testing.T) {
	c := renderer.NewCamera(false, lin.Vec3{-442800, 162.11664, -172800})

	c.QuatRotateRel((math.Pi/2), 0, 0)
	if !equal(c.Front[0], 1.0) || !equal(c.Front[1], 0.0) || !equal(c.Front[2], 0.0) {
		t.Errorf("Front vector is wrong, Front[%f, %f, %f]", c.Front[0], c.Front[1], c.Front[2])
	}
	
	c.QuatRotateRel(0, (math.Pi/2), 0)
	if !equal(c.Front[0], 0) || !equal(c.Front[1], 1) || !equal(c.Front[2], 0) {
		t.Errorf("Front vector is wrong, Front[%f, %f, %f]", c.Front[0], c.Front[1], c.Front[2])
	}
	
	c.QuatRotateRel(0, 0, (math.Pi/2))
	if !(equal(c.Up[0], 0) && equal(c.Up[1], 0) && equal(c.Up[2], -1)) {
		t.Errorf("Up vector is wrong, Up:[%f, %f, %f]", c.Up[0], c.Up[1], c.Up[2])
	}
}

// For floating point comparisons
func equal(a, b float32) bool {
	return math.Abs(float64(a) - float64(b)) <= 0.05
}
