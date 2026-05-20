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
	"github.com/stretchr/testify/assert"
	lin "github.com/xlab/linmath"
)

// Test configuration
var (
	testStartPos = lin.Vec3{-442800, 162.11664, -172800}
	testStartDir = lin.Vec3{0, 0, -1}
	testConfig   = CameraConfig{
		StartPos:  testStartPos,
		StartDir:  testStartDir,
		EnableGPS: false,
		EnableHat: false,
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

const floatTolerance = 0.05
const eps = 1e-5

func assertVec3InDelta(assert *assert.Assertions, expected, actual lin.Vec3) {
	assert.InDelta(expected[0], actual[0], floatTolerance)
	assert.InDelta(expected[1], actual[1], floatTolerance)
	assert.InDelta(expected[2], actual[2], floatTolerance)
}

func float32Close(a, b float32) bool {
	return math.Abs(float64(a-b)) < eps
}

func vec3Close(a, b lin.Vec3) bool {
	return float32Close(a[0], b[0]) && float32Close(a[1], b[1]) && float32Close(a[2], b[2])
}

func newTestMotionHandler() *MotionHandler {
	posVal := lin.Vec3{0, 0, 0}
	frontVal := lin.Vec3{1, 0, 0}
	rightVal := lin.Vec3{0, 0, 1}
	upVal := lin.Vec3{0, 1, 0}

	cam := &Camera{
		pos:   &posVal,
		front: &frontVal,
		right: &rightVal,
		up:    &upVal,
	}
	cam.sensorManager = &SensorManager{
		camera:           cam,
		smoothingEnabled: true,
	}
	mh := &MotionHandler{
		camera:        cam,
		splineManager: &SplineManager{averageGPSInterval: 1.0},
	}
	cam.motionHandler = mh
	return mh
}

// Camera Initialization Tests

func TestCameraInitialization(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Test position
	pos := cam.Position()
	assertVec3InDelta(assert, testStartPos, pos)

	// Test view matrix is generated
	viewMatrix := cam.ViewMatrix()
	// Just verify it's not zero matrix
	assert.False(viewMatrix[0][0] == 0 && viewMatrix[1][1] == 0 && viewMatrix[2][2] == 0)

	// Test position string generation
	posStr := cam.GetPositionString()
	assert.NotEmpty(posStr)

	// Test vectors string generation
	vecStr := cam.GetViewVectors()
	assert.NotEmpty(vecStr)
}

func TestCameraConfiguration(t *testing.T) {
	assert := assert.New(t)
	customConfig := CameraConfig{
		StartPos: lin.Vec3{100, 200, 300},
		StartDir: lin.Vec3{1, 0, 0},
	}

	cam := NewCamera(customConfig)

	// Test custom position
	pos := cam.Position()
	assertVec3InDelta(assert, customConfig.StartPos, pos)
}

// Motion and Rotation Tests

func TestRelativeRotation(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Test yaw rotation (90 degrees right)
	cam.motionHandler.ApplyRelativeRotation(math.Pi/2, 0, 0)
	front, _, _ := cam.getVectors()

	// After 90-degree yaw, front should point right (positive X)
	assert.InDelta(1.0, front[0], floatTolerance)
	assert.InDelta(0.0, front[1], floatTolerance)
	assert.InDelta(0.0, front[2], floatTolerance)
}

func TestPitchAfterYaw(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// First rotate 90 degrees right (yaw)
	cam.motionHandler.ApplyRelativeRotation(math.Pi/2, 0, 0)

	// Then pitch up 90 degrees
	cam.motionHandler.ApplyRelativeRotation(0, math.Pi/2, 0)
	front, _, _ := cam.getVectors()

	// After yaw right then pitch up, should be looking up
	assert.InDelta(0.0, front[0], floatTolerance)
	assert.InDelta(1.0, front[1], floatTolerance)
	assert.InDelta(0.0, front[2], floatTolerance)
}

func TestAbsoluteRotation(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Test absolute rotation to specific orientation
	yaw, pitch, roll := math.Pi/4, math.Pi/6, 0.0
	cam.motionHandler.ApplyAbsoluteRotation(yaw, pitch, roll)

	// Verify orientation was set
	actualYaw, actualPitch, actualRoll := cam.getOrientation()
	assert.InDelta(yaw, actualYaw, floatTolerance)
	assert.InDelta(pitch, actualPitch, floatTolerance)
	assert.InDelta(roll, actualRoll, floatTolerance)
}

func TestRollRotation(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Test roll rotation (90 degrees)
	cam.motionHandler.ApplyRelativeRotation(0, 0, math.Pi/2)
	_, _, up := cam.getVectors()

	// After 90-degree roll, up should point left (negative X)
	assert.InDelta(-1.0, up[0], floatTolerance)
	assert.InDelta(0.0, up[1], floatTolerance)
	assert.InDelta(0.0, up[2], floatTolerance)
}

// Position and Movement Tests

func TestPositionUpdates(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Test setting position
	newPos := lin.Vec3{1000, 2000, 3000}
	cam.SetPosition(newPos)

	pos := cam.Position()
	assertVec3InDelta(assert, newPos, pos)
}

func TestMovement(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	initialPos := cam.Position()

	// Test forward movement
	front, _, _ := cam.getVectors()
	delta := lin.Vec3{}
	delta.Scale(&front, 10.0)
	cam.addPosition(delta)

	newPos := cam.Position()

	// Position should have changed
	assert.NotEqual(initialPos, newPos)

	// Test backward movement to return to original
	delta.Scale(&front, -10.0)
	cam.addPosition(delta)

	finalPos := cam.Position()
	assertVec3InDelta(assert, initialPos, finalPos)
}

// GPS Integration Tests

func TestGPSNoFix(t *testing.T) {
	assert := assert.New(t)
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
	assertVec3InDelta(assert, initialPos, currentPos)
}

func TestGPSValidFix(t *testing.T) {
	assert := assert.New(t)
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
	assert.InDelta(expectedX, pos[0], floatTolerance)
	assert.InDelta(expectedY, pos[1], floatTolerance)
	assert.InDelta(expectedZ, pos[2], floatTolerance)
}

func TestOldestOut(t *testing.T) {
	mh := newTestMotionHandler()
	sm := mh.camera.sensorManager

	for i := 0; i < 3; i++ {
		sm.pushGPS(&gpsd.TPVReport{
			Lat:  47.6 + float64(i)*0.0001,
			Lon:  -122.3,
			Alt:  100,
			Mode: gpsd.Mode3D,
		})
		time.Sleep(5 * time.Millisecond)
	}

	secondReadingPos := sm.gpsRecordings[1].position

	sm.pushGPS(&gpsd.TPVReport{
		Lat:  47.6 + 3*0.0001,
		Lon:  -122.3,
		Alt:  100,
		Mode: gpsd.Mode3D,
	})

	if len(sm.gpsRecordings) != 3 {
		t.Errorf("buffer size should remain 3 after shift, got %d", len(sm.gpsRecordings))
	}
	if !vec3Close(sm.gpsRecordings[0].position, secondReadingPos) {
		t.Errorf("after shift, gpsRecordings[0] = %v, expected previous gpsRecordings[1] = %v",
			sm.gpsRecordings[0].position, secondReadingPos)
	}
}

func TestVelocityCalculation(t *testing.T) {
	mh := newTestMotionHandler()
	sm := mh.camera.sensorManager

	sm.pushGPS(&gpsd.TPVReport{Lat: 47.6, Lon: -122.3, Alt: 100, Mode: gpsd.Mode3D})
	time.Sleep(10 * time.Millisecond)

	sm.pushGPS(&gpsd.TPVReport{Lat: 47.6001, Lon: -122.3, Alt: 100, Mode: gpsd.Mode3D})

	if len(sm.gpsRecordings) < 2 {
		t.Fatalf("expected at least 2 readings, got %d", len(sm.gpsRecordings))
	}

	secondVel := sm.gpsRecordings[1].velocity
	if secondVel[0] == 0 && secondVel[1] == 0 && secondVel[2] == 0 {
		t.Errorf("expected non-zero velocity on second reading after position change, got %v", secondVel)
	}
}

func TestSplineBuild(t *testing.T) {
	mh := newTestMotionHandler()
	sm := mh.camera.sensorManager

	if mh.splineManager.currentSplineData != nil {
		t.Errorf("splineData should be nil before any GPS reports")
	}

	for i := 0; i < 4; i++ {
		sm.pushGPS(&gpsd.TPVReport{
			Lat:  47.6 + float64(i)*0.0001,
			Lon:  -122.3,
			Alt:  100,
			Mode: gpsd.Mode3D,
		})
		time.Sleep(5 * time.Millisecond)
	}

	if mh.splineManager.currentSplineData == nil {
		t.Errorf("expected splineData to be populated after 4th pushGPS triggered FeedGPSUpdate, got nil")
	}
}

func TestInsufficientReadings(t *testing.T) {
	mh := newTestMotionHandler()
	sm := mh.camera.sensorManager

	now := time.Now()
	sm.gpsRecordings = []GPSReading{
		{position: lin.Vec3{0, 0, 0}, occurred: now.Add(-time.Second), velocity: lin.Vec3{10, 0, 0}},
		{position: lin.Vec3{10, 0, 0}, occurred: now, velocity: lin.Vec3{10, 0, 0}},
	}

	mh.FeedGPSUpdate()

	if mh.splineManager.currentSplineData != nil {
		t.Errorf("FeedGPSUpdate should not build spline with <3 readings, got %v",
			mh.splineManager.currentSplineData)
	}
}

func TestSplineData(t *testing.T) {
	mh := newTestMotionHandler()
	sm := mh.camera.sensorManager

	now := time.Now()
	sm.gpsRecordings = []GPSReading{
		{position: lin.Vec3{0, 0, 0}, occurred: now.Add(-2 * time.Second), velocity: lin.Vec3{10, 0, 0}},
		{position: lin.Vec3{10, 0, 0}, occurred: now.Add(-1 * time.Second), velocity: lin.Vec3{10, 0, 0}},
		{position: lin.Vec3{20, 0, 0}, occurred: now, velocity: lin.Vec3{10, 0, 0}},
	}

	mh.FeedGPSUpdate()

	spline := mh.splineManager.currentSplineData
	if spline == nil {
		t.Fatalf("expected splineData to be populated, got nil")
	}
	if !vec3Close(spline.p1, mh.camera.Position()) {
		t.Errorf("spline.p1 should match current camera position %v, got %v",
			mh.camera.Position(), spline.p1)
	}
}

func TestFeedLargeError(t *testing.T) {
	mh := newTestMotionHandler()
	sm := mh.camera.sensorManager

	now := time.Now()

	farPos := lin.Vec3{1000, 0, 0}
	sm.gpsRecordings = []GPSReading{
		{position: lin.Vec3{0, 0, 0}, occurred: now.Add(-2 * time.Second), velocity: lin.Vec3{500, 0, 0}},
		{position: lin.Vec3{500, 0, 0}, occurred: now.Add(-1 * time.Second), velocity: lin.Vec3{500, 0, 0}},
		{position: farPos, occurred: now, velocity: lin.Vec3{500, 0, 0}},
	}

	mh.FeedGPSUpdate()

	if !vec3Close(mh.camera.Position(), farPos) {
		t.Errorf("expected camera position to snap to g[2].position %v on large correction, got %v",
			farPos, mh.camera.Position())
	}
}

// Sensor Data Tests

func TestSensorDataStructure(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Set some test orientation
	testYaw, testPitch, testRoll := 1.0, 0.5, -0.3
	cam.setOrientation(testYaw, testPitch, testRoll)

	// Get sensor data
	sensorData := cam.sensorManager.GetSensorData()

	// Verify sensor data structure
	assert.InDelta(testYaw, sensorData.Yaw, floatTolerance)
	assert.InDelta(testPitch, sensorData.Pitch, floatTolerance)
	assert.InDelta(testRoll, sensorData.Roll, floatTolerance)
	assert.True(sensorData.IMUValid)
}

func TestSetSensorData(t *testing.T) {
	assert := assert.New(t)
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

	assert.InDelta(expectedX, pos[0], floatTolerance)
	assert.InDelta(expectedY, pos[1], floatTolerance)
	assert.InDelta(expectedZ, pos[2], floatTolerance)

	// Verify orientation was updated
	yaw, pitch, roll := cam.getOrientation()
	assert.InDelta(testData.Yaw, yaw, floatTolerance)
	assert.InDelta(testData.Pitch, pitch, floatTolerance)
	assert.InDelta(testData.Roll, roll, floatTolerance)
}

// Motion State Tests

func TestMotionState(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Set some known state
	testPos := lin.Vec3{1000, 2000, 3000}
	cam.SetPosition(testPos)

	testYaw, testPitch, testRoll := 1.0, 0.5, -0.3
	cam.motionHandler.SetOrientation(testYaw, testPitch, testRoll)

	// Get motion state
	state := cam.motionHandler.GetMotionState()

	// Verify position
	assertVec3InDelta(assert, testPos, state.Position)

	// Verify orientation
	assert.InDelta(testYaw, state.Orientation.Yaw, floatTolerance)
	assert.InDelta(testPitch, state.Orientation.Pitch, floatTolerance)
	assert.InDelta(testRoll, state.Orientation.Roll, floatTolerance)
}

func TestResetOrientation(t *testing.T) {
	assert := assert.New(t)
	cam := NewCamera(testConfig)

	// Apply some rotation
	cam.motionHandler.ApplyRelativeRotation(math.Pi/2, math.Pi/4, math.Pi/6)

	// Reset orientation
	cam.motionHandler.ResetOrientation()

	// Verify orientation is reset
	yaw, pitch, roll := cam.getOrientation()
	assert.InDelta(0.0, yaw, floatTolerance)
	assert.InDelta(0.0, pitch, floatTolerance)
	assert.InDelta(0.0, roll, floatTolerance)

	// Verify vectors are reset to initial state
	front, right, up := cam.getVectors()
	expectedFront := lin.Vec3{0, 0, -1}
	expectedRight := lin.Vec3{1, 0, 0}
	expectedUp := START_CAMERA_UP

	assertVec3InDelta(assert, expectedFront, front)
	assertVec3InDelta(assert, expectedRight, right)
	assertVec3InDelta(assert, expectedUp, up)
}

// Spline Math Tests

func TestSlerp(t *testing.T) {
	x := lin.Vec3{1, 0, 0}
	z := lin.Vec3{0, 0, 1}

	got := Slerp(x, z, 0)
	if !vec3Close(got, x) {
		t.Errorf("Slerp(x, z, 0) = %v, want %v", got, x)
	}
	got = Slerp(x, z, 1)
	if !vec3Close(got, z) {
		t.Errorf("Slerp(x, z, 1) = %v, want %v", got, z)
	}
	got = Slerp(x, z, 0.5)
	half := float32(math.Sqrt2 / 2)
	want := lin.Vec3{half, 0, half}
	if !vec3Close(got, want) {
		t.Errorf("Slerp(x, z, 0.5) = %v, want %v", got, want)
	}
}

func TestSignedAngle(t *testing.T) {
	axis := lin.Vec3{0, 1, 0}
	x := lin.Vec3{1, 0, 0}
	z := lin.Vec3{0, 0, 1}
	negz := lin.Vec3{0, 0, -1}

	got := SignedAngle(x, z, axis)
	if math.Abs(math.Abs(got)-90) > 1e-3 {
		t.Errorf("SignedAngle(+X, +Z, +Y) magnitude = %v, want 90", got)
	}

	gotOpp := SignedAngle(x, negz, axis)
	if math.Signbit(got) == math.Signbit(gotOpp) {
		t.Errorf("SignedAngle did not flip sign for opposite turn: %v then %v", got, gotOpp)
	}

	gotSame := SignedAngle(x, x, axis)
	if math.Abs(gotSame) > 1e-3 {
		t.Errorf("SignedAngle(v, v) = %v, want ~0", gotSame)
	}
}

func TestGetT(t *testing.T) {
	mh := &MotionHandler{}
	a := lin.Vec3{0, 0, 0}
	b := lin.Vec3{1, 0, 0}

	got := mh.GetT(a, b, 0)
	if math.Abs(got-1.0) > eps {
		t.Errorf("GetT(unit-apart, 0) = %v, want 1.0", got)
	}

	t1 := mh.GetT(a, b, 0)
	t2 := mh.GetT(b, lin.Vec3{3, 0, 0}, t1)
	if t2 <= t1 {
		t.Errorf("knots not monotonic: t1=%v, t2=%v", t1, t2)
	}
}

func TestSplineEndpoints(t *testing.T) {
	mh := &MotionHandler{}
	data := &SplineData{
		p0: lin.Vec3{0, 0, 0},
		p1: lin.Vec3{1, 0, 0},
		p2: lin.Vec3{2, 0, 0},
		p3: lin.Vec3{3, 0, 0},
	}

	got0 := mh.CentCatmullRom(0, data)
	if !vec3Close(got0, data.p1) {
		t.Errorf("CentCatmullRom(0) = %v, want %v (p1)", got0, data.p1)
	}

	got1 := mh.CentCatmullRom(1, data)
	if !vec3Close(got1, data.p2) {
		t.Errorf("CentCatmullRom(1) = %v, want %v (p2)", got1, data.p2)
	}
}

func TestStraightLineInterp(t *testing.T) {
	mh := &MotionHandler{}
	data := &SplineData{
		p0: lin.Vec3{0, 0, 0},
		p1: lin.Vec3{1, 0, 0},
		p2: lin.Vec3{2, 0, 0},
		p3: lin.Vec3{3, 0, 0},
	}
	for _, tParam := range []float64{0.1, 0.25, 0.5, 0.75, 0.9} {
		got := mh.CentCatmullRom(tParam, data)
		if math.Abs(float64(got[1])) > 1e-4 || math.Abs(float64(got[2])) > 1e-4 {
			t.Errorf("CentCatmullRom(%v) drifted off the X-axis: %v", tParam, got)
		}
		if got[0] < 1 || got[0] > 2 {
			t.Errorf("CentCatmullRom(%v).x = %v, expected within [1, 2]", tParam, got[0])
		}
	}
}

// Turn Calculation Tests

func TestTurnStraight(t *testing.T) {
	mh := newTestMotionHandler()
	v := lin.Vec3{10, 0, 0}
	info := mh.CalculateTurn(v, v, lin.Vec3{0, 0, 0})
	if math.Abs(info.angularVel) > 1e-3 {
		t.Errorf("straight motion should give ~0 angular velocity, got %v", info.angularVel)
	}
	if info.turnRadius < 1000 {
		t.Errorf("straight motion should give very large turn radius, got %v", info.turnRadius)
	}
}

func TestRightTurn(t *testing.T) {
	mh := newTestMotionHandler()
	v1 := lin.Vec3{10, 0, 0}
	v2 := lin.Vec3{0, 0, 10}
	info := mh.CalculateTurn(v1, v2, lin.Vec3{0, 0, 0})
	if math.Abs(info.angularVel) < 1 {
		t.Errorf("90° turn should give significant angular velocity, got %v", info.angularVel)
	}
}

func TestTurnRadiusCalculation(t *testing.T) {
	mh := newTestMotionHandler()
	v1 := lin.Vec3{10, 0, 0}
	v2 := lin.Vec3{10, 0, 1}
	accel := lin.Vec3{0, 0, 1}
	info := mh.CalculateTurn(v1, v2, accel)
	if info.turnRadius < 50 || info.turnRadius > 150 {
		t.Errorf("turnRadius for speed≈10 hAccel≈1 should be near 100, got %v", info.turnRadius)
	}
}

// Position Prediction Tests

func TestInsufficientData(t *testing.T) {
	mh := newTestMotionHandler()
	pos := mh.PredictPosition(1.0)
	expected := mh.GetMotionState().Position
	if !vec3Close(pos, expected) {
		t.Errorf("with < 3 GPS readings, expected current pos %v, got %v", expected, pos)
	}
}

func TestStraightLine(t *testing.T) {
	mh := newTestMotionHandler()

	now := time.Now()
	mh.camera.sensorManager.gpsRecordings = []GPSReading{
		{position: lin.Vec3{0, 0, 0}, occurred: now.Add(-2 * time.Second), velocity: lin.Vec3{10, 0, 0}},
		{position: lin.Vec3{10, 0, 0}, occurred: now.Add(-1 * time.Second), velocity: lin.Vec3{10, 0, 0}},
		{position: lin.Vec3{20, 0, 0}, occurred: now, velocity: lin.Vec3{10, 0, 0}},
	}

	pred := mh.PredictPosition(1.0)
	expected := lin.Vec3{30, 0, 0}
	if !vec3Close(pred, expected) {
		t.Errorf("straight-line prediction = %v, want %v", pred, expected)
	}
}

func TestPredictCurved(t *testing.T) {
	mh := newTestMotionHandler()
	info := &TurnInfo{
		angularVel: 0.5,
		turnRadius: 10000,
		curvedDir:  lin.Vec3{1, 0, 0},
	}
	vel := lin.Vec3{10, 0, 0}
	start := lin.Vec3{0, 0, 0}
	pred := mh.PredictCurved(start, vel, info, 1.0)
	if math.Abs(float64(pred[0])-10) > 1 {
		t.Errorf("nearly-straight curved prediction X = %v, expected near 10", pred[0])
	}
	if math.Abs(float64(pred[2])) > 1 {
		t.Errorf("nearly-straight curved prediction Z = %v, expected near 0", pred[2])
	}
}
