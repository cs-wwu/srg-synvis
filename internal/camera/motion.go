package camera

import (
	"github.com/ungerik/go3d/float64/vec3"
	lin "github.com/xlab/linmath"
)

// MotionHandler handles camera movement and rotation calculations
type MotionHandler struct {
	camera *CameraImpl
}

// Quaternion-based rotation vectors for consistent calculations
var (
	qStartCameraUp    = vec3.T{0.0, 1.0, 0.0}
	qStartCameraFront = vec3.T{0, 0, -1.0}
	qStartCameraRight = vec3.Cross(&qStartCameraUp, &qStartCameraFront)
)

// NewMotionHandler creates a new motion handler
func NewMotionHandler(camera *CameraImpl) *MotionHandler {
	return &MotionHandler{
		camera: camera,
	}
}

// SetOrientation sets absolute orientation angles and applies rotation
// Convenience method that combines setting orientation state and applying rotation
func (mh *MotionHandler) SetOrientation(yaw, pitch, roll float64) {
	mh.camera.SetOrientation(yaw, pitch, roll)
}

// AdjustOrientation makes relative adjustments to current orientation
// Useful for smooth incremental changes
func (mh *MotionHandler) AdjustOrientation(deltaYaw, deltaPitch, deltaRoll float64) {
	currentYaw, currentPitch, currentRoll := mh.camera.getOrientation()

	newYaw := currentYaw + deltaYaw
	newPitch := currentPitch + deltaPitch
	newRoll := currentRoll + deltaRoll

	mh.camera.SetOrientation(newYaw, newPitch, newRoll)
}

// ResetOrientation resets camera to initial orientation
func (mh *MotionHandler) ResetOrientation() {
	mh.camera.SetOrientation(0, 0, 0)
}

// GetMotionState returns current motion state for debugging
type MotionState struct {
	Position    lin.Vec3
	Orientation struct {
		Yaw   float64
		Pitch float64
		Roll  float64
	}
	Vectors struct {
		Front lin.Vec3
		Right lin.Vec3
		Up    lin.Vec3
	}
}

// GetMotionState returns the current motion state
func (mh *MotionHandler) GetMotionState() MotionState {
	pos := mh.camera.Position()
	yaw, pitch, roll := mh.camera.getOrientation()
	front, right, up := mh.camera.getVectors()

	return MotionState{
		Position: pos,
		Orientation: struct {
			Yaw   float64
			Pitch float64
			Roll  float64
		}{
			Yaw:   yaw,
			Pitch: pitch,
			Roll:  roll,
		},
		Vectors: struct {
			Front lin.Vec3
			Right lin.Vec3
			Up    lin.Vec3
		}{
			Front: front,
			Right: right,
			Up:    up,
		},
	}
}

// Future: Plane Mode Motion Physics
// These methods would be used for implementing realistic aircraft physics

// ApplyPhysicsUpdate would handle physics-based movement for plane mode
// func (mh *MotionHandler) ApplyPhysicsUpdate(deltaTime float32, airspeed, thrust, drag float32) {
//     // Physics calculations for realistic aircraft movement
//     // This would include:
//     // - Velocity integration
//     // - Acceleration from thrust
//     // - Drag calculations
//     // - Gravity effects
//     // - Banking physics for turns
// }

// SetAircraftConstraints would apply realistic flight constraints
// func (mh *MotionHandler) SetAircraftConstraints(maxBankAngle, maxPitchAngle, stallSpeed float32) {
//     // Implementation would enforce realistic aircraft limitations
// }
