package camera

import (
	"math"
	"time"

	"github.com/ungerik/go3d/float64/quaternion"
	dvec3 "github.com/ungerik/go3d/float64/vec3"
)

const DEAD_RECKONING_TIMEOUT = 5

// MotionHandler handles camera movement and rotation calculations
type MotionHandler struct {
	camera        *CameraImpl
	splineManager *SplineManager
}

// Keeps track of the state of the spline in real space
type SplineManager struct {
	stalling           bool
	averageGPSInterval float64
	timeSinceLastGPS   float64
	currentVelocity    dvec3.T
	predictions        []dvec3.T
	currentSplineData  *SplineData
	splineStartTime    time.Time
	prevSplineData     *SplineData
}

// Data container for the actual spline (mathematical representation)
type SplineData struct {
	p0              dvec3.T
	p1              dvec3.T
	p2              dvec3.T
	p3              dvec3.T
	splineStartTime float64
}

// Information for ensuring turning modulates position prediction
type TurnInfo struct {
	turnRadius float64
	angularVel float64 // degrees/second
	curvedDir  dvec3.T // curving in direction of, normalized
	damping    float64
}

// Quaternion-based rotation vectors for consistent calculations
var (
	qStartCameraUp    = dvec3.T{0.0, 1.0, 0.0}
	qStartCameraFront = dvec3.T{0, 0, -1.0}
	qStartCameraRight = dvec3.Cross(&qStartCameraUp, &qStartCameraFront)
)

const (
	minTurnRadius             = float64(1.0)
	sharpTurnThreshold        = float64(60.0) // degrees/second
	dampingMin                = float64(0.25)
	dampingMax                = float64(0.6)
	curveTightnessFactor      = float64(0.8)
	windingDetectionThreshold = float64(20.0) // degrees/second
	minTimeDelta              = float64(0.0001)
)

// NewMotionHandler creates a new motion handler
func NewMotionHandler(camera *CameraImpl) *MotionHandler {
	return &MotionHandler{
		camera: camera,
		splineManager: &SplineManager{
			averageGPSInterval: 1.0,
			currentVelocity:    dvec3.T{},
			predictions:        make([]dvec3.T, 4),
		},
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

	// Convert to dvec3s and update camera
	// I think this is redundant? Just refactoring for rn.
	front := dvec3.T{float64(qFront[0]), float64(qFront[1]), float64(qFront[2])}
	right := dvec3.T{float64(qRight[0]), float64(qRight[1]), float64(qRight[2])}
	up := dvec3.T{float64(qUp[0]), float64(qUp[1]), float64(qUp[2])}

	mh.camera.updateVectors(front, right, up)
	mh.camera.setOrientation(yaw, pitch, roll)
}

// ApplyRelativeRotation calculates rotations based on current orientation
// Used primarily for keyboard input and incremental adjustments
func (mh *MotionHandler) ApplyRelativeRotation(yaw, pitch, roll float64) {
	// Get current camera vectors
	front, right, up := mh.camera.getVectors()

	// Convert linmath vectors to quaternion-compatible vectors
	qUp := dvec3.T{float64(up[0]), float64(up[1]), float64(up[2])}
	qFront := dvec3.T{float64(front[0]), float64(front[1]), float64(front[2])}
	qRight := dvec3.T{float64(right[0]), float64(right[1]), float64(right[2])}

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
	newFront := dvec3.T{float64(qFront[0]), float64(qFront[1]), float64(qFront[2])}
	newRight := dvec3.T{float64(qRight[0]), float64(qRight[1]), float64(qRight[2])}
	newUp := dvec3.T{float64(qUp[0]), float64(qUp[1]), float64(qUp[2])}

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
	front := dvec3.T{0, 0, -1}
	right := dvec3.T{1, 0, 0}
	upLV3 := START_CAMERA_UP
	up := dvec3.T{float64(upLV3[0]),
		float64(upLV3[1]), float64(upLV3[2])}

	mh.camera.updateVectors(front, right, up)
}

// GetMotionState returns current motion state for debugging
type MotionState struct {
	Position    dvec3.T
	Orientation struct {
		Yaw   float64
		Pitch float64
		Roll  float64
	}
	Vectors struct {
		Front dvec3.T
		Right dvec3.T
		Up    dvec3.T
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
			Front dvec3.T
			Right dvec3.T
			Up    dvec3.T
		}{
			Front: front,
			Right: right,
			Up:    up,
		},
	}
}

func (mh *MotionHandler) PredictPosition(futureSeconds float64) dvec3.T {
	currPos := mh.GetMotionState().Position
	if len(mh.camera.sensorManager.gpsRecordings) < 3 {
		return currPos
	}
	gps1 := mh.camera.sensorManager.gpsRecordings[1]
	gps2 := mh.camera.sensorManager.gpsRecordings[2]

	// we use basic kinematic equations to predict the position of the aircraft
	// tempered with some prediction for turning angles
	// without considering turns, prediction would only consider the forward vector
	v1 := gps1.velocity
	v2 := gps2.velocity

	const epsZ = float64(1e-3)
	if float64(math.Abs(float64(v1[2]))) < epsZ && float64(math.Abs(float64(v2[2]))) < epsZ {
		v1[2] = 0
		v2[2] = 0
	}

	if v2.Length() < .01 {
		return gps2.position
	}

	deltaVel := dvec3.Sub(&v2, &v1)

	// deltaseconds is to calculate the acceleration between our two positions
	// note that this is not the same as predicted time!
	deltaTime := gps2.occurred.Sub(gps1.occurred)
	deltaSeconds := deltaTime.Seconds()
	if deltaSeconds < minTimeDelta { // minTimeDelta is defined above in this file
		if mh.splineManager.averageGPSInterval > 0 {
			deltaSeconds = mh.splineManager.averageGPSInterval
		} else {
			deltaSeconds = 1.0
		}
	}

	// final pos = initial pos + 1/2 a*t^2
	// on average, GPS readings come in every second
	// To smooth for noise or loss, we keep a rolling average and
	// use that as a factor in predicting future positions
	accel := deltaVel.Scaled(1.0 / deltaSeconds)
	// cap absurd acceleration spikes
	const absMaxAccel = float64(80.0) // TODO config
	if math.Abs(accel.Length()) > absMaxAccel {
		n := accel.Normalized()
		accel = dvec3.T{n[0] * absMaxAccel, n[1] * absMaxAccel, n[2] * absMaxAccel}
	}
	predTime := mh.splineManager.averageGPSInterval * futureSeconds

	// velocity difference (past+present) is sufficient to determine
	// whether or not we are turning, and by how much
	turn := mh.CalculateTurn(v1, v2, accel)

	// If we are turning, predict position differently than if
	// we are moving in a straight line
	if math.Abs(float64(turn.angularVel)) > 1.0 {
		return mh.PredictCurved(gps2.position, v2, &turn, predTime)
	}
	// x_f = x_0 + vt + at^2. or, to find the predicted position,
	// sum the original position, the velocity term, and the acceleration term
	velTerm := v2.Scaled(predTime)
	accelTerm := accel.Scaled(0.5 * predTime * predTime)
	offset := dvec3.Add(&velTerm, &accelTerm)
	predicted := dvec3.Add(&gps2.position, &offset)

	return predicted
}

// use provided info from turn calculation for position prediction
// has same issue with only focusing on XZ. Needs generalized.
func (mh *MotionHandler) PredictCurved(start dvec3.T, vel dvec3.T, turn *TurnInfo, t float64) dvec3.T {
	// distance/rate parameters
	speed := vel.Length()
	arc := speed * t
	radiansTraveled := arc / turn.turnRadius

	toRotate := radiansTraveled * math.Copysign(1, float64(turn.angularVel))
	axis := MAP_UP
	quat := quaternion.FromAxisAngle(&axis, toRotate)

	tangent := quat.RotatedVec3(&turn.curvedDir)
	normalizedTan := tangent.Normalized()
	hOffset := normalizedTan.Scaled(arc)
	vOffset := vel[1] * t

	return dvec3.T{
		start[0] + hOffset[0],
		start[1] + vOffset,
		start[2] + hOffset[2],
	}
}

// This compares the orientation of the two most recent readings to extrapolate
// information about the current curve/turn to project future position(s)
// TODO: the XZ logic is incomplete. curves of course happen in the air
// still works for now.
func (mh *MotionHandler) CalculateTurn(v1, v2, accel dvec3.T) TurnInfo {
	var info TurnInfo

	dir1 := dvec3.T{v1[0], 0, v1[2]}
	dir1 = dir1.Normalized()
	dir2 := dvec3.T{v2[0], 0, v2[2]}
	dir2 = dir2.Normalized()

	angle := SignedAngle(dir1, dir2, MAP_UP)
	info.angularVel = angle / mh.splineManager.averageGPSInterval

	turnRatio := math.Abs(info.angularVel) / sharpTurnThreshold

	sharpness := float64(math.Min(1.0, math.Max(0.0, turnRatio)))
	winding := math.Abs(float64(info.angularVel)) > float64(windingDetectionThreshold)

	// TODO: A lot of these are magic numbers from when I was prototyping before I serialized
	// fields. We can make these modular for different aircraft later on
	if winding {
		info.damping = Lerp(dampingMin*1.5, dampingMax*0.7, sharpness)
	} else {
		info.damping = Lerp(dampingMin, dampingMax, sharpness)
	}

	// Determine the direction to move in as a function of
	// whether or not we are in a winding path or entering a
	// sharp turn (determined currently by magic nums)
	rawCurvedDir := Slerp(dir1, dir2, info.damping)
	info.curvedDir = rawCurvedDir.Normalized()

	// speed is simply the magnitude of a vector
	speed := v2.Length()

	// Finally, we keep information about the turning radius between these two positions
	// We use information about acceleration along X/Z and the directions of movement logged
	// when those positions occurred
	hSquares := float64(accel[0]*accel[0] + accel[2]*accel[2])
	hAccel := float64(math.Sqrt(hSquares))
	if hAccel > 0.001 {
		if (speed*speed)/hAccel > minTurnRadius {
			info.turnRadius = (speed * speed) / hAccel
		} else {
			info.turnRadius = minTurnRadius
		}
	} else {
		// If no hAccel, turn radius can be represented by a really large number
		info.turnRadius = 100000.0
	}

	if sharpness > 0.2 {
		info.turnRadius *= curveTightnessFactor
	}
	return info
}

// Spline functions

// t is the progress along a spline at a given moment
func (mh *MotionHandler) CentCatmullRom(t float64, data *SplineData) dvec3.T {
	p0 := data.p0
	p1 := data.p1
	p2 := data.p2
	p3 := data.p3

	// Knots are functions of position and progress along spline
	// given by t_i+1 = (P_i+1 - P_i)^0.5 + t_i
	t0 := float64(0.0)
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

func (mh *MotionHandler) GetT(a, b dvec3.T, t float64) float64 {
	dx := b[0] - a[0]
	dy := b[1] - a[1]
	dz := b[2] - a[2]
	dist := math.Sqrt(float64(dx*dx + dy*dy + dz*dz))
	return t + float64(math.Pow(dist, 0.5))
}

func LerpVec3(a, b dvec3.T, f1, f2 float64) dvec3.T {
	return dvec3.T{
		f1*a[0] + f2*b[0],
		f1*a[1] + f2*b[1],
		f1*a[2] + f2*b[2],
	}
}

// Vector, angle, and general helpers

func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// Spherically interpolate between two pre-normalized vectors
func Slerp(dir1, dir2 dvec3.T, t float64) dvec3.T {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	dot := dvec3.Dot(&dir1, &dir2)
	// account for floating point error i.e. 1.0001
	// domain of acos [-1, 1]
	dot = math.Max(-1, math.Min(1, dot))
	theta := float64(math.Acos(dot))
	sinTheta := float64(math.Sin(theta))

	w1 := float64(math.Sin(float64((1-t)*theta))) / sinTheta
	w2 := float64(math.Sin(float64(t*theta))) / sinTheta

	return dvec3.T{
		dir1[0]*w1 + dir2[0]*w2,
		dir1[1]*w1 + dir2[1]*w2,
		dir1[2]*w1 + dir2[2]*w2,
	}
}

// Get angle between two direction vectors, p1, p2 measured about specified axis
// i.e. p1 = north & p2 = northwest about y axis returns -45 degrees, etc.
// TODO: needs extended to axes other than y. This was a hack for functionality present in Unity.
// which doesn't seem to have given issues so far
func SignedAngle(p1, p2, axis dvec3.T) float64 {
	p1Normalized := p1.Normalized()
	p2Normalized := p2.Normalized()

	// a dot b = |a|*|b| cos (angle). So we find the dot product of
	// those directions and acos returns the angle.
	dot := dvec3.Dot(&p1Normalized, &p2Normalized)
	// account for floating point error i.e. 1.0001
	// domain of acos [-1, 1]
	dot = math.Max(-1, math.Min(1, dot))
	angle := math.Acos(dot) * (180.0 / math.Pi) // return in degrees

	// acos only returns an angle between 0-180 degrees. Right hand rule.
	// determine the sign of the angle using cross product relative to y-axis
	cross := dvec3.Cross(&p1Normalized, &p2Normalized)
	sign := dvec3.Dot(&axis, &cross)
	if sign < 0 {
		angle = -angle
	}
	return angle
}

// Future: Plane Mode Motion Physics
// These methods would be used for implementing realistic aircraft physics

// ApplyPhysicsUpdate would handle physics-based movement for plane mode
// func (mh *MotionHandler) ApplyPhysicsUpdate(deltaTime float64, airspeed, thrust, drag float64) {
// // Physics calculations for realistic aircraft movement
// // This would include:
// // - Velocity integration
// // - Acceleration from thrust
// // - Drag calculations
// // - Gravity effects
// // - Banking physics for turns
// }

// SetAircraftConstraints would apply realistic flight constraints
// func (mh *MotionHandler) SetAircraftConstraints(maxBankAngle, maxPitchAngle, stallSpeed float64) {
// // Implementation would enforce realistic aircraft limitations
// }
