package camera

import (
	"math"

	"github.com/ungerik/go3d/float64/quaternion"
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

// t is the progress along a spline at a given moment
func (mh *MotionHandler) CentCatmullRom(t float32) *lin.Vec3 {
	splineData := mh.splineManager.currentSplineData
	p0 := splineData.p0
	p1 := splineData.p1
	p2 := splineData.p2
	p3 := splineData.p3

	// Knots are functions of position and progress along spline
	// given by t_i+1 = (P_i+1 - P_i)^0.5 + t_i
	t0 := float32(0.0)
	t1 := mh.GetT(p0, p1, t0)
	t2 := mh.GetT(p1, p2, t1)
	t3 := mh.GetT(p2, p3, t2)

	// Interpolation is done between P1 and P2
	// Map given progress into the space between P1 and P2
	// We use knots to represent these positions in spline math
	t = t1 + (t2-t1)*t

	// Compute A (A1, A2, A3), 1st level interpolation
	f1 := (t1 - t) / (t1 - t0)
	f2 := (t - t0) / (t1 - t0)
	a1 := LerpVec3(*p0, *p1, f1, f2)

	f1 = (t2 - t) / (t2 - t1)
	f2 = (t - t1) / (t2 - t1)
	a2 := LerpVec3(*p1, *p2, f1, f2)

	f1 = (t3 - t) / (t3 - t2)
	f2 = (t - t2) / (t3 - t2)
	a3 := LerpVec3(*p2, *p3, f1, f2)

	// Compute B1 and B2. 2nd level interpolation
	f1 = (t2 - t) / (t2 - t0)
	f2 = (t - t0) / (t2 - t0)
	b1 := LerpVec3(a1, a2, f1, f2)

	f1 = (t3 - t) / (t3 - t1)
	f2 = (t - t1) / (t3 - t1)
	b2 := LerpVec3(a2, a3, f1, f2)

	f1 = (t2 - t) / (t2 - t1)
	f2 = (t - t1) / (t2 - t1)
	c := LerpVec3(b1, b2, f1, f2)

	return &c
}

func (mh *MotionHandler) GetT(a, b *lin.Vec3, t float32) float32 {
	dx := b[0] - a[0]
	dy := b[1] - a[1]
	dz := b[2] - a[2]
	dist := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))
	return t + float32(math.Pow(dist, 0.5))
}

func LerpVec3(a, b lin.Vec3, f1, f2 float32) lin.Vec3 {
	return lin.Vec3{
		f1*a[0] + f2*b[0],
		f1*a[1] + f2*b[1],
		f1*a[2] + f2*b[2],
	}
}

func Lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// Spherically interpolate between two pre-normalized vectors
func Slerp(dir1, dir2 *lin.Vec3, t float32) *lin.Vec3 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	dot := Clamp(Dot(dir1, dir2), -1.0, 1.0)
	theta := float32(math.Acos(float64(dot)))
	sinTheta := float32(math.Sin(float64(theta)))

	w1 := float32(math.Sin(float64((1-t)*theta))) / sinTheta
	w2 := float32(math.Sin(float64(t*theta))) / sinTheta

	return &lin.Vec3{
		dir1[0]*w1 + dir2[0]*w2,
		dir1[1]*w1 + dir2[1]*w2,
		dir1[2]*w1 + dir2[2]*w2,
	}
}

func GetMagnitude3D(v *lin.Vec3) float32 {
	a := v[0]
	b := v[1]
	c := v[2]
	squareSum := float64(a*a + b*b + c*c)
	return float32(math.Sqrt(squareSum))
}

func GetNormalizedVector3D(v *lin.Vec3) *lin.Vec3 {
	magnitude := GetMagnitude3D(v)
	normX := v[0] / magnitude
	normY := v[1] / magnitude
	normZ := v[2] / magnitude
	return &lin.Vec3{
		normX,
		normY,
		normZ,
	}
}

// Get unsigned angle using dot and cross product of last 2 position vectors
func SignedAngle(p1, p2, axis *lin.Vec3) float32 {
	p1Normalized := GetNormalizedVector3D(p1)
	p2Normalized := GetNormalizedVector3D(p2)

	// We use the dot product of the last two position vectors to determine
	// the angle between them
	dotprod := float64(Clamp(Dot(p1Normalized, p2Normalized), -1.0, 1.0))
	angle := math.Acos(dotprod) * (180.0 / math.Pi) // return in degrees

	// Determine the sign of this angle using cross product
	var cross lin.Vec3
	cross.MultCross(p1Normalized, p2Normalized)
	sign := Dot(axis, &cross)

	if sign < 0 {
		angle = -angle
	}
	return float32(angle)
}

// I don't think our vector library contains operations for dot products
// Let me know if I'm wrong
func Dot(v1, v2 *lin.Vec3) float32 {
	return v1[0]*v2[0] + v1[1]*v2[1] + v1[2]*v2[2]
}

// Dot product returns float32, protect against floating point error
func Clamp(val, min, max float32) float32 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
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
