package camera

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os/exec"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	lin "github.com/xlab/linmath"

	"github.com/stratoberry/go-gpsd"
	"github.com/westphae/geomag/pkg/egm96"
	"github.com/westphae/geomag/pkg/wmm"
)

const DAMP = 0.95 // Smoothing factor for sensor data

// SensorManager handles GPS and IMU sensor integration
type SensorManager struct {
	camera        *CameraImpl
	testingMode   bool         // When true, skips smoothing for deterministic test results
	gpsRecordings []GPSReading // rolling buffer of raw position readings
}

// Stores lat, long, and alt and the time the reading thereof occurred.
// Time will be useful for retroactive path construction
// for interpolation and other future additions
type GPSReading struct {
	position lin.Vec3
	occurred time.Time
	velocity lin.Vec3
}

// SetTestingMode enables or disables the testing mode
// When testing mode is enabled, smoothing will be disabled for deterministic test results
func (sm *SensorManager) SetTestingMode(enabled bool) {
	sm.testingMode = enabled
}

// NewSensorManager creates a new sensor manager
func NewSensorManager(camera *CameraImpl) *SensorManager {
	return &SensorManager{
		camera:      camera,
		testingMode: false,
	}
}

// StartGPS connects to the GPSD service and gets lat, lon, and alt with TPV report
// Runs as a separate goroutine
func (sm *SensorManager) StartGPS() error {
	// Connect to GPSD
	gps, err := gpsd.Dial("localhost:2947")
	if err != nil {
		log.Println("Failed to connect to GPSD:", err)
		return err
	}
	defer gps.Close()

	// Set up position tracking for recorded positions, not predicted ones.
	// For interpolation, only a small buffer will be needed for splines
	// TODO: for testing + iteration, we need an unfixed length to generate a path
	if len(sm.gpsRecordings) == 0 {
		initialPos := sm.camera.Position()
		sm.gpsRecordings = append(sm.gpsRecordings, GPSReading{
			position: initialPos,
			occurred: time.Now(),
			velocity: lin.Vec3{0, 0, 0},
		})
	}
	log.Println("Setup: Initial position recorded.")

	// Set up the TPV function that will run indefinitely
	tpvFilter := func(r any) {
		report := r.(*gpsd.TPVReport)

		// Check if data has arrived or is faulty.
		// When GPS is updated, a the spline by which we interpolate position will be edited
		// and new predictions for future positions will be generated.
		if report.Mode == gpsd.NoFix || report.Mode == gpsd.NoValueSeen {
			log.Println("GPS: No fix, using predicted position based on previous movement")
			return
		} else {
			sm.pushGPS(report)
		}
	}

	// Add the filter and start watching
	gps.AddFilter("TPV", tpvFilter)

	// Start watching event stream
	done := gps.Watch()
	<-done
	return nil
}

// StartSensors gets yaw, pitch and roll from the sense hat via a pipe
// and updates the camera orientation indefinitely
// Runs as a separate goroutine
func (sm *SensorManager) StartSensors(filename string) Mode {
	cmd := exec.Command("python3", filename)

	// Create a pipe for standard output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Error creating stdout pipe:", err)
		return StartPipe
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		log.Println("Error starting sensor command:", err)
		return StartCommand
	}

	first := true
	var oldYaw, oldRoll, oldPitch float64

	// Read the output from the pipe
	reader := bufio.NewReader(stdout)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading sensor data:", err)
			break
		}

		// Parse the sensor data
		var yaw, roll, pitch float64
		n, err := fmt.Sscanf(input, "[%f, %f, %f]", &yaw, &roll, &pitch)
		if err != nil || n != 3 {
			log.Println("Error parsing sensor input:", err)
			continue
		}

		// Invert pitch for correct orientation
		pitch = -pitch
		pitch = pitch - 3.14159/2

		// Apply magnetic declination correction
		if err := sm.applyMagneticDeclination(&yaw); err != nil {
			log.Printf("Warning: Could not apply magnetic declination: %v", err)
		}

		// Apply smoothing to reduce sensor noise
		if !first {
			yaw, roll, pitch = Smoother64(yaw, oldYaw, roll, oldRoll, pitch, oldPitch)
		}
		first = false

		// Update camera orientation
		sm.camera.setOrientation(yaw, pitch, roll)

		// Apply absolute rotation to camera
		sm.camera.motionHandler.ApplyAbsoluteRotation(yaw, pitch, roll)

		// Store values for next smoothing iteration
		oldYaw = yaw
		oldRoll = roll
		oldPitch = pitch
	}

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		log.Println("Error waiting for sensor command:", err)
		return Waiting
	}

	return Waiting
}

// applyMagneticDeclination applies magnetic declination correction to compass heading
func (sm *SensorManager) applyMagneticDeclination(yaw *float64) error {
	pos := sm.camera.Position()

	// Convert world coordinates to GPS for magnetic model
	// Note: This assumes coords package has the inverse function
	// lat := float64(-pos[2]) / 3600.0           // Convert arcseconds to degrees
	// lon := float64(pos[0]) / 3600.0            // Convert arcseconds to degrees
	lat32, lon32 := coords.WorldToGPSFloat(pos[0], pos[2])
	lat := float64(lat32)
	lon := float64(lon32)
	alt := float64(pos[1]) / HEIGHT_CONVERSION // Convert to meters

	// Create location for magnetic model
	loc := egm96.NewLocationGeodetic(lat, lon, alt)

	// Calculate magnetic field
	mag, err := wmm.CalculateWMMMagneticField(loc, sm.camera.decimalYear)
	if err != nil {
		return fmt.Errorf("failed to calculate magnetic field: %w", err)
	}

	// Apply declination correction (magnetic to true heading)
	*yaw = *yaw + mag.D()

	log.Printf("Applied magnetic declination: %.2f° at position (%.4f, %.4f)",
		mag.D(), lat, lon)

	return nil
}

// GetSensorData returns the current sensor data (for future use)
func (sm *SensorManager) GetSensorData() SensorData {
	pos := sm.camera.Position()
	yaw, pitch, roll := sm.camera.getOrientation()

	// Convert position back to GPS coordinates for the data structure
	lat, lon := coords.WorldToGPSFloat(pos[0], pos[2])
	alt := pos[1] / HEIGHT_CONVERSION

	return SensorData{
		GPSPosition: GPSPosition{
			Latitude:  float64(lat),
			Longitude: float64(lon),
			Altitude:  float64(alt),
			Mode:      1, // Assume valid for now
		},
		GPSValid: true,
		Yaw:      yaw,
		Pitch:    pitch,
		Roll:     roll,
		IMUValid: yaw != 0 || pitch != 0 || roll != 0, // Simple validity check
	}
}

// SetSensorData allows external setting of sensor data (for testing/simulation)
func (sm *SensorManager) SetSensorData(data SensorData) {
	if data.GPSValid {
		// Convert GPS to world coordinates
		worldX, worldZ := coords.GPSToWorld(float32(data.GPSPosition.Latitude),
			float32(data.GPSPosition.Longitude))
		worldY := float32(data.GPSPosition.Altitude) * HEIGHT_CONVERSION

		sm.camera.updatePosition(worldX, worldZ, worldY)
	}

	if data.IMUValid {
		sm.camera.setOrientation(data.Yaw, data.Pitch, data.Roll)
		sm.camera.motionHandler.ApplyAbsoluteRotation(data.Yaw, data.Pitch, data.Roll)
	}
}

// Push a reading based off TPV report (overwrites oldest when full)
// Ensure a valid update has been delivered before you call this!
func (sm *SensorManager) pushGPS(report *gpsd.TPVReport) {
	lastReading := sm.gpsRecordings[len(sm.gpsRecordings)-1] // TODO: catch this, could be out of bounds

	// Convert GPS coordinates to world coordinates
	newY := float32(report.Alt) * HEIGHT_CONVERSION
	newX, newZ := coords.GPSToWorld(float32(report.Lat), float32(report.Lon))
	currPos := lin.Vec3{newX, newY, newZ}
	oldPos := lastReading.position

	// cache position, occurred time, and velocity
	// TODO: Check sub order for bug?
	var deltaPos lin.Vec3
	deltaPos.Sub(&currPos, &oldPos) // arg1 - arg2, result stored in first arg

	// time
	currTime := time.Now()
	deltaTime := currTime.Sub(lastReading.occurred)
	deltaSeconds := float32(deltaTime.Seconds())
	if deltaSeconds < 0.01 {
		deltaSeconds = 0.05
	}

	// vel
	var currVel lin.Vec3
	currVel.Scale(&deltaPos, 1.0/deltaSeconds)

	// TEMPNOTE: position, time occurred, and velocity should be sufficient for data logging
	// Construct the reading to be used for future reference
	reading := GPSReading{
		position: currPos,
		occurred: currTime,
		velocity: currVel,
	}

	// Our spline only uses 3 points, so roll back if already there
	// Construct new spline iff buffer is now full. otherwise just add new reading
	if len(sm.gpsRecordings) >= 3 {
		sm.gpsRecordings = append(sm.gpsRecordings[1:], reading)
		sm.camera.motionHandler.FeedGPSUpdate() // TEMPNOTE: Ignore anything using FeedGPS as it calls the actual spline/interpolation logic
	} else {
		sm.gpsRecordings = append(sm.gpsRecordings, reading)
	}

	// Update interarrival time
	if len(sm.gpsRecordings) >= 2 {
		mma := &sm.camera.motionHandler.splineManager.averageGPSInterval // TEMPNOTE: Ignore splinemanager
		if *mma <= 0 {
			*mma = deltaSeconds
		} else {
			*mma = 0.8**mma + 0.2*deltaSeconds
		}
	}
	log.Println("New GPS update.")
}

func (mh *MotionHandler) FeedGPSUpdate() {
	if len(mh.camera.sensorManager.gpsRecordings) < 3 {
		return
	}

	spline := mh.splineManager
	g := mh.camera.sensorManager.gpsRecordings

	// Where camera actually is
	camPos := mh.camera.Position()
	camPosPtr := &lin.Vec3{camPos[0], camPos[1], camPos[2]}

	// Current velocity from GPS
	v2 := g[2].velocity
	spline.currentVelocity = &v2

	camVel := mh.camera.smoothedVel
	if GetMagnitude3D(&camVel) < 0.1 {
		camVel = v2 // fallback to GPS velocity if smoothed is too small
	}

	// Compute p0 so tangent at p1 matches current velocity
	speed := GetMagnitude3D(&camVel)
	var p0 lin.Vec3
	if speed > 0.1 {
		// Step backward from camera position along velocity direction
		backDist := speed * spline.averageGPSInterval
		velDir := GetNormalizedVector3D(&camVel)
		p0 = lin.Vec3{
			camPos[0] - velDir[0]*backDist,
			camPos[1] - velDir[1]*backDist,
			camPos[2] - velDir[2]*backDist,
		}
	} else {
		// Fallback if stationary
		p0 = g[1].position
	}

	// Predictions for p2, p3
	pred1 := mh.PredictPosition(1.0)
	pred2 := mh.PredictPosition(2.0)

	spline.currentSplineData = &SplineData{
		p0: &p0,       // synthetic control point matching velocity
		p1: camPosPtr, // camera position (start)
		p2: pred1,     // predicted (end)
		p3: pred2,     // far prediction (exit tangent control)
	}
	spline.splineStartTime = time.Now()
}

// TODO: This should be a test case, not inlined in sensors.
func (sm *SensorManager) StartMockGPS(path []lin.Vec3, updateInterval time.Duration) {
	if sm == nil || len(path) == 0 {
		return
	}
	log.Println("Starting mock GPS with determined path")

	// Initialize if empty
	if len(sm.gpsRecordings) == 0 {
		sm.gpsRecordings = append(sm.gpsRecordings, GPSReading{
			position: path[0],
			occurred: time.Now(),
			velocity: lin.Vec3{0, 0, 0},
		})
	}

	// baselines for Δ comparisons
	var lastGPS *lin.Vec3
	var lastPlane *lin.Vec3

	for i := 1; i < len(path); i++ {
		time.Sleep(updateInterval)

		// Calculate velocity
		var velocity lin.Vec3
		deltaPos := lin.Vec3{}
		deltaPos.Sub(&path[i], &path[i-1])
		velocity.Scale(&deltaPos, 1.0/float32(updateInterval.Seconds()))

		// Create mock reading
		reading := GPSReading{
			position: path[i],
			occurred: time.Now(),
			velocity: velocity,
		}

		// ΔGPS / ΔPlane / instantaneous error logging (BEFORE appending)
		if lastGPS == nil {
			p := path[i-1]
			lastGPS = &lin.Vec3{p[0], p[1], p[2]}
		}
		if lastPlane == nil && sm.camera != nil {
			p := sm.camera.Position()
			lastPlane = &lin.Vec3{p[0], p[1], p[2]}
		}

		// ΔGPS since previous GPS tick
		dGPS := lin.Vec3{}
		dGPS.Sub(&reading.position, lastGPS)
		gpsDist := float32(math.Sqrt(float64(dGPS[0]*dGPS[0] + dGPS[1]*dGPS[1] + dGPS[2]*dGPS[2])))

		// ΔPlane (camera) since previous GPS tick
		planeNow := lin.Vec3{}
		if sm.camera != nil {
			p := sm.camera.Position()
			planeNow = lin.Vec3{p[0], p[1], p[2]}
		}
		dPlane := lin.Vec3{}
		if lastPlane != nil {
			dPlane.Sub(&planeNow, lastPlane)
		}
		planeDist := float32(math.Sqrt(float64(dPlane[0]*dPlane[0] + dPlane[1]*dPlane[1] + dPlane[2]*dPlane[2])))

		// Instantaneous error between plane and current GPS
		errVec := lin.Vec3{}
		errVec.Sub(&planeNow, &reading.position)
		errDist := float32(math.Sqrt(float64(errVec[0]*errVec[0] + errVec[1]*errVec[1] + errVec[2]*errVec[2])))

		// Speeds (units/sec) over the GPS interval
		dt := float32(updateInterval.Seconds())
		gpsSpeed := gpsDist / dt
		planeSpeed := planeDist / dt

		log.Printf(
			"\n#%d: ΔGPS=(%.2f,%.2f,%.2f)|%.2f\nΔPlane=(%.2f,%.2f,%.2f)|%.2f\nErr=(%.2f,%.2f,%.2f)|%.2f\nvGPS=%.2f u/s\nvPlane=%.2f u/s",
			i, dGPS[0], dGPS[1], dGPS[2], gpsDist,
			dPlane[0], dPlane[1], dPlane[2], planeDist,
			errVec[0], errVec[1], errVec[2], errDist,
			gpsSpeed, planeSpeed,
		)

		// advance baselines
		*lastGPS = reading.position
		*lastPlane = planeNow
		// --- end logging block ---

		// Push like real GPS (rolling buffer of 3)
		sm.gpsRecordings = append(sm.gpsRecordings, reading)
		if len(sm.gpsRecordings) > 3 {
			sm.gpsRecordings = sm.gpsRecordings[1:]
		}

		// Trigger spline update when we have 3+ readings
		if len(sm.gpsRecordings) >= 3 && sm.camera != nil && sm.camera.motionHandler != nil {
			sm.camera.motionHandler.FeedGPSUpdate()
		}

		log.Printf("Mock GPS [%d/%d]: pos=(%.1f, %.1f, %.1f), vel=(%.1f, %.1f, %.1f)\n\n",
			i, len(path)-1,
			path[i][0], path[i][1], path[i][2],
			velocity[0], velocity[1], velocity[2])
	}
	log.Println("Mock GPS path completed")
}

// Returns a flight path that emulates feeding GPS updates live
// This has been very useful for testing position interpolation without a pi
func GenerateScriptedPath(start lin.Vec3, speedMPH float32, dt time.Duration) []lin.Vec3 {
	path := []lin.Vec3{}
	cur := start
	step := func(dx, dy, dz float32) {
		cur = lin.Vec3{cur[0] + dx, cur[1] + dy, cur[2] + dz}
		path = append(path, cur)
	}

	// Parameters
	const fastHoriz = 60.0
	const normHoriz = 35.0
	const climbSlope = 0.2679491924311227 // tan(15°)

	// 1) Really fast north for 10 seconds (−Z)
	for i := 0; i < 10; i++ {
		step(0, 0, -fastHoriz)
	}

	// 2) Bank east slowly: north (0,−1) → east (+1,0)
	for i := 0; i < 10; i++ {
		t := float32(i+1) / 10.0
		dirX := +t
		dirZ := -(1.0 - t)
		len := float32(math.Sqrt(float64(dirX*dirX + dirZ*dirZ)))
		dirX /= len
		dirZ /= len
		step(dirX*normHoriz, 0, dirZ*normHoriz)
	}

	// 3) Fully east for 10 seconds (+X)
	for i := 0; i < 10; i++ {
		step(+normHoriz, 0, 0)
	}

	// 4) 15° climb east for 5 seconds
	for i := 0; i < 5; i++ {
		dx := float32(+normHoriz)
		dy := dx * float32(climbSlope)
		step(dx, dy, 0)
	}

	// 5) Maintain 15° climb, curve CCW east→south (+X → +Z)
	for i := 0; i < 10; i++ {
		t := float32(i+1) / 10.0
		dirX := +1.0 - t
		dirZ := +t
		len := float32(math.Sqrt(float64(dirX*dirX + dirZ*dirZ)))
		dirX /= len
		dirZ /= len
		h := float32(normHoriz)
		dx := dirX * h
		dz := dirZ * h
		dy := h * float32(climbSlope)
		step(dx, dy, dz)
	}

	// 6) Gentle undulation heading south (+Z)
	for i := 0; i < 10; i++ {
		phase := float64(i) * (2 * math.Pi / 10.0)
		dy := float32(math.Sin(phase)) * 5.0
		step(0, dy, normHoriz)
	}

	// 7) Stronger undulation, still south
	for i := 0; i < 5; i++ {
		phase := float64(i) * (2 * math.Pi / 5.0)
		dy := float32(math.Sin(phase)) * 10.0
		step(0, dy, normHoriz)
	}

	// 8) Gradual turn south → west (+Z → −X)
	for i := 0; i < 5; i++ {
		t := float32(i+1) / 5.0
		dirX := -t
		dirZ := +1.0 - t
		len := float32(math.Sqrt(float64(dirX*dirX + dirZ*dirZ)))
		dirX /= len
		dirZ /= len
		step(dirX*normHoriz, 0, dirZ*normHoriz)
	}

	// 9) Fully west 5 seconds (−X)
	for i := 0; i < 5; i++ {
		step(-normHoriz, 0, 0)
	}

	return path
}

func Smoother32(new1, old1, new2, old2, new3, old3 float32) (float32, float32, float32) {
	new1 = new1*DAMP + (1-DAMP)*old1
	new2 = new2*DAMP + (1-DAMP)*old2
	new3 = new3*DAMP + (1-DAMP)*old3
	return new1, new2, new3
}

func Smoother64(new1, old1, new2, old2, new3, old3 float64) (float64, float64, float64) {
	new1 = new1*DAMP + (1-DAMP)*old1
	new2 = new2*DAMP + (1-DAMP)*old2
	new3 = new3*DAMP + (1-DAMP)*old3
	return new1, new2, new3
}
