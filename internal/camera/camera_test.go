package camera

import (
	"encoding/json"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	"github.com/stratoberry/go-gpsd"
	lin "github.com/xlab/linmath"
)

// Test configuration
var (
	testStartPos = lin.Vec3{-442800, 162.11664, -172800}
	testStartDir = lin.Vec3{0, 0, -1}
	testConfig   = CameraConfig{
		StartPos:    testStartPos,
		StartDir:    testStartDir,
		EnableGPS:   false,
		EnableHat:   false,
	}
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

// accept accepts connections to the mock GPSD server
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

// handleConnection handles starting the MockGPSD connection
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
	tpv := map[string]any{
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

// Close closes the mock GPSD server
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

// Test utility functions

// equal compares two float32 values with tolerance
func equal(a, b float32) bool {
	return math.Abs(float64(a)-float64(b)) <= 0.05
}

// equalVec3 compares two Vec3 with tolerance
func equalVec3(a, b lin.Vec3) bool {
	return equal(a[0], b[0]) && equal(a[1], b[1]) && equal(a[2], b[2])
}

// Camera Initialization Tests

func TestCameraInitialization(t *testing.T) {
	cam := NewCamera(testConfig)

	// Test position
	pos := cam.Position()
	if !equalVec3(pos, testStartPos) {
		t.Errorf("Position not set correctly. Got %v, expected %v", pos, testStartPos)
	}

	// Test view matrix is generated
	viewMatrix := cam.ViewMatrix()
	// Just verify it's not zero matrix
	if viewMatrix[0][0] == 0 && viewMatrix[1][1] == 0 && viewMatrix[2][2] == 0 {
		t.Error("View matrix appears to be zero matrix")
	}

	// Test position string generation
	posStr := cam.GetPositionString()
	if posStr == "" {
		t.Error("Position string should not be empty")
	}

	// Test vectors string generation
	vecStr := cam.GetViewVectors()
	if vecStr == "" {
		t.Error("View vectors string should not be empty")
	}
}

func TestCameraConfiguration(t *testing.T) {
	customConfig := CameraConfig{
		StartPos:    lin.Vec3{100, 200, 300},
		StartDir:    lin.Vec3{1, 0, 0},
	}

	cam := NewCamera(customConfig)

	// Test custom position
	pos := cam.Position()
	if !equalVec3(pos, customConfig.StartPos) {
		t.Errorf("Custom position not set. Got %v, expected %v", pos, customConfig.StartPos)
	}
}

// Motion and Rotation Tests

func TestRelativeRotation(t *testing.T) {
	cam := NewCamera(testConfig)

	// Test yaw rotation (90 degrees right)
	cam.motionHandler.ApplyRelativeRotation(math.Pi/2, 0, 0)
	front, _, _ := cam.getVectors()

	// After 90-degree yaw, front should point right (positive X)
	if !equal(front[0], 1.0) || !equal(front[1], 0.0) || !equal(front[2], 0.0) {
		t.Errorf("Yaw rotation failed. Front vector: [%f, %f, %f], expected [1, 0, 0]",
			front[0], front[1], front[2])
	}
}

func TestPitchAfterYaw(t *testing.T) {
	cam := NewCamera(testConfig)

	// First rotate 90 degrees right (yaw)
	cam.motionHandler.ApplyRelativeRotation(math.Pi/2, 0, 0)

	// Then pitch up 90 degrees
	cam.motionHandler.ApplyRelativeRotation(0, math.Pi/2, 0)
	front, _, _ := cam.getVectors()

	// After yaw right then pitch up, should be looking up
	if !equal(front[0], 0.0) || !equal(front[1], 1.0) || !equal(front[2], 0.0) {
		t.Errorf("Pitch after yaw failed. Front vector: [%f, %f, %f], expected [0, 1, 0]",
			front[0], front[1], front[2])
	}
}

func TestAbsoluteRotation(t *testing.T) {
	cam := NewCamera(testConfig)

	// Test absolute rotation to specific orientation
	yaw, pitch, roll := math.Pi/4, math.Pi/6, 0.0
	cam.motionHandler.ApplyAbsoluteRotation(yaw, pitch, roll)

	// Verify orientation was set
	actualYaw, actualPitch, actualRoll := cam.getOrientation()
	if !equal(float32(actualYaw), float32(yaw)) ||
		!equal(float32(actualPitch), float32(pitch)) ||
		!equal(float32(actualRoll), float32(roll)) {
		t.Errorf("Absolute rotation orientation not set correctly. Got (%f, %f, %f), expected (%f, %f, %f)",
			actualYaw, actualPitch, actualRoll, yaw, pitch, roll)
	}
}

func TestRollRotation(t *testing.T) {
	cam := NewCamera(testConfig)

	// Test roll rotation (90 degrees)
	cam.motionHandler.ApplyRelativeRotation(0, 0, math.Pi/2)
	_, _, up := cam.getVectors()

	// After 90-degree roll, up should point left (negative X)
	if !equal(up[0], -1.0) || !equal(up[1], 0.0) || !equal(up[2], 0.0) {
		t.Errorf("Roll rotation failed. Up vector: [%f, %f, %f], expected [-1, 0, 0]",
			up[0], up[1], up[2])
	}
}

// Position and Movement Tests

func TestPositionUpdates(t *testing.T) {
	cam := NewCamera(testConfig)

	// Test setting position
	newPos := lin.Vec3{1000, 2000, 3000}
	cam.SetPosition(newPos)

	pos := cam.Position()
	if !equalVec3(pos, newPos) {
		t.Errorf("SetPosition failed. Got %v, expected %v", pos, newPos)
	}
}

func TestMovement(t *testing.T) {
	cam := NewCamera(testConfig)

	initialPos := cam.Position()

	// Test forward movement
	front, _, _ := cam.getVectors()
	delta := lin.Vec3{}
	delta.Scale(&front, 10.0)
	cam.addPosition(delta)

	newPos := cam.Position()

	// Position should have changed
	if equalVec3(newPos, initialPos) {
		t.Error("Position should have changed after movement")
	}

	// Test backward movement to return to original
	delta.Scale(&front, -10.0)
	cam.addPosition(delta)

	finalPos := cam.Position()
	if !equalVec3(finalPos, initialPos) {
		t.Errorf("Position should return to original after forward then backward movement. Got %v, expected %v",
			finalPos, initialPos)
	}
}

// GPS Integration Tests

func TestGPSNoFix(t *testing.T) {
	config := testConfig
	config.EnableGPS = true
	cam := NewCamera(config)

	mock := NewMockGPSD(t)
	defer mock.Close()

	initialPos := cam.Position()

	// Disable smoothing for deterministic test results
	cam.sensorManager.SetSmoothing(false)

	// Start GPS in background
	cam.EnableGPS()

	// Wait a moment then send no-fix data
	time.Sleep(100 * time.Millisecond)
	mock.SendTPV(0, 0, 0, gpsd.NoFix)
	time.Sleep(100 * time.Millisecond)

	// Position should not have changed significantly
	currentPos := cam.Position()
	if !equalVec3(currentPos, initialPos) {
		t.Errorf("Camera position should not change with NoFix. Got %v, expected %v",
			currentPos, initialPos)
	}
}

func TestGPSValidFix(t *testing.T) {
	config := testConfig
	config.EnableGPS = true
	cam := NewCamera(config)

	mock := NewMockGPSD(t)
	defer mock.Close()

	// Disable smoothing for deterministic test results
	cam.sensorManager.SetSmoothing(false)

	// Start GPS in background
	cam.EnableGPS()

	// Wait a moment then send valid GPS data
	time.Sleep(100 * time.Millisecond)

	expectedLat := 45.5
	expectedLon := -122.6
	expectedAlt := 100.0

	mock.SendTPV(expectedLat, expectedLon, expectedAlt, gpsd.Mode3D)
	time.Sleep(200 * time.Millisecond)

	// Calculate expected world coordinates
	expectedX, expectedZ := coords.GPSToWorld(float32(expectedLat), float32(expectedLon))
	expectedY := float32(expectedAlt) * HEIGHT_CONVERSION

	pos := cam.Position()
	if !equal(pos[0], expectedX) || !equal(pos[1], expectedY) || !equal(pos[2], expectedZ) {
		t.Errorf("GPS position not updated correctly.\nGot: %v\nExpected: (%f, %f, %f)",
			pos, expectedX, expectedY, expectedZ)
	}
}

// Sensor Data Tests

func TestSensorDataStructure(t *testing.T) {
	cam := NewCamera(testConfig)

	// Set some test orientation
	testYaw, testPitch, testRoll := 1.0, 0.5, -0.3
	cam.setOrientation(testYaw, testPitch, testRoll)

	// Get sensor data
	sensorData := cam.sensorManager.GetSensorData()

	// Verify sensor data structure
	if !equal(float32(sensorData.Yaw), float32(testYaw)) ||
		!equal(float32(sensorData.Pitch), float32(testPitch)) ||
		!equal(float32(sensorData.Roll), float32(testRoll)) {
		t.Errorf("Sensor data orientation incorrect. Got (%f, %f, %f), expected (%f, %f, %f)",
			sensorData.Yaw, sensorData.Pitch, sensorData.Roll, testYaw, testPitch, testRoll)
	}

	if !sensorData.IMUValid {
		t.Error("IMU should be marked as valid when orientation is set")
	}
}

func TestSetSensorData(t *testing.T) {
	cam := NewCamera(testConfig)

	// Create test sensor data
	testData := SensorData{
		GPSPosition: GPSPosition{
			Latitude:  47.6062,
			Longitude: -122.3321,
			Altitude:  100.0,
			Mode:      1,
		},
		GPSValid: true,
		Yaw:      1.57, // 90 degrees
		Pitch:    0.0,
		Roll:     0.0,
		IMUValid: true,
	}

	// Set the sensor data
	cam.sensorManager.SetSensorData(testData)

	// Verify position was updated
	pos := cam.Position()
	expectedX, expectedZ := coords.GPSToWorld(float32(testData.GPSPosition.Latitude),
		float32(testData.GPSPosition.Longitude))
	expectedY := float32(testData.GPSPosition.Altitude) * HEIGHT_CONVERSION

	if !equal(pos[0], expectedX) || !equal(pos[1], expectedY) || !equal(pos[2], expectedZ) {
		t.Errorf("SetSensorData position not updated correctly.\nGot: %v\nExpected: (%f, %f, %f)",
			pos, expectedX, expectedY, expectedZ)
	}

	// Verify orientation was updated
	yaw, pitch, roll := cam.getOrientation()
	if !equal(float32(yaw), float32(testData.Yaw)) ||
		!equal(float32(pitch), float32(testData.Pitch)) ||
		!equal(float32(roll), float32(testData.Roll)) {
		t.Errorf("SetSensorData orientation not updated correctly. Got (%f, %f, %f), expected (%f, %f, %f)",
			yaw, pitch, roll, testData.Yaw, testData.Pitch, testData.Roll)
	}
}

// Motion State Tests

func TestMotionState(t *testing.T) {
	cam := NewCamera(testConfig)

	// Set some known state
	testPos := lin.Vec3{1000, 2000, 3000}
	cam.SetPosition(testPos)

	testYaw, testPitch, testRoll := 1.0, 0.5, -0.3
	cam.motionHandler.SetOrientation(testYaw, testPitch, testRoll)

	// Get motion state
	state := cam.motionHandler.GetMotionState()

	// Verify position
	if !equalVec3(state.Position, testPos) {
		t.Errorf("Motion state position incorrect. Got %v, expected %v", state.Position, testPos)
	}

	// Verify orientation
	if !equal(float32(state.Orientation.Yaw), float32(testYaw)) ||
		!equal(float32(state.Orientation.Pitch), float32(testPitch)) ||
		!equal(float32(state.Orientation.Roll), float32(testRoll)) {
		t.Errorf("Motion state orientation incorrect. Got (%f, %f, %f), expected (%f, %f, %f)",
			state.Orientation.Yaw, state.Orientation.Pitch, state.Orientation.Roll,
			testYaw, testPitch, testRoll)
	}
}

func TestResetOrientation(t *testing.T) {
	cam := NewCamera(testConfig)

	// Apply some rotation
	cam.motionHandler.ApplyRelativeRotation(math.Pi/2, math.Pi/4, math.Pi/6)

	// Reset orientation
	cam.motionHandler.ResetOrientation()

	// Verify orientation is reset
	yaw, pitch, roll := cam.getOrientation()
	if !equal(float32(yaw), 0.0) || !equal(float32(pitch), 0.0) || !equal(float32(roll), 0.0) {
		t.Errorf("Orientation not reset correctly. Got (%f, %f, %f), expected (0, 0, 0)",
			yaw, pitch, roll)
	}

	// Verify vectors are reset to initial state
	front, right, up := cam.getVectors()
	expectedFront := lin.Vec3{0, 0, -1}
	expectedRight := lin.Vec3{1, 0, 0}
	expectedUp := START_CAMERA_UP

	if !equalVec3(front, expectedFront) {
		t.Errorf("Front vector not reset correctly. Got %v, expected %v", front, expectedFront)
	}
	if !equalVec3(right, expectedRight) {
		t.Errorf("Right vector not reset correctly. Got %v, expected %v", right, expectedRight)
	}
	if !equalVec3(up, expectedUp) {
		t.Errorf("Up vector not reset correctly. Got %v, expected %v", up, expectedUp)
	}
}

// Integration Tests

func TestCameraUpdate(t *testing.T) {
	cam := NewCamera(testConfig)

	// Test that update doesn't crash
	cam.Update(1.0 / 60.0) // 60 FPS delta time

	// Currently Update doesn't do much
	pos := cam.Position()
	if equalVec3(pos, lin.Vec3{0, 0, 0}) {
		t.Log("Update method executed without error")
	}
}
