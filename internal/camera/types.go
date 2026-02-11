package camera

import (
	"time"

	"github.com/elokore/glfw/v3.4/glfw"
	dmat4 "github.com/ungerik/go3d/float64/mat4"
	dvec3 "github.com/ungerik/go3d/float64/vec3"
	lin "github.com/xlab/linmath"
)

// Constants
const (
	HEIGHT_CONVERSION = 60.0 / 1852.0
	FPS               = 60 // Frames per second
	SPR               = 2  // Seconds per 360 degree rotation
)

var (
	START_CAMERA_UP  = dvec3.T{0.0, 1.0, 0.0}
	CAMERA_SPEED     = float32(10.0)
	LOOK_SENSITIVITY = float32((1.0 * 3.14159) / (SPR * FPS))
)

// InputState represents the current state of all input sources
type InputState struct {
	// Keyboard inputs
	KeysPressed   map[glfw.Key]bool
	WindowFocused bool

	// For future extensibility
	MouseDelta lin.Vec2
}

// SensorData represents data from hardware sensors
type SensorData struct {
	// GPS Data
	GPSPosition GPSPosition
	GPSValid    bool

	// IMU/Hat Data
	Yaw      float64
	Pitch    float64
	Roll     float64
	IMUValid bool
}

// GPSPosition represents GPS coordinates and metadata
type GPSPosition struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Mode      int // GPS fix mode
}

// Camera interface defines what external systems need from the camera
type Camera interface {
	// Position and orientation
	Position() dvec3.T
	ViewMatrix() dmat4.T
	SetPosition(pos dvec3.T)
	PlaneMatrix(*dmat4.T)

	// Input processing
	ProcessInput(window *glfw.Window)

	// Hardware integration
	EnableGPS() error
	EnableSensors(sensorScript string) error

	// Updates and info
	Update(deltaTime float32)
	GetPositionString() string
	GetViewVectors() string

	MapView() bool
}

// Mode represents different camera operation modes
type Mode int

const (
	StartPipe Mode = iota
	StartCommand
	ReadPipe
	ParseInput
	Waiting
)

// CameraConfig holds camera initialization parameters
type CameraConfig struct {
	StartPos    dvec3.T
	StartDir    dvec3.T
	EnableGPS   bool
	EnableHat   bool
	DecimalYear time.Time
}
