package camera

import (
	"fmt"
	"math"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	"github.com/go-gl/glfw/v3.3/glfw"
	dmat4 "github.com/ungerik/go3d/float64/mat4"
	dvec3 "github.com/ungerik/go3d/float64/vec3"
)

// CameraImpl implements the Camera interface
type CameraImpl struct {
	// Position and orientation vectors
	pos   dvec3.T // (x, y, z)
	up    dvec3.T // Up Direction of the camera
	front dvec3.T // Front direction of the camera
	right dvec3.T // Right direction of the Camera

	// Camera parameters
	speed       float32 // Camera speed
	sensitivity float32 // Camera sensitivity

	// Motion parameters
	prevPos    *lin.Vec3
	planePitch float32 // Pitch angle of the plane based on movement

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

	prevPosTime time.Time
	smoothedVel lin.Vec3
}

var (
	// A vector pointing up in the map view.
	MAP_UP = dvec3.T{0.0, 1.0, 0.0}
	// The direction facing upwards on the screen in the map view.
	MAP_NORTH = dvec3.T{0.0, 0.0, 1.0}
	// Distance above the plane to be in the map view.
<<<<<<< HEAD
	MAP_OFFSET = float32(10.0)
	// Threshold for camera movement to actually update viewport
	ROTATION_THRESHOLD = float32(1e-3)
	// Smoothing for above rotation threshold
	ROTATION_DAMPING = float32(0.075)
=======
	MAP_OFFSET = 10.0
>>>>>>> origin/use-float64
)

// NewCamera creates and initializes a new Camera
func NewCamera(config CameraConfig) Camera {
	// Initialize position and direction vectors
	pos := config.StartPos

	// Calculate initial vectors based on start direction
	front := config.StartDir
	front.Normalize()

	right := dvec3.Cross(&front, &START_CAMERA_UP)
	right.Normalize()

	up := dvec3.Cross(&right, &front)
	up.Normalize()

	// Set defaults if not provided
	speed := CAMERA_SPEED

	sensitivity := LOOK_SENSITIVITY

	// Create camera instance
	camera := &CameraImpl{
		pos:         pos,
		up:          up,
		front:       front,
		right:       right,
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
func (c *CameraImpl) Position() dvec3.T {
	return c.pos
}

func (c *CameraImpl) MapView() bool {
	return c.mapView
}

// ViewMatrix returns the camera's view matrix
func (c *CameraImpl) ViewMatrix() dmat4.T {
	if c.mapView {
<<<<<<< HEAD
		// your existing map view code
		up := lin.Vec3{}
		up.Scale(&MAP_UP, MAP_OFFSET)
		eye := *c.pos
		eye.Add(c.pos, &up)
=======
		up := MAP_UP
		up.Scale(MAP_OFFSET)
		eye := c.pos
		eye.Add(&up)
>>>>>>> origin/use-float64

		forward := dvec3.T{}
		if c.northView {
			forward = MAP_NORTH
		} else {
			forward = c.front
			forward[0] = -forward[0]
			forward[1] = -forward[1]
			forward[2] = -forward[2]
		}

		return LookAt(&eye, &c.pos, &forward)
	} else {
<<<<<<< HEAD
		// base horizontal direction from camera front
		base := lin.Vec3{c.front[0], 0, c.front[2]}
		if base[0]*base[0]+base[2]*base[2] < 1e-8 {
			base = lin.Vec3{0, 0, 1}
		}
		base.Norm(&base)

		// apply planePitch: nose up/down
		cosP := float32(math.Cos(float64(c.planePitch)))
		sinP := float32(math.Sin(float64(c.planePitch)))

		dir := lin.Vec3{
			base[0] * cosP, // keep horizontal X scaled by cos(pitch)
			sinP,           // vertical component from pitch
			base[2] * cosP, // keep horizontal Z scaled by cos(pitch)
		}

		target := lin.Vec3{}
		target.Add(c.pos, &dir)

		worldUp := lin.Vec3{0, 1, 0}
		result.LookAt(c.pos, &target, &worldUp)
=======
		target := c.pos
		target.Add(&c.front)
		return LookAt(&c.pos, &target, &c.up)
>>>>>>> origin/use-float64
	}
}

func LookAt(eye, target, up *dvec3.T) dmat4.T {
	f := target.Subed(eye)
	f.Normalize()
	u := up.Normalized()
	s := dvec3.Cross(&f, &u)
	s.Normalize()
	u = dvec3.Cross(&s, &f)
	u.Normalize()

	m := dmat4.T{}

	m[0][0] = s[0]
	m[1][0] = s[1]
	m[2][0] = s[2]
	m[0][1] = u[0]
	m[1][1] = u[1]
	m[2][1] = u[2]
	m[0][2] = -f[0]
	m[1][2] = -f[1]
	m[2][2] = -f[2]
	m[3][3] = 1
	m[3][0] = -dvec3.Dot(&s, eye)
	m[3][1] = -dvec3.Dot(&u, eye)
	m[3][2] = dvec3.Dot(&f, eye)

	return m
}

const CAMERA_TO_PLANE_DISTANCE = 50.0

func (c *CameraImpl) PlaneMatrix(result *dmat4.T) {
	*result = dmat4.Ident

	// Face forwards.
	result.AssignEulerRotation(math.Pi/2, 0, 0)

	// Scale the plane up
<<<<<<< HEAD
	scale := float32(5)
	result.Mult(result, &lin.Mat4x4{
=======
	scale := 5.0
	result.AssignMul(result, &dmat4.T{
>>>>>>> origin/use-float64
		{scale, 0, 0, 0},
		{0, scale, 0, 0},
		{0, 0, scale, 0},
		{0, 0, 0, 1},
	})

	// Move it towards the camera.
	result[3][2] = -CAMERA_TO_PLANE_DISTANCE

	forward := dvec3.T{}
	if c.northView {
		forward = c.front
		forward[0] = -forward[0]
		forward[1] = -forward[1]
		forward[2] = -forward[2]
	} else {
		forward = MAP_NORTH
	}

<<<<<<< HEAD
	lookAt := lin.Mat4x4{}
	lookAt.LookAt(&lin.Vec3{0, 0, 0}, &forward, &lin.Vec3{0, 1, 0})
	tmp := lin.Mat4x4{}
	tmp.Transpose(&lookAt)
	result.Mult(result, &tmp)
=======
	lookAt := LookAt(&dvec3.T{0, 0, 0}, &forward, &c.up)
	lookAt.Transpose()
	result.AssignMul(result, &lookAt)
>>>>>>> origin/use-float64
}

// SetPosition sets the camera position
func (c *CameraImpl) SetPosition(newPos dvec3.T) {
	c.pos = newPos
}

// ProcessInput handles input from the window
func (c *CameraImpl) ProcessInput(window *glfw.Window) {
	c.inputProcessor.ProcessInput(window)
}

// EnableGPS starts GPS integration
func (c *CameraImpl) EnableGPS() error {
	go c.sensorManager.StartGPS()
	return nil
}

// EnableSensors starts sensor integration with the specified script
func (c *CameraImpl) EnableSensors(sensorScript string) error {
	go c.sensorManager.StartSensors(sensorScript)
	return nil
}

func (c *CameraImpl) Update() {
	mh := c.motionHandler
	if mh == nil || mh.splineManager == nil {
		return
	}

	sm := mh.splineManager
	if sm.currentSplineData == nil {
		return
	}

	elapsed := float32(time.Since(sm.splineStartTime).Seconds())
	t := elapsed / sm.averageGPSInterval

	var newPos lin.Vec3
	if t < 1.0 {
		pos := mh.CentCatmullRom(t, sm.currentSplineData)
		newPos = *pos
	} else {
		// Dead reckon from p2
		dt := elapsed - sm.averageGPSInterval
		var delta lin.Vec3
		delta.Scale(sm.currentVelocity, dt)
		newPos.Add(sm.currentSplineData.p2, &delta)
	}

	c.updatePosition(newPos[0], newPos[2], newPos[1])

	// Update the forward vector of the camera/eye
	cur := c.Position()
	if c.prevPos == nil {
		c.prevPos = &lin.Vec3{cur[0], cur[1], cur[2]}
	} else {
		var d lin.Vec3
		d.Sub(&cur, c.prevPos)

		// Calculate horizontal distance for pitch calculation
		horizontalDist := float32(math.Sqrt(float64(d[0]*d[0] + d[2]*d[2])))

		// Calculate pitch from vertical movement (smoothed)
		if horizontalDist > ROTATION_THRESHOLD {
			targetPitch := float32(math.Atan2(float64(d[1]), float64(horizontalDist)))
			// Smooth the pitch
			c.planePitch = c.planePitch + (targetPitch-c.planePitch)*ROTATION_DAMPING
		}

		d[1] = 0 // flatten to horizontal

		if d[0]*d[0]+d[2]*d[2] > ROTATION_THRESHOLD {
			d.Norm(&d)
			smoothedView := lin.Vec3{
				(*c.front)[0] + (d[0]-(*c.front)[0])*ROTATION_DAMPING,
				(*c.front)[1] + (d[1]-(*c.front)[1])*ROTATION_DAMPING,
				(*c.front)[2] + (d[2]-(*c.front)[2])*ROTATION_DAMPING,
			}
			smoothedView.Norm(&smoothedView)

			worldUp := lin.Vec3{0, 1, 0}
			right := lin.Vec3{}
			right.MultCross(&smoothedView, &worldUp)
			right.Norm(&right)
			up := lin.Vec3{}
			up.MultCross(&right, &smoothedView)
			up.Norm(&up)
			c.updateVectors(smoothedView, right, up)
		}

		// Smoother motion with rolling average for velocity
		if c.prevPos != nil {
			dt := float32(time.Since(c.prevPosTime).Seconds())
			if dt > 0.001 {
				frameVel := lin.Vec3{
					(cur[0] - c.prevPos[0]) / dt,
					(cur[1] - c.prevPos[1]) / dt,
					(cur[2] - c.prevPos[2]) / dt,
				}
				const alpha = float32(0.5)
				c.smoothedVel[0] = alpha*frameVel[0] + (1-alpha)*c.smoothedVel[0]
				c.smoothedVel[1] = alpha*frameVel[1] + (1-alpha)*c.smoothedVel[1]
				c.smoothedVel[2] = alpha*frameVel[2] + (1-alpha)*c.smoothedVel[2]
			}
		}
		// update baseline for next frame
		(*c.prevPos)[0] = cur[0]
		(*c.prevPos)[1] = cur[1]
		(*c.prevPos)[2] = cur[2]
		c.prevPosTime = time.Now()
	}
}

// Starts mock GPS, interpolation testing
func (c *CameraImpl) StartMockGPS(path []lin.Vec3, updateInterval time.Duration) {
	go c.sensorManager.StartMockGPS(path, updateInterval)
}

// GetPositionString returns a human-readable position string
func (c *CameraImpl) GetPositionString() string {
	pos := c.Position()
	gpsCoords := coords.WorldToGPS(float32(pos[0]), float32(pos[2]))
	return fmt.Sprintf("World: (%.1f, %.1f, %.1f), GPS: %s",
		pos[0], pos[1], pos[2], gpsCoords)
}

// GetViewVectors returns a string representation of view vectors
func (c *CameraImpl) GetViewVectors() string {
	return fmt.Sprintf("Up: (%.3f, %.3f, %.3f), Front: (%.3f, %.3f, %.3f), Right: (%.3f, %.3f, %.3f)",
		c.up[0], c.up[1], c.up[2],
		c.front[0], c.front[1], c.front[2],
		c.right[0], c.right[1], c.right[2])
}

// Internal methods for component access

// updatePosition updates the camera position
func (c *CameraImpl) updatePosition(newX, newZ, newY float64) {
	c.pos[0] = newX
	c.pos[2] = newZ
	c.pos[1] = newY
}

// addPosition adds to the camera position
func (c *CameraImpl) addPosition(delta dvec3.T) {
	c.pos.Add(&delta)
}

// subPosition subtracts from the camera position
func (c *CameraImpl) subPosition(delta dvec3.T) {
	c.pos.Sub(&delta)
}

// updateVectors updates the camera orientation vectors
func (c *CameraImpl) updateVectors(front, right, up dvec3.T) {
	c.front = front
	c.right = right
	c.up = up
}

// getVectors returns copies of the current vectors
func (c *CameraImpl) getVectors() (front, right, up dvec3.T) {
	front = c.front
	right = c.right
	up = c.up
	return
}

// setOrientation updates the orientation angles
func (c *CameraImpl) setOrientation(yaw, pitch, roll float64) {
	c.yaw = yaw
	c.pitch = pitch
	c.roll = roll
}

// getOrientation returns the current orientation angles
func (c *CameraImpl) getOrientation() (yaw, pitch, roll float64) {
	return c.yaw, c.pitch, c.roll
}
