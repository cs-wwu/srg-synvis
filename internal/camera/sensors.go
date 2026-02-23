package camera

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os/exec"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/coords"
	dvec3 "github.com/ungerik/go3d/float64/vec3"

	"github.com/stratoberry/go-gpsd"
	"github.com/westphae/geomag/pkg/egm96"
	"github.com/westphae/geomag/pkg/wmm"
)

const DAMP = 0.95              // Smoothing factor for sensor data
const CORRECTION_DISTANCE = 50 // max distance error before position correction

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
	position dvec3.T
	occurred time.Time
	velocity dvec3.T
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
			velocity: dvec3.T{0, 0, 0},
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
	lat32, lon32 := coords.WorldToGPSFloat(float32(pos[0]), float32(pos[2]))
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
	lat, lon := coords.WorldToGPSFloat(float32(pos[0]), float32(pos[2]))
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
		worldY := data.GPSPosition.Altitude * HEIGHT_CONVERSION

		sm.camera.updatePosition(float64(worldX), float64(worldZ), worldY)
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
	newY := float64(report.Alt) * HEIGHT_CONVERSION
	newX, newZ := coords.GPSToWorld(float32(report.Lat), float32(report.Lon))
	currPos := dvec3.T{float64(newX), newY, float64(newZ)}
	oldPos := lastReading.position

	// cache position, occurred time, and velocity
	deltaPos := dvec3.Sub(&currPos, &oldPos)

	// time
	currTime := time.Now()
	deltaTime := currTime.Sub(lastReading.occurred)
	deltaSeconds := deltaTime.Seconds()
	if deltaSeconds < 0.01 {
		deltaSeconds = 0.05
	}

	// vel
	currVel := deltaPos.Scaled(1.0 / deltaSeconds)

	// Construct the reading to be used for future reference
	reading := GPSReading{
		position: currPos,
		occurred: currTime,
		velocity: currVel,
	}

	// Construct new spline iff buffer is now full. otherwise just add new reading
	if len(sm.gpsRecordings) >= 3 {
		sm.gpsRecordings = append(sm.gpsRecordings[1:], reading)
		sm.camera.motionHandler.FeedGPSUpdate()
	} else {
		sm.gpsRecordings = append(sm.gpsRecordings, reading)
	}

	// Update interarrival time
	if len(sm.gpsRecordings) >= 2 {
		mma := &sm.camera.motionHandler.splineManager.averageGPSInterval
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

	// error between current camera position and current gps fix
	posErrorX := math.Abs(camPos[0] - g[2].position[0])
	posErrorY := math.Abs(camPos[1] - g[2].position[1])
	posErrorZ := math.Abs(camPos[2] - g[2].position[2])
	magnitude := math.Sqrt(posErrorX*posErrorX + posErrorY*posErrorY + posErrorZ*posErrorZ)
	// if error in predicted position vs. actual position exceeds allowance
	// then correct. harsh for now. TODO: implement smoothing w/ lerp?
	if magnitude >= CORRECTION_DISTANCE {
		mh.camera.SetPosition(g[2].position)
	}

	// Current velocity from GPS
	v2 := g[2].velocity
	spline.currentVelocity = v2

	// Compute p0 so tangent at p1 matches current velocity
	camVel := mh.camera.smoothedVel
	speed := camVel.Length()
	var p0 dvec3.T
	if speed > 0.1 {
		// Step backward from camera position along velocity direction
		backDist := speed * spline.averageGPSInterval
		velDir := camVel.Normalized()
		p0 = dvec3.T{
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
		p0: p0,     // synthetic control point matching velocity
		p1: camPos, // camera position (start)
		p2: pred1,  // predicted (end)
		p3: pred2,  // far prediction (exit tangent control)
	}
	spline.splineStartTime = time.Now()
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
