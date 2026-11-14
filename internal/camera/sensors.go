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
)

const DAMP = 0.9 // Smoothing factor for GPS data (position)

// Orientation smoothing time constants, in seconds. Larger = steadier but laggier.
// Yaw comes from the magnetometer and is much noisier than pitch/roll (which are
// held by the gyro + accelerometer), so it gets a longer time constant.
const (
	YAW_TAU  = 0.40
	TILT_TAU = 0.12
)

// Declination changes extremely slowly, so there is no reason to run the full
// magnetic model on every IMU sample.
const DECL_REFRESH = 10 * time.Second

// Set to false if mag.D() turns out to already be in radians.
// Sanity check: around Bellingham, WA the declination is roughly +15 degrees,
// so a degrees value prints as ~15 and a radians value prints as ~0.26.
const DECL_IS_DEGREES = true

// SensorManager handles GPS and IMU sensor integration
type SensorManager struct {
	camera           *Camera
	smoothingEnabled bool // When true, smooths path through space
	ctxGPS           context.Context
	cancelGPS        context.CancelFunc
	ctxHat           context.Context
	cancelHat        context.CancelFunc

	// Cached magnetic declination (radians)
	decl      float64
	declTime  time.Time
	declValid bool
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

	// Set up the TPV function that will run indefinitely
	tpvFilter := func(r any) {
		report := r.(*gpsd.TPVReport)
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

// wrapPi wraps an angle in radians to (-pi, pi].
func wrapPi(a float64) float64 {
	return math.Atan2(math.Sin(a), math.Cos(a))
}

// lowPassAngle moves oldA toward newA along the shortest arc by a fraction alpha,
// so angles blend correctly across the +/-pi wraparound.
func lowPassAngle(newA, oldA, alpha float64) float64 {
	return wrapPi(oldA + alpha*wrapPi(newA-oldA))
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
	var last time.Time

	// Read the output from the pipe
	reader := bufio.NewReader(stdout)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading sensor data:", err)
			break
		}

		// Parse the sensor data.
		// NOTE: hat.py prints [yaw, pitch, roll], but this scans the 2nd value into
		// `roll` and the 3rd into `pitch`. If that is not intentional (e.g. because of
		// how the hat is mounted), swap &roll and &pitch here and drop the pitch offset below.
		var yaw, roll, pitch float64
		n, err := fmt.Sscanf(input, "[%f, %f, %f]", &yaw, &roll, &pitch)
		if err != nil || n != 3 {
			log.Println("Error parsing sensor input:", err)
			continue
		}

		// Invert pitch for correct orientation
		// pitch = -pitch
		pitch = pitch + 3.14159/2

		// Apply magnetic declination correction (cached; see applyMagneticDeclination)
		if err := sm.applyMagneticDeclination(&yaw); err != nil {
			log.Printf("Warning: Could not apply magnetic declination: %v", err)
		}

		// Smooth with a time constant so the result does not depend on how fast
		// samples arrive, and so angles blend correctly across the wraparound.
		now := time.Now()
		if !first {
			dt := now.Sub(last).Seconds()
			aYaw := 1 - math.Exp(-dt/YAW_TAU)
			aTilt := 1 - math.Exp(-dt/TILT_TAU)
			yaw = lowPassAngle(yaw, oldYaw, aYaw)
			roll = lowPassAngle(roll, oldRoll, aTilt)
			pitch = lowPassAngle(pitch, oldPitch, aTilt)
		}
		last = now
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

// applyMagneticDeclination applies magnetic declination correction to compass heading.
// The declination is recomputed at most every DECL_REFRESH and cached in radians.
func (sm *SensorManager) applyMagneticDeclination(yaw *float64) error {
	if sm.declValid && time.Since(sm.declTime) < DECL_REFRESH {
		*yaw = *yaw + sm.decl
		return nil
	}

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

	d := mag.D()
	if DECL_IS_DEGREES {
		d = d * math.Pi / 180.0
	}
	sm.decl = d
	sm.declTime = time.Now()
	sm.declValid = true
	log.Printf("Magnetic declination: %.2f deg", d*180.0/math.Pi)

	// Apply declination correction (magnetic to true heading)
	*yaw = *yaw + d

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
