package camera

import (
	"fmt"
	"math"

	"github.com/elokore/glfw/v3.4/glfw"
	lin "github.com/xlab/linmath"
)

// InputProcessor handles keyboard input for camera movement
type InputProcessor struct {
	camera *CameraImpl
}

// NewInputProcessor creates a new input processor
func NewInputProcessor(camera *CameraImpl) *InputProcessor {
	return &InputProcessor{
		camera: camera,
	}
}

// ProcessInput processes input from the GLFW window and moves the camera accordingly
// This function is mainly for debugging purposes and manual control
func (ip *InputProcessor) ProcessInput(window *glfw.Window) {
	if window.GetAttrib(glfw.Focused) != glfw.True {
		return
	}

	// Handle application exit
	if window.GetKey(glfw.KeyLeftControl) == glfw.Press &&
		window.GetKey(glfw.KeyC) == glfw.Press {
		window.SetShouldClose(true)
		return
	}

	// Process movement inputs
	ip.processMovementInput(window)

	// Process view inputs
	ip.processViewInput(window)

	// Process debug inputs
	ip.processDebugInput(window)
}

// processMovementInput handles WASD movement and vertical controls
func (ip *InputProcessor) processMovementInput(window *glfw.Window) {
	tmp := lin.Vec3{}
	front, _, up := ip.camera.getVectors()
	speed := ip.camera.speed

	// Forward/Backward movement
	if window.GetKey(glfw.KeyW) == glfw.Press { // Move Forward
		tmp.Scale(&front, speed)
		ip.camera.addPosition(tmp)
	}
	if window.GetKey(glfw.KeyS) == glfw.Press { // Move Backward
		tmp.Scale(&front, speed)
		ip.camera.subPosition(tmp)
	}

	// Left/Right strafe movement
	if window.GetKey(glfw.KeyA) == glfw.Press { // Move Left
		tmp.MultCross(&front, &up)
		tmp.Norm(&tmp)
		tmp.Scale(&tmp, speed)
		ip.camera.subPosition(tmp)
	}
	if window.GetKey(glfw.KeyD) == glfw.Press { // Move Right
		tmp.MultCross(&front, &up)
		tmp.Norm(&tmp)
		tmp.Scale(&tmp, speed)
		ip.camera.addPosition(tmp)
	}

	// Vertical movement
	if window.GetKey(glfw.KeySpace) == glfw.Press { // Move Up
		tmp.Scale(&up, speed)
		ip.camera.addPosition(tmp)
	}
	if window.GetKey(glfw.KeyLeftControl) == glfw.Press { // Move Down
		tmp.Scale(&up, speed)
		ip.camera.subPosition(tmp)
	}

	// Toggle map mode
	mapButtonPressed := window.GetKey(glfw.KeyM) == glfw.Press
	if mapButtonPressed && !ip.camera.mapButtonPressed {
		ip.camera.mapView = !ip.camera.mapView
	}
	ip.camera.mapButtonPressed = mapButtonPressed

	// Toggle map north mode
	mapNorthButtonPressed := window.GetKey(glfw.KeyN) == glfw.Press
	if mapNorthButtonPressed && !ip.camera.northButtonPressed && ip.camera.mapView {
		ip.camera.northView = !ip.camera.northView
	}
	ip.camera.northButtonPressed = mapNorthButtonPressed
}

// processViewInput handles camera rotation using arrow keys and Q/E for roll
func (ip *InputProcessor) processViewInput(window *glfw.Window) {
	// Get relative degree of turning from user input
	var pitch, roll, yaw float64
	sensitivity := float64(ip.camera.sensitivity)

	// Camera viewing using arrow keys
	if window.GetKey(glfw.KeyUp) == glfw.Press { // View Up
		pitch += sensitivity
	}
	if window.GetKey(glfw.KeyDown) == glfw.Press { // View Down
		pitch -= sensitivity
	}
	if window.GetKey(glfw.KeyLeft) == glfw.Press { // View Left
		yaw -= sensitivity
	}
	if window.GetKey(glfw.KeyRight) == glfw.Press { // View Right
		yaw += sensitivity
	}

	// Roll controls
	if window.GetKey(glfw.KeyE) == glfw.Press { // Roll Right
		roll -= sensitivity
		// Keep roll within reasonable bounds
		if roll < -3.14159 {
			roll += 2 * 3.14159
		}
	}
	if window.GetKey(glfw.KeyQ) == glfw.Press { // Roll Left
		roll += sensitivity
		// Keep roll within reasonable bounds
		if roll > 3.14159 {
			roll -= 2 * 3.14159
		}
	}

	// Apply rotation if any input was detected
	if pitch != 0 || roll != 0 || yaw != 0 {
		ip.camera.motionHandler.AdjustOrientation(yaw, pitch, roll)
	}
}

// processDebugInput handles debug and information display keys
func (ip *InputProcessor) processDebugInput(window *glfw.Window) {
	if window.GetKey(glfw.KeyR) == glfw.Press { // Print Debug Info
		fmt.Printf("Position: %s\n", ip.camera.GetPositionString())
		fmt.Printf("Vectors: %s\n", ip.camera.GetViewVectors())

		// Print orientation if available
		yaw, pitch, roll := ip.camera.getOrientation()
		if yaw != 0 || pitch != 0 || roll != 0 {
			fmt.Printf("Orientation: Yaw=%.3f°, Pitch=%.3f°, Roll=%.3f°\n",
				yaw*180/math.Pi, pitch*180/math.Pi, roll*180/math.Pi)
		}
	}
}

// createInputState creates an InputState from the current window state
// This could be useful for future input abstraction
func (ip *InputProcessor) createInputState(window *glfw.Window) InputState {
	keyMap := make(map[glfw.Key]bool)

	// Movement keys
	keyMap[glfw.KeyW] = window.GetKey(glfw.KeyW) == glfw.Press
	keyMap[glfw.KeyA] = window.GetKey(glfw.KeyA) == glfw.Press
	keyMap[glfw.KeyS] = window.GetKey(glfw.KeyS) == glfw.Press
	keyMap[glfw.KeyD] = window.GetKey(glfw.KeyD) == glfw.Press
	keyMap[glfw.KeySpace] = window.GetKey(glfw.KeySpace) == glfw.Press
	keyMap[glfw.KeyLeftControl] = window.GetKey(glfw.KeyLeftControl) == glfw.Press

	// View keys
	keyMap[glfw.KeyUp] = window.GetKey(glfw.KeyUp) == glfw.Press
	keyMap[glfw.KeyDown] = window.GetKey(glfw.KeyDown) == glfw.Press
	keyMap[glfw.KeyLeft] = window.GetKey(glfw.KeyLeft) == glfw.Press
	keyMap[glfw.KeyRight] = window.GetKey(glfw.KeyRight) == glfw.Press
	keyMap[glfw.KeyQ] = window.GetKey(glfw.KeyQ) == glfw.Press
	keyMap[glfw.KeyE] = window.GetKey(glfw.KeyE) == glfw.Press

	// Debug keys
	keyMap[glfw.KeyR] = window.GetKey(glfw.KeyR) == glfw.Press
	keyMap[glfw.KeyC] = window.GetKey(glfw.KeyC) == glfw.Press

	return InputState{
		KeysPressed:   keyMap,
		WindowFocused: window.GetAttrib(glfw.Focused) == glfw.True,
		MouseDelta:    lin.Vec2{0, 0}, // Not used currently
	}
}
