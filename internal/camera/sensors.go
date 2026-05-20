package camera

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"os/exec"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/coords"

	"github.com/stratoberry/go-gpsd"
	"github.com/westphae/geomag/pkg/egm96"
	"github.com/westphae/geomag/pkg/wmm"
	lin "github.com/xlab/linmath"
)

const DAMP = 0.9               // Smoothing factor for sensor data
const CORRECTION_DISTANCE = 50 // max distance error before position correction

// Stores lat, long, and alt and the time the reading thereof occurred.
// Time will be useful for retroactive path construction
// for interpolation and other future additions
type GPSReading struct {
	position lin.Vec3
	occurred time.Time
	velocity lin.Vec3
}

// SensorManager handles GPS and IMU sensor integration
type SensorManager struct {
	camera           *Camera
	smoothingEnabled bool         // When true, smooths path through space
	gpsRecordings    []GPSReading // rolling buffer of raw position readings
	ctxGPS           context.Context
	cancelGPS        context.CancelFunc
	ctxHat           context.Context
	cancelHat        context.CancelFunc
}

// SetSmoothing allows for motion through space to be discrete or smoothed (continuous)
// TODO: After triage, smoothing enabled will just reroute to motion prediction.
func (sm *SensorManager) SetSmoothing(enabled bool) {
	sm.smoothingEnabled = enabled
}

// NewSensorManager creates a new sensor manager
func NewSensorManager(camera *Camera) *SensorManager {
	return &SensorManager{
		camera:           camera,
		smoothingEnabled: true,
		ctxGPS:           nil,
		cancelGPS:        nil,
		ctxHat:           nil,
		cancelHat:        nil,
	}
}

// StartGPS connects to the GPSD service and gets lat, lon, and alt with TPV report
// Runs as a separate goroutine
func (sm *SensorManager) StartGPS(ctx context.Context) error {
	// Connect to GPSD
	gps, err := gpsd.Dial("localhost:2947")
	if err != nil {
		log.Println("Failed to connect to GPSD:", err)
		return err
	}
	// defer gps.Close()

	// Store previous data in case of GPS fix issues
	pos := sm.camera.Position()
	oldX := pos[0]
	oldY := pos[1]
	oldZ := pos[2]
	var xdif, ydif, zdif float32

	if sm.smoothingEnabled && len(sm.gpsRecordings) == 0 {
		sm.gpsRecordings = append(sm.gpsRecordings, GPSReading{
			position: pos,
			occurred: time.Now(),
			velocity: lin.Vec3{0, 0, 0},
		})
		log.Println("Setup: Initial position recorded.")
	}

	// Set up the TPV function that will run indefinitely
	tpvFilter := func(r any) {
		report := r.(*gpsd.TPVReport)

		if sm.smoothingEnabled {
			if report.Mode == gpsd.NoFix || report.Mode == gpsd.NoValueSeen {
				log.Println("GPS: No fix, using predicted position based on previous movement")
				return
			}
			sm.pushGPS(report)
			return
		}

		// Convert GPS coordinates to world coordinates
		newY := float32(report.Alt) * HEIGHT_CONVERSION
		newX, newZ := coords.GPSToWorld(float32(report.Lat), float32(report.Lon))

		// Check if data is faulty, if so, assume position based on history
		if report.Mode == gpsd.NoFix || report.Mode == gpsd.NoValueSeen {
			log.Println("GPS: No fix, using predicted position based on previous movement")
			newX = oldX + xdif
			newY = oldY + ydif
			newZ = oldZ + zdif
		}

		// Apply smoothing to reduce GPS jitter
		// Testing mode = no smoothing
		if sm.smoothingEnabled {
			newX, newY, newZ = Smoother32(newX, oldX, newY, oldY, newZ, oldZ)
		}

		// Update camera position
		sm.camera.updatePosition(newX, newZ, newY)

		// Log movement delta for prediction
		xdif = newX - oldX
		ydif = newY - oldY
		zdif = newZ - oldZ
		oldX = newX
		oldY = newY
		oldZ = newZ
	}

	// Add the filter and start watching
	gps.AddFilter("TPV", tpvFilter)

	// Start watching event stream
	done := gps.Watch()
	<-done

	select {
	case <-ctx.Done():
		log.Println("GPS: context cancelled, stopping GPS")
		gps.Close()
		return ctx.Err()
	case <-done:
		log.Println("GPS: watch ended")
		return nil
	}
}

// Push a reading based off TPV report (overwrites oldest when full)
// Ensure a valid update has been delivered before you call this!
func (sm *SensorManager) pushGPS(report *gpsd.TPVReport) {
	// Convert GPS coordinates to world coordinates
	newY := float32(report.Alt) * HEIGHT_CONVERSION
	newX, newZ := coords.GPSToWorld(float32(report.Lat), float32(report.Lon))
	newPos := lin.Vec3{newX, newY, newZ}

	// cache position, occurred time, and velocity
	// time
	t := time.Now()
	// vel
	var vel lin.Vec3
	if len(sm.gpsRecordings) > 0 {
		last := sm.gpsRecordings[len(sm.gpsRecordings)-1]
		dt := float32(t.Sub(last.occurred).Seconds())
		if dt > 0 {
			vel = lin.Vec3{
				(newPos[0] - last.position[0]) / dt,
				(newPos[1] - last.position[1]) / dt,
				(newPos[2] - last.position[2]) / dt,
			}
		}
	}

	// Construct the reading to be used for future reference
	reading := GPSReading{
		position: newPos,
		occurred: t,
		velocity: vel,
	}

	// Construct new spline iff buffer is now full. otherwise just add new reading
	if len(sm.gpsRecordings) < 3 {
		sm.gpsRecordings = append(sm.gpsRecordings, reading)
		return
	}

	// shift buffer
	sm.gpsRecordings[0] = sm.gpsRecordings[1]
	sm.gpsRecordings[1] = sm.gpsRecordings[2]
	sm.gpsRecordings[2] = reading

	// Update interarrival time
	mh := sm.camera.motionHandler
	if mh != nil && mh.splineManager != nil {
		dt := sm.gpsRecordings[2].occurred.Sub(sm.gpsRecordings[1].occurred).Seconds()
		if dt > 0 {
			if mh.splineManager.averageGPSInterval == 0 {
				mh.splineManager.averageGPSInterval = dt
			} else {
				mh.splineManager.averageGPSInterval = 0.7*mh.splineManager.averageGPSInterval + 0.3*dt
			}
		}
		mh.FeedGPSUpdate()
	}
}

// New GPS update.
func (mh *MotionHandler) FeedGPSUpdate() {
	if len(mh.camera.sensorManager.gpsRecordings) < 3 {
		return
	}
	spline := mh.splineManager
	g := mh.camera.sensorManager.gpsRecordings

	// Where camera actually is
	camPos := mh.camera.Position()

	// error between current camera position and current gps fix
	posErrorX := math.Abs(float64(camPos[0] - g[2].position[0]))
	posErrorY := math.Abs(float64(camPos[1] - g[2].position[1]))
	posErrorZ := math.Abs(float64(camPos[2] - g[2].position[2]))
	magnitude := math.Sqrt(posErrorX*posErrorX + posErrorY*posErrorY + posErrorZ*posErrorZ)
	// if error in predicted position vs. actual position exceeds allowance
	// then correct. harsh for now. TODO: implement smoothing w/ lerp?
	if magnitude >= CORRECTION_DISTANCE {
		mh.camera.SetPosition(g[2].position)
	}

	// Current velocity from GPS
	v2 := g[2].velocity

	// Compute p0 so tangent at p1 matches current velocity
	// Step backward from camera position along velocity direction
	camVel := mh.camera.smoothedVel
	speed := camVel.Len()
	var p0 lin.Vec3
	if speed > 0.1 {
		backDist := speed * float32(spline.averageGPSInterval)
		var velDir lin.Vec3
		velDir.Norm(&camVel)
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
		p0: p0,     // control point
		p1: camPos, // camera position (start)
		p2: pred1,  // predicted (end)
		p3: pred2,  // far prediction (control point)
	}
	spline.splineStartTime = time.Now()
	spline.currentVelocity = v2
}

// StartSensors gets yaw, pitch and roll from the sense hat via a pipe
// and updates the camera orientation indefinitely
// Runs as a separate goroutine
func (sm *SensorManager) StartSensors(ctx context.Context) Mode {
	filename := "./internal/camera/hat.py"
	cmd := exec.CommandContext(ctx, "python3", filename)

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

	defer func() {
		_ = cmd.Wait()
	}()

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
		// pitch = -pitch
		pitch = pitch + 3.14159/2

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
	_ = cmd.Wait()
	return Waiting
}

// applyMagneticDeclination applies magnetic declination correction to compass heading
func (sm *SensorManager) applyMagneticDeclination(yaw *float64) error {
	pos := sm.camera.Position()

	// Convert world coordinates to GPS for magnetic model
	// Note: This assumes coords package has the inverse function
	lat := float64(-pos[2]) / 3600.0           // Convert arcseconds to degrees
	lon := float64(pos[0]) / 3600.0            // Convert arcseconds to degrees
	alt := float64(pos[1]) / HEIGHT_CONVERSION // Convert to meters

	// Create location for magnetic model
	loc := egm96.NewLocationGeodetic(lat, lon, alt)

	// Calculate magnetic field
	mag, err := wmm.CalculateWMMMagneticField(loc, time.Now())
	if err != nil {
		return fmt.Errorf("failed to calculate magnetic field: %w", err)
	}

	// Apply declination correction (magnetic to true heading)
	*yaw = *yaw + mag.D()

	// log.Printf("Applied magnetic declination: %.2f° at position (%.4f, %.4f)",
	//	mag.D(), lat, lon)

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
