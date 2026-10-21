package camera

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	"github.com/go-gl/glfw/v3.4/glfw"
	lin "github.com/xlab/linmath"
)

// The camera is the view and its associated position/rotation
type Camera struct {
	// Position and orientation vectors
	pos          *lin.Vec3 // (x, y, z)
	lastFramePos lin.Vec3
	up           *lin.Vec3 // Up Direction of the camera
	front        *lin.Vec3 // Front direction of the camera
	right        *lin.Vec3 // Right direction of the Camera

	// Camera parameters
	measuredSpeed float32
	speed         float32 // Camera speed
	sensitivity   float32 // Camera sensitivity

	// Orientation tracking (for sensors)
	pitch float64 // Pitch relative to start
	yaw   float64 // Yaw relative to start
	roll  float64 // Roll angle relative to start

	// Motion parameters
	prevPos     *lin.Vec3
	prevPosTime time.Time
	smoothedVel lin.Vec3
	planePitch  float64 // Pitch angle of the plane based on movement

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

const (
	ROTATION_THRESHOLD = 1e-3
	ROTATION_DAMPING   = 0.075
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
	lastPos := lin.Vec3{pos[0], pos[1], pos[2]}

	// Create camera instance
	camera := &Camera{
		pos:           &pos,
		lastFramePos:  lastPos,
		up:            &up,
		front:         &front,
		right:         &right,
		speed:         speed,
		measuredSpeed: 0,
		sensitivity:   sensitivity,
		pitch:         0.0,
		roll:          0.0,
		yaw:           0.0,
	}

	// Initialize component managers
	camera.inputProcessor = NewInputProcessor(camera)
	camera.sensorManager = NewSensorManager(camera)
	camera.motionHandler = NewMotionHandler(camera)

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
	ctx, cancel := context.WithCancel(context.Background())
	c.sensorManager.ctxGPS = ctx
	c.sensorManager.cancelGPS = cancel
	go c.sensorManager.StartGPS(ctx)
	return nil
}

// Terminates the GPS integration
func (c *Camera) CancelGPS() error {
	c.sensorManager.cancelGPS()
	return nil
}

// EnableSensors starts sensor integration with the specified script
func (c *Camera) EnableSensors() error {
	ctx, cancel := context.WithCancel(context.Background())
	c.sensorManager.ctxHat = ctx
	c.sensorManager.cancelHat = cancel
	go c.sensorManager.StartSensors(ctx)
	return nil
}

// Terimates the Sensor integration
func (c *Camera) CancelSensors() error {
	c.sensorManager.cancelHat()
	return nil
}

func (c *Camera) MeasureSpeed() float32 {
	return c.measuredSpeed
}

// Update performs any per-frame updates
func (c *Camera) Update(deltaTime float32) {
	// Currently no per-frame updates needed
	// This is where future plane mode physics could go

	// Calculate Speed
	var posDelta lin.Vec3
	currentPos := *c.pos

	posDelta.Sub(&currentPos, &c.lastFramePos)
	distance := posDelta.Len()
	c.measuredSpeed = distance / deltaTime
	c.lastFramePos = currentPos

	mh := c.motionHandler
	if mh == nil || mh.splineManager == nil {
		return
	}
	sm := mh.splineManager
	if sm.currentSplineData == nil {
		return
	}
	elapsed := time.Since(sm.splineStartTime).Seconds()
	t := elapsed / sm.averageGPSInterval

	var newPos lin.Vec3
	if t < 1.0 {
		pos := mh.CentCatmullRom(t, sm.currentSplineData)
		newPos = pos
	} else {
		// TODO this is not even really dead reckoning yet
		// timeout/late update logic, should just continue
		// on path for now
		endPos := mh.CentCatmullRom(1.0, sm.currentSplineData)
		overtime := float32(elapsed - sm.averageGPSInterval)
		newPos = lin.Vec3{
			endPos[0] + c.smoothedVel[0]*overtime,
			endPos[1] + c.smoothedVel[1]*overtime,
			endPos[2] + c.smoothedVel[2]*overtime,
		}
		c.updatePosition(newPos[0], newPos[2], newPos[1])
		return
	}
	c.updatePosition(newPos[0], newPos[2], newPos[1])

	// Update the forward vector of the camera
	cur := c.Position()
	if c.prevPos == nil {
		prev := lin.Vec3{cur[0], cur[1], cur[2]}
		c.prevPos = &prev
		c.prevPosTime = time.Now()
	} else {
		deltaPos := lin.Vec3{
			cur[0] - (*c.prevPos)[0],
			cur[1] - (*c.prevPos)[1],
			cur[2] - (*c.prevPos)[2],
		}

		// Calculate horizontal distance for pitch calculation
		horizontalDist := math.Sqrt(float64(deltaPos[0]*deltaPos[0] + deltaPos[2]*deltaPos[2]))

		// Calculate pitch from vertical movement (smoothed)
		if horizontalDist > ROTATION_THRESHOLD {
			targetPitch := math.Atan2(float64(deltaPos[1]), horizontalDist)
			// Smooth the pitch
			c.planePitch = c.planePitch + (targetPitch-c.planePitch)*ROTATION_DAMPING
		}

		deltaPos[1] = 0 // flatten to horizontal

		if float64(deltaPos[0]*deltaPos[0]+deltaPos[2]*deltaPos[2]) > ROTATION_THRESHOLD {
			deltaPos.Norm(&deltaPos)
			smoothedView := lin.Vec3{
				(*c.front)[0] + (deltaPos[0]-(*c.front)[0])*float32(ROTATION_DAMPING),
				(*c.front)[1] + (deltaPos[1]-(*c.front)[1])*float32(ROTATION_DAMPING),
				(*c.front)[2] + (deltaPos[2]-(*c.front)[2])*float32(ROTATION_DAMPING),
			}
			smoothedView.Norm(&smoothedView)

			var right lin.Vec3
			right.MultCross(&smoothedView, &MAP_UP)
			right.Norm(&right)

			var up lin.Vec3
			up.MultCross(&right, &smoothedView)
			up.Norm(&up)

			c.updateVectors(smoothedView, right, up)
		}

		dt := time.Since(c.prevPosTime).Seconds()
		if dt > 0.001 {
			dtf := float32(dt)
			frameVel := lin.Vec3{
				(cur[0] - (*c.prevPos)[0]) / dtf,
				(cur[1] - (*c.prevPos)[1]) / dtf,
				(cur[2] - (*c.prevPos)[2]) / dtf,
			}
			c.smoothedVel[0] = 0.5*frameVel[0] + 0.5*c.smoothedVel[0]
			c.smoothedVel[1] = 0.5*frameVel[1] + 0.5*c.smoothedVel[1]
			c.smoothedVel[2] = 0.5*frameVel[2] + 0.5*c.smoothedVel[2]
		}

		// update baseline for next frame
		(*c.prevPos)[0] = cur[0]
		(*c.prevPos)[1] = cur[1]
		(*c.prevPos)[2] = cur[2]
		c.prevPosTime = time.Now()
	}
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

// Returns basis vectors in an array ordered: Up, Front, Right
func (c *Camera) GetBasis() []lin.Vec3 {
	return []lin.Vec3{*c.up, *c.front, *c.right}
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
