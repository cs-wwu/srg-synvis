package renderer

// This file should be used to manipulate the camera position and vectors

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os/exec"
	"sync"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/stratoberry/go-gpsd"
	quat "github.com/ungerik/go3d/float64/quaternion"
	"github.com/ungerik/go3d/float64/vec3"
	"github.com/westphae/geomag/pkg/egm96"
	"github.com/westphae/geomag/pkg/wmm"

	lin "github.com/xlab/linmath"
)

const (
	BUF_SIZE          = 10
	HEIGHT_CONVERSION = 60.0 / 1852.0
)

type Mode int

const (
	StartPipe Mode = iota
	StartCommand
	ReadPipe
	ParseInput
	Waiting
)

var (
	START_CAMERA_UP      = lin.Vec3{0.0, 1.0, 0.0}
	START_CAMERA_FRONT   = lin.Vec3{-0.5, -0.5, -0.7}
	Q_START_CAMERA_UP    = vec3.T{0.0, 1.0, 0.0}
	Q_START_CAMERA_FRONT = vec3.T{-0.5, -0.5, -0.7}
	Q_START_CAMERA_RIGHT = vec3.Cross(&Q_START_CAMERA_UP, &Q_START_CAMERA_FRONT)
	CAMERA_SPEED         = float32(10.0)
	LOOK_SENSITIVITY     = float32(0.1)
)

type Camera struct {
	Pos      *lin.Vec3 // (x, y, z)
	Up       *lin.Vec3 // Up Direction of the camera
	Front    *lin.Vec3 // Front direction of the camera
	Right    *lin.Vec3 // Right direction of the Camera
	WorldUp  *lin.Vec3 // Global up vector
	Rotation *lin.Quat // Quaternion for Rotation

	frontMutex sync.Mutex // Mutex for updating the front vector
	posMutex   sync.Mutex // Mutex for updating the position vector

	Speed       float32 // Camera speed
	Sensitivity float32 // Camera sensitivity

	Decimal_year time.Time // Date for magnetic world
}

// NewCamera creates and initializes a new Camera
func NewCamera(hat bool, startPos lin.Vec3) *Camera {
	today := time.Now()

	pos := startPos
	worldUp := START_CAMERA_UP

	// Create identity quaternion for initial rotation
	var rotation lin.Quat
	rotation.Identity()

	// Calculate initial vectors based on identity rotation
	front := START_CAMERA_FRONT
	right := lin.Vec3{}
	right.MultCross(&front, &worldUp)
	right.Norm(&right)

	up := lin.Vec3{}
	up.MultCross(&right, &front)
	up.Norm(&up)

	log.Printf("Initialized Camera at (%f, %f)\n", pos[0], pos[2])

	return &Camera{
		Pos:          &pos,
		Up:           &up,
		Front:        &front,
		Right:        &right,
		WorldUp:      &worldUp,
		Rotation:     &rotation,
		Speed:        CAMERA_SPEED,
		Sensitivity:  LOOK_SENSITIVITY,
		Decimal_year: today,
	}
}

// GetViewMatrix returns the camera's view matrix
func (c *Camera) GetViewMatrix() lin.Mat4x4 {
	center := lin.Vec3{}
	center.Add(c.Pos, c.Front)

	result := lin.Mat4x4{}
	result.LookAt(c.Pos, &center, c.Up)
	return result
}

// Connect to the gpsd service and get lat, lon, and alt with TPV report until closed
// Run as one go thread separate from main
func (c *Camera) TryGps() error {
	// Connect to gspd
	gps, err := gpsd.Dial("localhost:2947")
	if err != nil {
		// log.Fatalf("Failed to connect to GPSD: %s", err)
		log.Println("Failed to connect to GPSD: ", err)
		return err
	}

	// Set up circular buffer in case of gps fix issues
	var oldX [BUF_SIZE]float32
	var oldY [BUF_SIZE]float32
	var oldZ [BUF_SIZE]float32
	oldX[0] = c.Pos[0]
	oldY[0] = c.Pos[1]
	oldZ[0] = c.Pos[2]
	spot := 0
	timer := 0
	xdif := float32(0)
	ydif := float32(0)
	zdif := float32(0)

	// Set up the TPV function that will run indefintely
	tpvFilter := func(r interface{}) {
		// Get gpsd data
		report := r.(*gpsd.TPVReport)
		log.Println("Location updated:", report.Lat, report.Lon, report.Alt)
		newY := float32(report.Alt) * HEIGHT_CONVERSION
		newX, newZ := CoordinateGPSFloatToFloat(float32(report.Lat), float32(report.Lon))
		// fmt.Println("Location:", report.Lat, report.Lon)

		// Check if data is faulty, if so, assume spot based on history
		// if (report.Lat == 0 && report.Lon == 0) {
		if report.Mode == gpsd.NoFix || report.Mode == gpsd.NoValueSeen {
			log.Println("No fix, using prior data")
			newX = oldX[spot] + xdif
			newY = oldY[spot] + ydif
			newZ = oldZ[spot] + zdif
			timer--
		}

		// Log data in buffer
		timer++
		if timer == 10 {
			timer = 0
			xdif = oldX[spot] - newX
			ydif = oldY[spot] - newY
			zdif = oldZ[spot] - newZ
			spot = (spot + 1) % BUF_SIZE
			oldX[spot] = newX
			oldZ[spot] = newZ
			oldY[spot] = newY
		}

		// Update position
		c.UpdatePos(newX, newZ, newY)
	}
	gps.AddFilter("TPV", tpvFilter)

	// Start watching event stream with channel done to signal stop
	done := gps.Watch()
	<-done

	gps.Close()
	return nil
}

// Gets yaw, pitch and roll from the sense hat via a pipe and updates the front vector indefinitely
// Run as one go thread separate from main
func (c *Camera) TrySense(filename string) Mode {
	var md Mode
	cmd := exec.Command("python3", filename)

	// Create a pipe for standard output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		return StartPipe
	}
	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error starting command:", err)
		return StartCommand
	}

	// Read the output from the pipe
	reader := bufio.NewReader(stdout)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		// Parse the sense data
		var yaw, roll, pitch float64
		n, err := fmt.Sscanf(input, "[%f, %f, %f]", &yaw, &roll, &pitch)
		if err != nil || n != 3 {
			fmt.Println("Error parsing input:", err)
			md = ParseInput
			continue
		}
		// fmt.Println(roll, pitch, yaw)

		// Account for magnetic world deviation

		// LAT, LON, HEIGHT
		loc := egm96.NewLocationGeodetic(-float64(c.Pos[2])/3600, float64(c.Pos[0])/3600, float64(c.Pos[1])/HEIGHT_CONVERSION)
		mag, _ := wmm.CalculateWMMMagneticField(loc, c.Decimal_year)
		fmt.Printf("Declination at your location: %2.2f\n", mag.D())
		yaw = yaw - mag.D() // Substract the declination from yaw
		// TODO inclinication + databackup?

		// Update the front vector with pitch and yaw data
		qRoll := quat.FromAxisAngle(&Q_START_CAMERA_FRONT, roll)
		qYaw := quat.FromAxisAngle(&Q_START_CAMERA_UP, yaw)
		qPitch := quat.FromAxisAngle(&Q_START_CAMERA_RIGHT, pitch)
		qRotate := quat.Mul3(&qRoll, &qYaw, &qPitch)

		front := qRotate.RotatedVec3(&Q_START_CAMERA_FRONT)
		up := qRotate.RotatedVec3(&Q_START_CAMERA_UP)
		right := qRotate.RotatedVec3(&Q_START_CAMERA_RIGHT)

		// Save the newly rotated vectors back into the camera information
		c.Front = &lin.Vec3{float32(front[0]), float32(front[1]), float32(front[2])}
		c.Right = &lin.Vec3{float32(right[0]), float32(right[1]), float32(right[2])}
		c.Up = &lin.Vec3{float32(up[0]), float32(up[1]), float32(up[2])}
	}

	// Print the output
	// fmt.Println("output " + string(output))
	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		fmt.Println("Error waiting for command:", err)
		return Waiting
	}

	return md
}

// Processes input from the glfw window and moves the camera accordingly
// This function is mainly for debugging purposes
// Uses WASD for movement, and arrow keys for viewing
// TODO: turn on / off parts based if getting gps or sense?
func (c *Camera) ProcessInput(window *glfw.Window) {

	if window.GetAttrib(glfw.Focused) == glfw.True {

		tmp := lin.Vec3{}

		// Ctrl+C will kill the program
		if window.GetKey(glfw.KeyLeftControl) == glfw.Press &&
			window.GetKey(glfw.KeyC) == glfw.Press {
			window.SetShouldClose(true) // This will trigger the cleanup in main's event loop
		}

		// Camera Movement
		if window.GetKey(glfw.KeyW) == glfw.Press { // Move Forward
			tmp.Scale(c.Front, c.Speed)
			c.AddPos(tmp)
		}
		if window.GetKey(glfw.KeyS) == glfw.Press { // Move Backward
			tmp.Scale(c.Front, c.Speed)
			c.SubPos(tmp)
		}
		if window.GetKey(glfw.KeyA) == glfw.Press { // Move Left
			tmp.MultCross(c.Front, c.Up)
			tmp.Norm(&tmp)
			tmp.Scale(&tmp, c.Speed)
			c.SubPos(tmp)
		}
		if window.GetKey(glfw.KeyD) == glfw.Press { // Move Right
			tmp.MultCross(c.Front, c.Up)
			tmp.Norm(&tmp)
			tmp.Scale(&tmp, c.Speed)
			c.AddPos(tmp)
		}
		if window.GetKey(glfw.KeySpace) == glfw.Press { // Move Up
			tmp.Scale(c.Up, c.Speed)
			c.AddPos(tmp)
		}
		if window.GetKey(glfw.KeyLeftControl) == glfw.Press { // Move Down
			tmp.Scale(c.Up, c.Speed)
			c.SubPos(tmp)
		}

		yaw := c.getYaw()
		pitch := c.getPitch()
		roll := c.getRoll()

		// Camera Viewing using quaternions
		if window.GetKey(glfw.KeyUp) == glfw.Press { // View Up
			pitch += float64(c.Sensitivity)
		}
		if window.GetKey(glfw.KeyDown) == glfw.Press { // View Down
			pitch -= float64(c.Sensitivity)
		}
		if window.GetKey(glfw.KeyLeft) == glfw.Press { // View Left
			yaw -= float64(c.Sensitivity)
		}
		if window.GetKey(glfw.KeyRight) == glfw.Press { // View Right
			yaw += float64(c.Sensitivity)
		}
		if window.GetKey(glfw.KeyE) == glfw.Press { // Roll Right
			roll += float64(c.Sensitivity)
		}
		if window.GetKey(glfw.KeyQ) == glfw.Press { // Roll Left
			roll -= float64(c.Sensitivity)
		}

		// fmt.Println(roll, pitch, yaw)
		//front := calcFrontVector(pitch, yaw)
		//c.UpdateFront(front)
		c.QuatRotate(pitch, yaw, roll)

		if window.GetKey(glfw.KeyE) == glfw.Press { // Print Debug
			fmt.Printf("Pos: %v\n", c.GetPositionString())
			fmt.Printf("Vectors: %v\n", c.GetViewVectors())
		}
		// TODO: Update the camera's front, right, and up vectors for quaternions
		// remove the current method of clamping top and bottom view vectors,

	}
}

func (c *Camera) GetPositionString() string {
	return fmt.Sprintf("%v, %s", c.Pos, CoordinateFloatToGPS(c.Pos[0], c.Pos[2]))
}

func (c *Camera) GetViewVectors() string {
	return fmt.Sprintf("(Up: %v, Front: %v, Right: %v)", c.Up, c.Front, c.Right)
}

// Calculate the front vector without quaternions, from pitch and yaw
func calcFrontVector(pitch, yaw float64) lin.Vec3 {
	// Clamping to avoid flipping
	if pitch > float64(lin.DegreesToRadians(89.0)) {
		pitch = float64(lin.DegreesToRadians(89.0))
	}
	if pitch < -float64(lin.DegreesToRadians(89)) {
		pitch = -float64(lin.DegreesToRadians(89.0))
	}

	var front lin.Vec3
	front[0] = float32(math.Cos(yaw) * math.Cos(pitch))
	front[1] = float32(math.Sin(pitch))
	front[2] = float32(math.Sin(yaw) * math.Cos(pitch))
	front.Norm(&front)
	return front
}

func (c *Camera) getYaw() float64 { // z
	return math.Atan2(float64(c.Front[2]), float64(c.Front[0]))
	// return math.Asin(-float64(c.Front[1]))
}

func (c *Camera) getPitch() float64 { // y
	return math.Asin(float64(c.Front[1]))
	// return math.Atan2(float64(c.Front[0]), float64(c.Front[2]))
}

func (c *Camera) getRoll() float64 { // x
	// assume up and front vectors are orthonormal
	x0 := lin.Vec3{}
	x0.MultCross(&lin.Vec3{0, 1, 0}, c.Front)
	x0.Norm(&x0)
	y0 := lin.Vec3{}
	y0.MultCross(c.Front, &x0)

	y0yr := dotProduct(y0, *c.Up)
	cos0 := math.Acos(float64(y0yr))
	return math.Asin((float64(y0[2])*cos0 - float64(c.Up[2])) / float64(x0[2]))
}

func dotProduct(v1, v2 lin.Vec3) float32 {
	return v1[0]*v2[0] + v1[1]*v2[1] + v1[2]*v2[2]
}

// RotateQuat rotates a vector by a quaternion
func RotateQuat(vec *lin.Vec3, quat *lin.Quat) lin.Vec3 {
	qv := lin.Quat{vec[0], vec[1], vec[2], 0}
	qConj := lin.Quat{-quat[0], -quat[1], -quat[2], quat[3]}

	tmp := lin.Quat{}
	tmp.Mult(quat, &qv)
	tmp.Mult(&tmp, &qConj)

	return lin.Vec3{tmp[0], tmp[1], tmp[2]}
}

func (c *Camera) UpdateFront(front lin.Vec3) {
	c.frontMutex.Lock()
	c.Front = &front
	c.frontMutex.Unlock()
}

func (c *Camera) UpdatePos(newX, newZ, newY float32) {
	c.posMutex.Lock()
	(*c.Pos)[0] = newX
	(*c.Pos)[2] = newZ
	(*c.Pos)[1] = newY
	c.posMutex.Unlock()
}

func (c *Camera) AddPos(tmp lin.Vec3) {
	c.posMutex.Lock()
	c.Pos.Add(c.Pos, &tmp)
	c.posMutex.Unlock()
}

func (c *Camera) SubPos(tmp lin.Vec3) {
	c.posMutex.Lock()
	c.Pos.Sub(c.Pos, &tmp)
	c.posMutex.Unlock()
}

func (c *Camera) QuatRotate(yaw, roll, pitch float64) {
	// Relative Axis
	front := &vec3.T{float64(c.Front[0]), float64(c.Front[1]), float64(c.Front[2])}
	right := &vec3.T{float64(c.Right[0]), float64(c.Right[1]), float64(c.Right[2])}
	up := &vec3.T{float64(c.Up[0]), float64(c.Up[1]), float64(c.Up[2])}

	// Calculate the rotation quaternions for each component
	qYaw := quat.FromAxisAngle(up, yaw)
	qRoll := quat.FromAxisAngle(front, roll)
	qPitch := quat.FromAxisAngle(right, pitch)

	// Calculate the rotation quaternion from Yaw, Roll, and Pitch
	rotQuat := quat.Mul3(&qYaw, &qRoll, &qPitch)

	// Rotate each vector based on the above quaternion rotation
	rotQuat.RotateVec3(front)
	rotQuat.RotateVec3(right)
	rotQuat.RotateVec3(up)

	// Save the newly rotated vectors back into the camera information
	c.Front = &lin.Vec3{float32(front[0]), float32(front[1]), float32(front[2])}
	c.Right = &lin.Vec3{float32(right[0]), float32(right[1]), float32(right[2])}
	c.Up = &lin.Vec3{float32(up[0]), float32(up[1]), float32(up[2])}
}
