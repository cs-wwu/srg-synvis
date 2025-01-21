package renderer

// This file should be used to manipulate the camera position and vectors

import (
	"bufio"
	_ "embed"
	"fmt"
	"log"
	"math"
	"os/exec"
	"sync"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/stratoberry/go-gpsd"
	lin "github.com/xlab/linmath"
)

var (
	START_CAMERA_UP    = lin.Vec3{0.0, 1.0, 0.0}
	START_CAMERA_FRONT = lin.Vec3{-0.5, -0.5, -0.7}
	CAMERA_SPEED       = float32(10.0)
	LOOK_SENSITIVITY   = float32(0.1)
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
}

// NewCamera creates and initializes a new Camera
func NewCamera(hat bool, startPos lin.Vec3) *Camera {
	pos := startPos
	up := START_CAMERA_UP
	front := START_CAMERA_FRONT
	worldUp := START_CAMERA_UP

	var rotation lin.Quat
	rotation.Identity()

	right := lin.Vec3{}
	right.MultCross(&front, &worldUp)
	right.Norm(&right)

	log.Printf("Initialized Camera at (%f, %f)\n", pos[0], pos[2])

	return &Camera{
		Pos:         &pos,
		Up:          &up,
		Front:       &front,
		Right:       &right,
		WorldUp:     &worldUp,
		Rotation:    &rotation,
		Speed:       CAMERA_SPEED,
		Sensitivity: LOOK_SENSITIVITY,
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

func (c *Camera) TryGps() {
	gps, err := gpsd.Dial("localhost:2947")
	if err != nil {
		log.Fatalf("Failed to connect to GPSD: %s", err)
	}

	tpvFilter := func(r interface{}) {
		report := r.(*gpsd.TPVReport)
		fmt.Println("Location updated:", report.Lat, report.Lon)
		newX, newZ := CoordinateGPSFloatToFloat(float32(report.Lat), float32(report.Lon))
		c.UpdatePos(newX, newZ)
		// fmt.Println("Location:", LAT, LON)
	}
	gps.AddFilter("TPV", tpvFilter)

	done := gps.Watch()
	<-done

	gps.Close()
}

func (c *Camera) TrySense() {
	cmd := exec.Command("python3", "./internal/sensors/hat.py")

	// Create a pipe for standard output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		return
	}
	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error starting command:", err)
		return
	}

	// Read the output from the pipe
	reader := bufio.NewReader(stdout)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading output:", err)
			break
		}

		// Parse the sense data
		var yaw, roll, pitch float64
		n, err := fmt.Sscanf(input, "[%f, %f, %f]", &yaw, &roll, &pitch)
		if err != nil || n != 3 {
			fmt.Println("Error parsing input:", err)
			continue
		}
		fmt.Println(roll, pitch, yaw)

		// Update the front vector with pitch and yaw data
		front := calcFrontVector(pitch, yaw)
		c.UpdateFront(front)
	}

	// Print the output
	// fmt.Println("output " + string(output))
	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		fmt.Println("Error waiting for command:", err)
		return
	}
}

// Processes input from the glfw window and moves the camera accordingly
// This function is mainly for debugging purposes
// Uses WASD for movement, and arrow keys for viewing
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
		front := calcFrontVector(pitch, yaw)
		c.UpdateFront(front)

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

// Update the camera's front, right, and up vectors based on the current rotation
func (c *Camera) updateCameraVectors(roll, pitch, yaw float64) {
	// Create quaternion for yaw, pitch, and roll
	yawQuat := QuatRotate(yaw, &lin.Vec3{0, 1, 0})     // Rotate around Y-axis
	pitchQuat := QuatRotate(pitch, &lin.Vec3{1, 0, 0}) // Rotate around X-axis
	rollQuat := QuatRotate(roll, &lin.Vec3{0, 0, 1})   // Rotate around Z-axis

	// Combine rotations
	c.Rotation.Mult(c.Rotation, &rollQuat)
	c.Rotation.Mult(c.Rotation, &pitchQuat)
	c.Rotation.Mult(c.Rotation, &yawQuat)
	c.Rotation.Norm(c.Rotation)

	// calculate new front, right, and up vectors

	front := lin.Vec3{0, 0, -1}            // Default front vector
	front = RotateQuat(&front, c.Rotation) // Rotate by the combined quaternion
	front.Norm(&front)                     // Normalize the front vector
	c.Front = &front

	right := lin.Vec3{1, 0, 0}
	right = RotateQuat(&right, c.Rotation)
	right.Norm(&right)
	c.Right = &right

	up := lin.Vec3{0, 1, 0}
	up = RotateQuat(&up, c.Rotation)
	up.Norm(&up)
	c.Up = &up

	fmt.Printf("Front: %v, Right: %v, Up: %v, Rot: %v\n", c.Front, c.Right, c.Up, c.Rotation)
}

// Helper Functions for Quaternion Rotations
// lin.Quat defined as [x,y,x,w]

// QuatRotate creates a quaternion representing a rotation around an axis
func QuatRotate(angle float64, axis *lin.Vec3) lin.Quat {
	halfAngle := angle / 2
	s := float32(math.Sin(float64(halfAngle)))
	return lin.Quat{
		axis[0] * s,
		axis[1] * s,
		axis[2] * s,
		float32(math.Cos(float64(halfAngle))),
	}
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

func (c *Camera) UpdatePos(newX, newZ float32) {
	c.posMutex.Lock()
	(*c.Pos)[0] = newX
	(*c.Pos)[2] = newZ
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
