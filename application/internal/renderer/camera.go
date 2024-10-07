package renderer

// This file should be used to manipulate the camera position and vectors

import (
	"fmt"
	"log"
	"math"

	"github.com/go-gl/glfw/v3.3/glfw"
	lin "github.com/xlab/linmath"
)

var (
	// For heightmap model
	startX, startZ = CoordinateGPSToFloat("48.0, -123.0")
	HeightMeters = float32(13000.0)
	HeightArcseconds = HeightMeters * float32(60.0 / 1852.0)

	START_CAMERA_POS   = lin.Vec3{startX, float32(HeightArcseconds), startZ}
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

	Speed       float32 // Camera speed
	Sensitivity float32 // Camera sensitivity
}

// NewCamera creates and initializes a new Camera
func NewCamera() *Camera {
	pos := START_CAMERA_POS
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

// Processes input from the glfw window and moves the camera accordingly
// This function is mainly for debugging purposes
// Uses WASD for movement, and arrow keys for viewing
func (c *Camera) ProcessInput(window *glfw.Window) {
	if window.GetAttrib(glfw.Focused) == glfw.True {
		tmp := lin.Vec3{}

		// Camera Movement
		if window.GetKey(glfw.KeyW) == glfw.Press { // Move Forward
			tmp.Scale(c.Front, c.Speed)
			c.Pos.Add(c.Pos, &tmp)
		}
		if window.GetKey(glfw.KeyS) == glfw.Press { // Move Backward
			tmp.Scale(c.Front, c.Speed)
			c.Pos.Sub(c.Pos, &tmp)
		}
		if window.GetKey(glfw.KeyA) == glfw.Press { // Move Left
			tmp.MultCross(c.Front, c.Up)
			tmp.Norm(&tmp)
			tmp.Scale(&tmp, c.Speed)
			c.Pos.Sub(c.Pos, &tmp)
		}
		if window.GetKey(glfw.KeyD) == glfw.Press { // Move Right
			tmp.MultCross(c.Front, c.Up)
			tmp.Norm(&tmp)
			tmp.Scale(&tmp, c.Speed)
			c.Pos.Add(c.Pos, &tmp)
		}
		if window.GetKey(glfw.KeySpace) == glfw.Press { // Move Up
			tmp.Scale(c.Up, c.Speed)
			c.Pos.Add(c.Pos, &tmp)
		}
		if window.GetKey(glfw.KeyLeftControl) == glfw.Press { // Move Down
			tmp.Scale(c.Up, c.Speed)
			c.Pos.Sub(c.Pos, &tmp)
		}

		yaw := c.getYaw()
		pitch := c.getPitch()

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

		front := calcFrontVector(pitch, yaw)
		c.Front = &front

		if window.GetKey(glfw.KeyE) == glfw.Press { // Print Debug
			fmt.Printf("Pos: %v\n", c.GetPositionString())
			fmt.Printf("Vectors: %v\n", c.GetViewVectors())
		}
		// TODO: Update the camera's front, right, and up vectors for quaternions
		// remove the current method of clamping top and bottom view vectors,
		
		// c.updateCameraVectors()
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
	front[0] = float32(math.Cos((yaw)) * math.Cos(pitch))
	front[1] = float32(math.Sin((pitch)))
	front[2] = float32(math.Sin(yaw) * math.Cos(pitch))
	front.Norm(&front)
	return front
}

func (c *Camera) getYaw() float64 {
	return math.Atan2(float64(c.Front[2]), float64(c.Front[0]))
}

func (c *Camera) getPitch() float64 {
	return math.Asin(float64(c.Front[1]))
}

// Update the camera's front, right, and up vectors based on the current rotation
func (c *Camera) updateCameraVectors() {
	front := lin.Vec3{0, 0, -1}
	front = RotateQuat(&front, c.Rotation)
	front.Norm(&front)
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

// QuatRotate creates a quaternion representing a rotation around an axis
func QuatRotate(angle float32, axis *lin.Vec3) lin.Quat {
	halfAngle := angle / 2
	s := float32(math.Sin(float64(halfAngle)))
	return lin.Quat{
		float32(math.Cos(float64(halfAngle))),
		axis[0] * s,
		axis[1] * s,
		axis[2] * s,
	}
}

// RotateQuat rotates a vector by a quaternion
func RotateQuat(vec *lin.Vec3, quat *lin.Quat) lin.Vec3 {
	qv := lin.Quat{0, vec[0], vec[1], vec[2]}
	qConj := lin.Quat{quat[0], -quat[1], -quat[2], -quat[3]}

	tmp := lin.Quat{}
	tmp.Mult(quat, &qv)
	tmp.Mult(&tmp, &qConj)

	return lin.Vec3{tmp[1], tmp[2], tmp[3]}
}
