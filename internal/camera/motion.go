package camera

import (
	"github.com/ungerik/go3d/float64/quaternion"
	"github.com/ungerik/go3d/float64/vec3"
	lin "github.com/xlab/linmath"
)

// MotionHandler handles camera movement and rotation calculations
type MotionHandler struct {
	camera *Camera
}

// Quaternion-based rotation vectors for consistent calculations
var (
	qStartCameraUp    = vec3.T{0.0, 1.0, 0.0}
	qStartCameraFront = vec3.T{0, 0, -1.0}
	qStartCameraRight = vec3.Cross(&qStartCameraUp, &qStartCameraFront)
)

// NewMotionHandler creates a new motion handler
func NewMotionHandler(camera *Camera) *MotionHandler {
	return &MotionHandler{
		camera: camera,
	}
}

// ApplyAbsoluteRotation calculates rotations based on absolute orientation
// Used primarily for sensor input (GPS, IMU)
func (mh *MotionHandler) ApplyAbsoluteRotation(yaw, pitch, roll float64) {
	// Calculate the rotation quaternions for each component
	qYaw := quaternion.FromAxisAngle(&qStartCameraUp, -yaw)
	qRoll := quaternion.FromAxisAngle(&qStartCameraFront, -roll)
	qPitch := quaternion.FromAxisAngle(&qStartCameraRight, -pitch)

	// Calculate the combined rotation quaternion from Yaw, Roll, and Pitch
	rotQuat := quaternion.Mul3(&qYaw, &qPitch, &qRoll)

	// Rotate each vector based on the quaternion rotation
	qFront := rotQuat.RotatedVec3(&qStartCameraFront)
	qRight := rotQuat.RotatedVec3(&qStartCameraRight)
	qUp := rotQuat.RotatedVec3(&qStartCameraUp)

	// Normalize the vectors
	qFront.Normalize()
	qRight.Normalize()
	qUp.Normalize()

	// Convert to linmath vectors and update camera
	front := lin.Vec3{float32(qFront[0]), float32(qFront[1]), float32(qFront[2])}
	right := lin.Vec3{float32(qRight[0]), float32(qRight[1]), float32(qRight[2])}
	up := lin.Vec3{float32(qUp[0]), float32(qUp[1]), float32(qUp[2])}

	mh.camera.updateVectors(front, right, up)
	mh.camera.setOrientation(yaw, pitch, roll)
}

// ApplyRelativeRotation calculates rotations based on current orientation
// Used primarily for keyboard input and incremental adjustments
func (mh *MotionHandler) ApplyRelativeRotation(yaw, pitch, roll float64) {
	// Get current camera vectors
	front, right, up := mh.camera.getVectors()

	// Convert linmath vectors to quaternion-compatible vectors
	qUp := vec3.T{float64(up[0]), float64(up[1]), float64(up[2])}
	qFront := vec3.T{float64(front[0]), float64(front[1]), float64(front[2])}
	qRight := vec3.T{float64(right[0]), float64(right[1]), float64(right[2])}

	// Calculate the rotation quaternions for each component relative to current orientation
	qYawRot := quaternion.FromAxisAngle(&qStartCameraUp, -yaw) // Yaw around world up
	qRollRot := quaternion.FromAxisAngle(&qFront, -roll)       // Roll around current front
	qPitchRot := quaternion.FromAxisAngle(&qRight, pitch)      // Pitch around current right

	// Calculate the combined rotation quaternion
	rotQuat := quaternion.Mul3(&qYawRot, &qPitchRot, &qRollRot)

	// Apply rotation to current vectors
	rotQuat.RotateVec3(&qFront)
	rotQuat.RotateVec3(&qRight)
	rotQuat.RotateVec3(&qUp)

	// Normalize the vectors
	qFront.Normalize()
	qRight.Normalize()
	qUp.Normalize()

	// Convert back to linmath vectors and update camera
	newFront := lin.Vec3{float32(qFront[0]), float32(qFront[1]), float32(qFront[2])}
	newRight := lin.Vec3{float32(qRight[0]), float32(qRight[1]), float32(qRight[2])}
	newUp := lin.Vec3{float32(qUp[0]), float32(qUp[1]), float32(qUp[2])}

	mh.camera.updateVectors(newFront, newRight, newUp)
}

// SetOrientation sets absolute orientation angles and applies rotation
// Convenience method that combines setting orientation state and applying rotation
func (mh *MotionHandler) SetOrientation(yaw, pitch, roll float64) {
	mh.camera.setOrientation(yaw, pitch, roll)
	mh.ApplyAbsoluteRotation(yaw, pitch, roll)
}

// AdjustOrientation makes relative adjustments to current orientation
// Useful for smooth incremental changes
func (mh *MotionHandler) AdjustOrientation(deltaYaw, deltaPitch, deltaRoll float64) {
	currentYaw, currentPitch, currentRoll := mh.camera.getOrientation()

	newYaw := currentYaw + deltaYaw
	newPitch := currentPitch + deltaPitch
	newRoll := currentRoll + deltaRoll

	// Keep roll within reasonable bounds (-π to π)
	for newRoll > 3.14159 {
		newRoll -= 2 * 3.14159
	}
	for newRoll < -3.14159 {
		newRoll += 2 * 3.14159
	}

	mh.camera.setOrientation(newYaw, newPitch, newRoll)
	mh.ApplyRelativeRotation(deltaYaw, deltaPitch, deltaRoll)
}

// ResetOrientation resets camera to initial orientation
func (mh *MotionHandler) ResetOrientation() {
	mh.camera.setOrientation(0, 0, 0)

	// Reset to initial vectors
	front := lin.Vec3{0, 0, -1}
	right := lin.Vec3{1, 0, 0}
	up := START_CAMERA_UP

	mh.camera.updateVectors(front, right, up)
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
