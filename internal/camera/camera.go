package camera

import (
	"fmt"
	"math"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	"github.com/elokore/glfw/v3.4/glfw"
	lin "github.com/xlab/linmath"
)

// Camera implements the Camera interface
type Camera struct {
	// Position and orientation vectors
	pos   *lin.Vec3 // (x, y, z)
	up    *lin.Vec3 // Up Direction of the camera
	front *lin.Vec3 // Front direction of the camera
	right *lin.Vec3 // Right direction of the Camera

	// Camera parameters
	speed       float32 // Camera speed
	sensitivity float32 // Camera sensitivity

	// Orientation tracking (for sensors)
	pitch float64 // Pitch relative to start
	yaw   float64 // Yaw relative to start
	roll  float64 // Roll angle relative to start

	// Configuration
	decimalYear time.Time // Date for magnetic world model

	// Component managers
	inputProcessor *InputProcessor
	sensorManager  *SensorManager
	motionHandler  *MotionHandler

	mapView          bool
	mapButtonPressed bool

	northView          bool
	northButtonPressed bool
}

var (
	// A vector pointing up in the map view.
	MAP_UP = lin.Vec3{0.0, 1.0, 0.0}
	// The direction facing upwards on the screen in the map view.
	MAP_NORTH = lin.Vec3{0.0, 0.0, 1.0}
	// Distance above the plane to be in the map view.
	MAP_OFFSET = float32(10.0)
)

// NewCamera creates and initializes a new Camera
func NewCamera(config CameraConfig) *Camera {
	// Initialize position and direction vectors
	pos := config.StartPos
	up := START_CAMERA_UP

	// Calculate initial vectors based on start direction
	front := config.StartDir
	front.Norm(&front)

	right := lin.Vec3{}
	right.MultCross(&front, &up)
	right.Norm(&right)

	up.MultCross(&right, &front)
	up.Norm(&up)

	// Set defaults if not provided
	speed := CAMERA_SPEED

	sensitivity := LOOK_SENSITIVITY

	// Create camera instance
	camera := &Camera{
		pos:         &pos,
		up:          &up,
		front:       &front,
		right:       &right,
		speed:       speed,
		sensitivity: sensitivity,
		decimalYear: config.DecimalYear,
		pitch:       0.0,
		roll:        0.0,
		yaw:         0.0,
	}

	// Initialize component managers
	camera.inputProcessor = NewInputProcessor(camera)
	camera.sensorManager = NewSensorManager(camera)
	camera.motionHandler = NewMotionHandler(camera)

	// Start hardware integration if requested
	if config.EnableGPS {
		go camera.sensorManager.StartGPS()
	}

	return camera
}

// Position returns the current camera position
func (c *Camera) Position() lin.Vec3 {
	return *c.pos
}

func (c *Camera) Speed() float32 {
	return c.speed
}

func (c *Camera) MapView() bool {
	return c.mapView
}

// ViewMatrix returns the camera's view matrix
func (c *Camera) ViewMatrix() (result lin.Mat4x4) {
	if c.mapView {
		up := lin.Vec3{}
		up.Scale(&MAP_UP, MAP_OFFSET)
		eye := *c.pos
		eye.Add(c.pos, &up)

		forward := lin.Vec3{}
		if c.northView {
			forward = MAP_NORTH
		} else {
			forward = *c.front
			forward[0] = -forward[0]
			forward[1] = -forward[1]
			forward[2] = -forward[2]
		}

		result.LookAt(&eye, c.pos, &forward)
	} else {
		target := lin.Vec3{}
		target.Add(c.pos, c.front)
		result.LookAt(c.pos, &target, c.up)
	}
	return
}

const CAMERA_TO_PLANE_DISTANCE = 50.0

func (c *Camera) PlaneMatrix(result *lin.Mat4x4) {
	result.Identity()

	// Face forwards.
	result.RotateX(result, math.Pi/2)

	// Scale the plane up (NB: `lin.Mat4x4.Scale` does not scale correctly!)
	scale := float32(5)
	result.Mult(result, &lin.Mat4x4{
		{scale, 0, 0, 0},
		{0, scale, 0, 0},
		{0, 0, scale, 0},
		{0, 0, 0, 1},
	})

	// Move it towards the camera.
	result[3][2] = -CAMERA_TO_PLANE_DISTANCE

	forward := lin.Vec3{}
	if c.northView {
		forward = *c.front
		forward[0] = -forward[0]
		forward[1] = -forward[1]
		forward[2] = -forward[2]
	} else {
		forward = MAP_NORTH
	}

	lookAt := lin.Mat4x4{}
	lookAt.LookAt(&lin.Vec3{0, 0, 0}, &forward, c.up)
	tmp := lin.Mat4x4{}
	tmp.Transpose(&lookAt)
	result.Mult(result, &tmp)
}

// SetPosition sets the camera position
func (c *Camera) SetPosition(newPos lin.Vec3) {
	*c.pos = newPos
}

// ProcessInput handles input from the window
func (c *Camera) ProcessInput(window *glfw.Window) {
	c.inputProcessor.ProcessInput(window)
}

// EnableGPS starts GPS integration
func (c *Camera) EnableGPS() error {
	go c.sensorManager.StartGPS()
	return nil
}

// EnableSensors starts sensor integration with the specified script
func (c *Camera) EnableSensors(sensorScript string) error {
	go c.sensorManager.StartSensors(sensorScript)
	return nil
}

// Update performs any per-frame updates
func (c *Camera) Update(deltaTime float32) {
	// Currently no per-frame updates needed
	// This is where future plane mode physics could go
}

// GetPositionString returns a human-readable position string
func (c *Camera) GetPositionString() string {
	pos := c.Position()
	gpsCoords := coords.WorldToGPS(pos[0], pos[2])
	return fmt.Sprintf("World: (%.1f, %.1f, %.1f), GPS: %s",
		pos[0], pos[1], pos[2], gpsCoords)
}

// GetViewVectors returns a string representation of view vectors
func (c *Camera) GetViewVectors() string {
	return fmt.Sprintf("Up: (%.3f, %.3f, %.3f), Front: (%.3f, %.3f, %.3f), Right: (%.3f, %.3f, %.3f)",
		c.up[0], c.up[1], c.up[2],
		c.front[0], c.front[1], c.front[2],
		c.right[0], c.right[1], c.right[2])
}

// Internal methods for component access

// updatePosition updates the camera position (thread-safe)
func (c *Camera) updatePosition(newX, newZ, newY float32) {
	(*c.pos)[0] = newX
	(*c.pos)[2] = newZ
	(*c.pos)[1] = newY
}

// addPosition adds to the camera position (thread-safe)
func (c *Camera) addPosition(delta lin.Vec3) {
	c.pos.Add(c.pos, &delta)
}

// subPosition subtracts from the camera position (thread-safe)
func (c *Camera) subPosition(delta lin.Vec3) {
	c.pos.Sub(c.pos, &delta)
}

// updateVectors updates the camera orientation vectors (thread-safe)
func (c *Camera) updateVectors(front, right, up lin.Vec3) {
	*c.front = front
	*c.right = right
	*c.up = up
}

// getVectors returns copies of the current vectors (thread-safe)
func (c *Camera) getVectors() (front, right, up lin.Vec3) {
	front = *c.front
	right = *c.right
	up = *c.up
	return
}

// setOrientation updates the orientation angles
func (c *Camera) setOrientation(yaw, pitch, roll float64) {
	c.yaw = yaw
	c.pitch = pitch
	c.roll = roll
}

// getOrientation returns the current orientation angles
func (c *Camera) getOrientation() (yaw, pitch, roll float64) {
	return c.yaw, c.pitch, c.roll
}
