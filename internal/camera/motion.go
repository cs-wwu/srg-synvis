package camera

import (
	"math"
	"time"

	"github.com/ungerik/go3d/float64/quaternion"
	"github.com/ungerik/go3d/float64/vec3"
	lin "github.com/xlab/linmath"
)

// MotionHandler handles camera movement and rotation calculations
type MotionHandler struct {
	camera        *Camera
	splineManager *SplineManager
}

// Keeps track of the state of the spline in real space
type SplineManager struct {
	currentSplineData  *SplineData
	splineStartTime    time.Time
	averageGPSInterval float64
	currentVelocity    lin.Vec3
}

// Data container for the actual spline (mathematical representation)
type SplineData struct {
	p0, p1, p2, p3 lin.Vec3
}

// Information for ensuring turning modulates position prediction
type TurnInfo struct {
	angularVel float64
	damping    float64
	curvedDir  lin.Vec3
	turnRadius float64
}

const (
	DEAD_RECKONING_TIMEOUT    = 3.0
	minTurnRadius             = 1.0
	sharpTurnThreshold        = 60.0
	dampingMin                = 0.25
	dampingMax                = 0.6
	curveTightnessFactor      = 0.8
	windingDetectionThreshold = 20.0
	minTimeDelta              = 0.0001
)

// Quaternion-based rotation vectors for consistent calculations
var (
	qStartCameraUp    = vec3.T{0.0, 1.0, 0.0}
	qStartCameraFront = vec3.T{0, 0, -1.0}
	qStartCameraRight = vec3.Cross(&qStartCameraUp, &qStartCameraFront)
)

// NewMotionHandler creates a new motion handler
func NewMotionHandler(camera *Camera) *MotionHandler {
	return &MotionHandler{
		camera:        camera,
		splineManager: &SplineManager{},
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

// we use basic kinematic equations to predict the position of the aircraft
// tempered with some prediction for turning angles
// without considering turns, prediction would only consider the forward vector
func (mh *MotionHandler) PredictPosition(futureSeconds float64) lin.Vec3 {
	currPos := mh.GetMotionState().Position
	if len(mh.camera.sensorManager.gpsRecordings) < 3 {
		return currPos
	}
	gps1 := mh.camera.sensorManager.gpsRecordings[1]
	gps2 := mh.camera.sensorManager.gpsRecordings[2]

	v1 := gps1.velocity
	v2 := gps2.velocity

	const epsZ = float64(1e-3)
	if math.Abs(float64(v1[2])) < epsZ && math.Abs(float64(v2[2])) < epsZ {
		v1[2] = 0
		v2[2] = 0
	}

	if v2.Len() < .01 {
		return gps2.position
	}

	var deltaVel lin.Vec3
	deltaVel.Sub(&v2, &v1)

	// deltaseconds is to calculate the acceleration between our two positions
	// note that this is not the same as predicted time!
	deltaTime := gps2.occurred.Sub(gps1.occurred)
	deltaSeconds := deltaTime.Seconds()
	if deltaSeconds < minTimeDelta {
		if mh.splineManager.averageGPSInterval > 0 {
			deltaSeconds = mh.splineManager.averageGPSInterval
		} else {
			deltaSeconds = 1.0
		}
	}

	var accel lin.Vec3
	accel.Scale(&deltaVel, float32(1.0/deltaSeconds))
	// cap absurd acceleration spikes
	const absMaxAccel = float64(80.0) // TODO config
	if math.Abs(float64(accel.Len())) > absMaxAccel {
		var n lin.Vec3
		n.Norm(&accel)
		accel = lin.Vec3{
			float32(float64(n[0]) * absMaxAccel),
			float32(float64(n[1]) * absMaxAccel),
			float32(float64(n[2]) * absMaxAccel),
		}
	}
	predTime := mh.splineManager.averageGPSInterval * futureSeconds

	turn := mh.CalculateTurn(v1, v2, accel)

	if math.Abs(turn.angularVel) > 1.0 {
		return mh.PredictCurved(gps2.position, v2, &turn, predTime)
	}

	// final pos = initial pos + 1/2 a*t^2
	var velTerm, accelTerm, offset, predicted lin.Vec3
	velTerm.Scale(&v2, float32(predTime))
	accelTerm.Scale(&accel, float32(0.5*predTime*predTime))
	offset.Add(&velTerm, &accelTerm)
	predicted.Add(&gps2.position, &offset)

	return predicted
}

// use provided info from turn calculation for position prediction
// has same issue with only focusing on XZ. Needs generalized.
func (mh *MotionHandler) PredictCurved(start lin.Vec3, vel lin.Vec3, turn *TurnInfo, t float64) lin.Vec3 {
	speed := float64(vel.Len())
	arc := speed * t
	radiansTraveled := arc / turn.turnRadius

	toRotate := radiansTraveled * math.Copysign(1, turn.angularVel)
	axis := vec3.T{float64(MAP_UP[0]), float64(MAP_UP[1]), float64(MAP_UP[2])}
	quat := quaternion.FromAxisAngle(&axis, toRotate)

	curvedDirVec3 := vec3.T{float64(turn.curvedDir[0]), float64(turn.curvedDir[1]), float64(turn.curvedDir[2])}
	tangent := quat.RotatedVec3(&curvedDirVec3)
	tangent.Normalize()

	return lin.Vec3{
		start[0] + float32(tangent[0]*arc),
		start[1] + float32(float64(vel[1])*t),
		start[2] + float32(tangent[2]*arc),
	}
}

// This compares the orientation of the two most recent readings to extrapolate
// information about the current curve/turn to project future position(s)
// TODO: A lot of these are magic numbers
func (mh *MotionHandler) CalculateTurn(v1, v2, accel lin.Vec3) TurnInfo {
	var info TurnInfo

	dir1 := lin.Vec3{v1[0], 0, v1[2]}
	dir1.Norm(&dir1)
	dir2 := lin.Vec3{v2[0], 0, v2[2]}
	dir2.Norm(&dir2)

	angle := SignedAngle(dir1, dir2, MAP_UP)
	info.angularVel = angle / mh.splineManager.averageGPSInterval

	turnRatio := math.Abs(info.angularVel) / sharpTurnThreshold

	sharpness := math.Min(1.0, math.Max(0.0, turnRatio))
	winding := math.Abs(info.angularVel) > windingDetectionThreshold

	if winding {
		info.damping = Lerp(dampingMin*1.5, dampingMax*0.7, sharpness)
	} else {
		info.damping = Lerp(dampingMin, dampingMax, sharpness)
	}

	rawCurvedDir := Slerp(dir1, dir2, info.damping)
	info.curvedDir = rawCurvedDir
	info.curvedDir.Norm(&info.curvedDir)

	// speed is simply the magnitude of a vector
	speed := float64(v2.Len())

	hSquares := float64(accel[0]*accel[0] + accel[2]*accel[2])
	hAccel := math.Sqrt(hSquares)
	// If no hAccel, turn radius can be represented by a really large number
	if hAccel > 0.001 {
		if (speed*speed)/hAccel > minTurnRadius {
			info.turnRadius = (speed * speed) / hAccel
		} else {
			info.turnRadius = minTurnRadius
		}
	} else {
		info.turnRadius = 100000.0
	}

	if sharpness > 0.2 {
		info.turnRadius *= curveTightnessFactor
	}
	return info
}

// Knots are functions of position
// Interpolation is done between P1 and P2
func (mh *MotionHandler) CentCatmullRom(t float64, data *SplineData) lin.Vec3 {
	p0 := data.p0
	p1 := data.p1
	p2 := data.p2
	p3 := data.p3

	t0 := float64(0.0)
	t1 := mh.GetT(p0, p1, t0)
	t2 := mh.GetT(p1, p2, t1)
	t3 := mh.GetT(p2, p3, t2)

	t = t1 + (t2-t1)*t

	// Compute A (A1, A2, A3), 1st level interpolation
	f1 := (t1 - t) / (t1 - t0)
	f2 := (t - t0) / (t1 - t0)
	a1 := LerpVec3(p0, p1, f1, f2)

	f1 = (t2 - t) / (t2 - t1)
	f2 = (t - t1) / (t2 - t1)
	a2 := LerpVec3(p1, p2, f1, f2)

	f1 = (t3 - t) / (t3 - t2)
	f2 = (t - t2) / (t3 - t2)
	a3 := LerpVec3(p2, p3, f1, f2)

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

	return c
}

func (mh *MotionHandler) GetT(a, b lin.Vec3, t float64) float64 {
	dx := b[0] - a[0]
	dy := b[1] - a[1]
	dz := b[2] - a[2]
	dist := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))
	return t + math.Pow(dist, 0.5)
}

func LerpVec3(a, b lin.Vec3, f1, f2 float64) lin.Vec3 {
	return lin.Vec3{
		float32(f1*float64(a[0]) + f2*float64(b[0])),
		float32(f1*float64(a[1]) + f2*float64(b[1])),
		float32(f1*float64(a[2]) + f2*float64(b[2])),
	}
}

func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// Spherically interpolate between two pre-normalized vectors
func Slerp(dir1, dir2 lin.Vec3, t float64) lin.Vec3 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	// a dot b = |a|*|b| cos (angle)
	dot := float64(dir1[0]*dir2[0] + dir1[1]*dir2[1] + dir1[2]*dir2[2])
	// account for floating point error
	dot = math.Max(-1, math.Min(1, dot))
	theta := math.Acos(dot)
	sinTheta := math.Sin(theta)

	w1 := math.Sin((1-t)*theta) / sinTheta
	w2 := math.Sin(t*theta) / sinTheta

	return lin.Vec3{
		float32(float64(dir1[0])*w1 + float64(dir2[0])*w2),
		float32(float64(dir1[1])*w1 + float64(dir2[1])*w2),
		float32(float64(dir1[2])*w1 + float64(dir2[2])*w2),
	}
}

func SignedAngle(p1, p2, axis lin.Vec3) float64 {
	var p1Normalized, p2Normalized lin.Vec3
	p1Normalized.Norm(&p1)
	p2Normalized.Norm(&p2)

	dot := float64(p1Normalized[0]*p2Normalized[0] + p1Normalized[1]*p2Normalized[1] + p1Normalized[2]*p2Normalized[2])
	dot = math.Max(-1, math.Min(1, dot))
	// acos only returns an angle between 0-180 degrees. Right hand rule.
	angle := math.Acos(dot) * (180.0 / math.Pi)

	// determine the sign of the angle using cross product relative to y-axis
	var cross lin.Vec3
	cross.MultCross(&p1Normalized, &p2Normalized)
	sign := float64(axis[0]*cross[0] + axis[1]*cross[1] + axis[2]*cross[2])
	if sign < 0 {
		angle = -angle
	}
	return angle
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
