package renderer

// This file should be used to manipulate the camera position and vectors

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os/exec"
	"sync"
	"time"

	"github.com/cs-wwu/srg-synvis/internal/sensors"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/stratoberry/go-gpsd"
	quat "github.com/ungerik/go3d/float64/quaternion"
	"github.com/ungerik/go3d/float64/vec3"
	"github.com/westphae/geomag/pkg/egm96"
	"github.com/westphae/geomag/pkg/wmm"

	lin "github.com/xlab/linmath"
)

const (
	HEIGHT_CONVERSION = 60.0 / 1852.0
	FPS               = 60 // Frames per second
	SPR               = 2  // Seconds per 360 degree rotation

	// flught constants
	DEFAULT_STARTING_SPEED = 250.0
	DEFAULT_MAX_SPEED      = 1000.0
	DEFAULT_ACCEL_RATE     = 100.0
	DEFAULT_DECEL_RATE     = 50.0
	DEFAULT_BANK_FACTOR    = 0.7
	DEFAULT_MAX_ROLL_ANGLE = math.Pi / 2 // 90 degrees
	DEFAULT_AUTO_LEVEL     = 0.5
)

type Mode int

const (
	StartPipe Mode = iota
	StartCommand
	ReadPipe
	ParseInput
	Waiting
)

var (
	START_CAMERA_UP      = lin.Vec3{0.0, 1.0, 0.0}
	START_CAMERA_FRONT   = lin.Vec3{0, 0, -1.0}
	Q_START_CAMERA_UP    = vec3.T{0.0, 1.0, 0.0}
	Q_START_CAMERA_FRONT = vec3.T{0, 0, -1.0}
	Q_START_CAMERA_RIGHT = vec3.Cross(&Q_START_CAMERA_UP, &Q_START_CAMERA_FRONT)
	CAMERA_SPEED         = float32(10.0)
	LOOK_SENSITIVITY     = float32((2 * math.Pi) / (SPR * FPS))
)

type Camera struct {
	Pos   *lin.Vec3 // (x, y, z)
	Up    *lin.Vec3 // Up Direction of the camera
	Front *lin.Vec3 // Front direction of the camera
	Right *lin.Vec3 // Right direction of the Camera

	frontMutex sync.Mutex // Mutex for updating the front vector
	posMutex   sync.Mutex // Mutex for updating the position vector
	upMutex    sync.Mutex // Mutex for updating the up vector

	Speed       float32 // Camera speed
	Sensitivity float32 // Camera sensitivity

	Pitch float64 // Pitch relative to start
	Yaw   float64 // Yaw relative to start
	Roll  float64 // Roll angle relative to start

	Decimal_year time.Time // Date for magnetic world

	// Debug plane acceleration
	Velocity         lin.Vec3 // Current velocity vector
	LastUpdateTime   float64  // Last time the camera was updated
	DeltaTime        float32  // Time between frames
	MaxSpeed         float32  // Maximum flying speed
	CurrentSpeed     float32  // Current flying speed
	AccelerationRate float32  // How quickly speed increases
	DecelerationRate float32  // How quickly speed decreases
	BankFactor       float32  // How much roll affects turning
	TurnRate         float32  // Base turning rate
	MaxRollAngle     float64  // Maximum banking angle in radians
	AutoLevelFactor  float32  // How quickly the aircraft returns to level

	// Inertia/smoothing fields
	TargetPitch  float64 // Target pitch angle
	TargetRoll   float64 // Target roll angle
	TargetYaw    float64 // Target yaw angle
	PitchInertia float32 // How quickly pitch catches up to target (lower = more inertia)
	RollInertia  float32 // How quickly roll catches up to target
	YawInertia   float32 // How quickly yaw catches up to targe
}

// NewCamera creates and initializes a new Camera
func NewCamera(hat bool, startPos lin.Vec3) *Camera {
	today := time.Now()

	pos := startPos
	up := START_CAMERA_UP

	// Calculate initial vectors based on identity rotation
	front := START_CAMERA_FRONT
	right := lin.Vec3{}
	right.MultCross(&front, &up)
	right.Norm(&right)

	up.MultCross(&right, &front)
	up.Norm(&up)

	log.Printf("Initialized Camera at (%f, %f)\n", pos[0], pos[2])

	return &Camera{
		Pos:              &pos,
		Up:               &up,
		Front:            &front,
		Right:            &right,
		Speed:            CAMERA_SPEED,
		Sensitivity:      LOOK_SENSITIVITY,
		Decimal_year:     today,
		Pitch:            0.0,
		Roll:             0.0,
		Yaw:              0.0,
		Velocity:         lin.Vec3{0, 0, 0},
		LastUpdateTime:   glfw.GetTime(),
		DeltaTime:        0.0,
		MaxSpeed:         100.0,
		CurrentSpeed:     30.0, // Start with some speed already
		AccelerationRate: 15.0,
		DecelerationRate: 5.0,
		BankFactor:       0.5, // Reduced from 0.8 to make banking less aggressive
		TurnRate:         1.0,
		MaxRollAngle:     math.Pi / 4, // Maximum 45 degree bank
		AutoLevelFactor:  0.5,         // Auto-level when no roll input
        TargetPitch:      0.0,
        TargetRoll:       0.0,
        TargetYaw:        0.0,
        PitchInertia:     0.15,  // Lower = more sluggish pitch response
        RollInertia:      0.25,  // Medium roll response
        YawInertia:       0.20,  // Moderate yaw smoothing
	}
}

// GetViewMatrix returns the camera's view matrix
func (c *Camera) GetViewMatrix() lin.Mat4x4 {
	center := lin.Vec3{}
	center.Add(c.Pos, c.Front)

	result := lin.Mat4x4{}
	result.LookAt(c.Pos, &center, c.Up)
	return result
}

// Connect to the gpsd service and get lat, lon, and alt with TPV report until closed
// Run as one Go thread separate from main
func (c *Camera) TryGps() error {
	// Connect to gspd
	gps, err := gpsd.Dial("localhost:2947")
	if err != nil {
		// log.Fatalf("Failed to connect to GPSD: %s", err)
		log.Println("Failed to connect to GPSD: ", err)
		return err
	}

	// Store previous data in case of gps fix issues
	oldX := c.Pos[0]
	oldY := c.Pos[1]
	oldZ := c.Pos[2]
	xdif := float32(0)
	ydif := float32(0)
	zdif := float32(0)

	// Set up the TPV function that will run indefintely
	tpvFilter := func(r any) {
		// Get gpsd data
		report := r.(*gpsd.TPVReport)
		log.Println("Location updated:", report.Lat, report.Lon, report.Alt)
		newY := float32(report.Alt) * HEIGHT_CONVERSION
		newX, newZ := CoordinateGPSFloatToFloat(float32(report.Lat), float32(report.Lon))
		// fmt.Println("Location:", report.Lat, report.Lon)

		// Check if data is faulty, if so, assume spot based on history
		// if (report.Lat == 0 && report.Lon == 0) {
		if report.Mode == gpsd.NoFix || report.Mode == gpsd.NoValueSeen {
			log.Println("No fix, using prior data")
			newX = oldX + xdif
			newY = oldY + ydif
			newZ = oldZ + zdif
		}

		// Update position
		newX, newY, newZ = sensors.Smoother32(newX, oldX, newY, oldY, newZ, oldZ)
		c.UpdatePos(newX, newZ, newY)

		// Log data
		xdif = oldX - newX
		ydif = oldY - newY
		zdif = oldZ - newZ
		oldX = newX
		oldY = newY
		oldZ = newZ
	}
	gps.AddFilter("TPV", tpvFilter)

	// Start watching event stream with channel done to signal stop
	done := gps.Watch()
	<-done

	gps.Close()
	return nil
}

// Gets yaw, pitch and roll from the sense hat via a pipe and updates the front vector indefinitely
// Run as one Go thread separate from main
func (c *Camera) TrySense(filename string) Mode {
	var md Mode
	cmd := exec.Command("python3", filename)

	// Create a pipe for standard output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		return StartPipe
	}
	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Println("Error starting command:", err)
		return StartCommand
	}

	first := true
	var oldYaw, oldRoll, oldPitch float64

	// Read the output from the pipe
	reader := bufio.NewReader(stdout)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		// Parse the sense data
		n, err := fmt.Sscanf(input, "[%f, %f, %f]", &c.Yaw, &c.Roll, &c.Pitch)
		if err != nil || n != 3 {
			fmt.Println("Error parsing input:", err)
			md = ParseInput
			continue
		}

		c.Pitch = -c.Pitch

		// Account for magnetic world deviation

		// LAT, LON, HEIGHT
		loc := egm96.NewLocationGeodetic(-float64(c.Pos[2])/3600, float64(c.Pos[0])/3600, float64(c.Pos[1])/HEIGHT_CONVERSION)
		mag, _ := wmm.CalculateWMMMagneticField(loc, c.Decimal_year)
		fmt.Printf("Declination at your location: %2.2f\n", mag.D())
		c.Yaw = c.Yaw + mag.D() // Magnetic to true: add declination
		// TODO databackup?

		if !first {
			c.Yaw, c.Roll, c.Pitch = sensors.Smoother64(c.Yaw, oldYaw, c.Roll, oldRoll, c.Pitch, oldPitch)
		}
		first = false

		// Rotate the camera based on the read Yaw, Roll, and Pitch
		c.QuatRotateAbs()

		// Log data
		oldYaw = c.Yaw
		oldRoll = c.Roll
		oldPitch = c.Pitch
	}

	// Print the output
	// fmt.Println("output " + string(output))
	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		fmt.Println("Error waiting for command:", err)
		return Waiting
	}

	return md
}

// Processes input from the glfw window and moves the camera accordingly
// This function is mainly for debugging purposes
// Uses WASD for movement, and arrow keys for viewing
// TODO: turn on / off parts based if getting gps or sense?
func (c *Camera) ProcessInput(window *glfw.Window) {
    // Calculate delta time
    currentTime := glfw.GetTime()
    c.DeltaTime = float32(currentTime - c.LastUpdateTime)
    // Clamp delta time to avoid huge jumps if the application freezes briefly
    if c.DeltaTime > 0.1 {
        c.DeltaTime = 0.1
    }
    c.LastUpdateTime = currentTime
    
    if window.GetAttrib(glfw.Focused) == glfw.True {
        // Emergency exit with Ctrl+C
        if window.GetKey(glfw.KeyLeftControl) == glfw.Press &&
            window.GetKey(glfw.KeyC) == glfw.Press {
            window.SetShouldClose(true)
        }
        
        // Calculate pitch influence on acceleration (like gravity)
        // Get the pitch angle relative to horizon - not the pitch rate
        pitchAngle := math.Asin(float64(c.Front[1])) // Extract pitch from front vector
        
        // Modify acceleration based on pitch - negative pitch speeds up, positive slows down
        pitchFactor := 1.0 - float64(pitchAngle) * 2.0 // Scale to make a reasonable effect
        
        // Throttle control (Forward acceleration) with pitch influence
        baseAccelRate := float64(c.AccelerationRate) * pitchFactor
        if baseAccelRate < 0 {
            baseAccelRate = 0 // Prevent negative acceleration from steep climb
        }
        
        if window.GetKey(glfw.KeyW) == glfw.Press {
            c.CurrentSpeed += float32(baseAccelRate) * c.DeltaTime
            if c.CurrentSpeed > c.MaxSpeed {
                c.CurrentSpeed = c.MaxSpeed
            }
        } else if window.GetKey(glfw.KeyS) == glfw.Press {
            c.CurrentSpeed -= c.DecelerationRate * c.DeltaTime
            if c.CurrentSpeed < 0 {
                c.CurrentSpeed = 0
            }
        } else {
            // Natural deceleration - increased when climbing
            decelRate := c.DecelerationRate
            if pitchAngle > 0 {
                decelRate = c.DecelerationRate * float32(1.0 + pitchAngle*3.0)
            }
            c.CurrentSpeed -= decelRate * c.DeltaTime
            if c.CurrentSpeed < 0 {
                c.CurrentSpeed = 0
            }
        }
        
        // Flight controls - Input processing
        // --------------------------------
        // Scales for control inputs
        baseSensitivity := float64(c.Sensitivity) * 60.0
        pitchInputRate := 0.0
        rollInputRate := 0.0
        yawInputRate := 0.0
        
        // Pitch control - target value with inertia
        if window.GetKey(glfw.KeyUp) == glfw.Press {
            pitchInputRate += baseSensitivity * float64(c.DeltaTime)
        }
        if window.GetKey(glfw.KeyDown) == glfw.Press {
            pitchInputRate -= baseSensitivity * float64(c.DeltaTime)
        }
        
        // Roll control with swapped keys
        rollInput := false
        if window.GetKey(glfw.KeyQ) == glfw.Press {
            // Q now rolls RIGHT
            rollInputRate += baseSensitivity * float64(c.DeltaTime) * 1.5 // Slightly faster roll
            rollInput = true
        }
        if window.GetKey(glfw.KeyE) == glfw.Press {
            // E now rolls LEFT
            rollInputRate -= baseSensitivity * float64(c.DeltaTime) * 1.5
            rollInput = true
        }
        
        // Direct yaw control (Rudder)
        if window.GetKey(glfw.KeyRight) == glfw.Press {
            yawInputRate += baseSensitivity * float64(c.DeltaTime)
        }
        if window.GetKey(glfw.KeyLeft) == glfw.Press {
            yawInputRate -= baseSensitivity * float64(c.DeltaTime)
        }
        
        // Update target rotations based on inputs
        // ------------------------------------
        // Extract current pitch angle from Front vector
        currentPitchAngle := math.Asin(float64(c.Front[1]))
        
        // Update target pitch with input (add input to current angle)
        c.TargetPitch = currentPitchAngle + pitchInputRate
        
        // Limit pitch to reasonable range to prevent loops
        maxPitch := 85.0 * math.Pi / 180.0 // 85 degrees in radians
        if c.TargetPitch > maxPitch {
            c.TargetPitch = maxPitch
        } else if c.TargetPitch < -maxPitch {
            c.TargetPitch = -maxPitch
        }
        
        // Update target roll
        c.TargetRoll += rollInputRate
        
        // Apply roll limits
        if c.TargetRoll > c.MaxRollAngle {
            c.TargetRoll = c.MaxRollAngle
        } else if c.TargetRoll < -c.MaxRollAngle {
            c.TargetRoll = -c.MaxRollAngle
        }
        
        // Auto-level when no roll input
        if !rollInput && c.TargetRoll != 0 {
            // Calculate auto-level rate based on current roll angle
            rollMagnitude := math.Abs(c.TargetRoll)
            normalizedRoll := rollMagnitude / c.MaxRollAngle
            
            // Smoother auto-level rate
            autoLevelStrength := float64(c.AutoLevelFactor) * normalizedRoll * float64(c.DeltaTime) * 1.5
            
            // Apply auto-level to target roll
            if c.TargetRoll > 0 {
                c.TargetRoll -= autoLevelStrength
                if c.TargetRoll < 0 {
                    c.TargetRoll = 0 // Prevent overshooting
                }
            } else {
                c.TargetRoll += autoLevelStrength
                if c.TargetRoll > 0 {
                    c.TargetRoll = 0 // Prevent overshooting
                }
            }
        }
        
        // Apply yaw input directly to yaw rate
        // We'll handle banking separately
        yawRate := yawInputRate
        
        // Apply banking effect - roll influences yaw rate
        bankingYaw := -c.Roll * float64(c.BankFactor) * float64(c.DeltaTime) * 1.2
        yawRate += bankingYaw
        
        // Calculate smooth rotation transitions
        // ------------------------------------
        // Extract current orientation for smooth transition
        currentRoll := c.Roll
        
        // Smoothly interpolate between current and target pitch
        // This is what gives the feeling of inertia in pitch movement
        pitchDiff := c.TargetPitch - currentPitchAngle
        pitchChange := pitchDiff * float64(c.PitchInertia)
        
        // Smoothly interpolate between current and target roll 
        rollDiff := c.TargetRoll - currentRoll
        rollChange := rollDiff * float64(c.RollInertia)
        
        // Add yaw smoothing
        // We don't track target yaw, just smooth the input
        yawChange := yawRate * float64(c.YawInertia)
        
        // Apply the smoothed rotation changes
        c.ApplyPitchRotation(pitchChange)
        c.Roll += rollChange
        c.ApplyRotation(yawChange, 0, rollChange) // Apply yaw and roll
        
        // Update position based on current speed and direction
        c.UpdatePosition()
        
        // Debug info with R key
        if window.GetKey(glfw.KeyR) == glfw.Press {
            fmt.Printf("Pos: %v\n", c.GetPositionString())
            fmt.Printf("Speed: %v\n", c.CurrentSpeed)
            fmt.Printf("Current Pitch: %v°, Target: %v°\n", 
                      currentPitchAngle * 180 / math.Pi, 
                      c.TargetPitch * 180 / math.Pi)
            fmt.Printf("Current Roll: %v°, Target: %v°\n", 
                      currentRoll * 180 / math.Pi, 
                      c.TargetRoll * 180 / math.Pi)
            fmt.Printf("Vectors: %v\n", c.GetViewVectors())
        }
    }
}

// ApplyPitchRotation applies a specific pitch change in a smoother way
func (c *Camera) ApplyPitchRotation(pitchChange float64) {
    if pitchChange == 0 {
        return
    }
    
    // Convert lin.vec3 to quat.vec3
    qRight := vec3.T{float64(c.Right[0]), float64(c.Right[1]), float64(c.Right[2])}
    qFront := vec3.T{float64(c.Front[0]), float64(c.Front[1]), float64(c.Front[2])}
    qUp := vec3.T{float64(c.Up[0]), float64(c.Up[1]), float64(c.Up[2])}

    // Create rotation quaternion for pitch
    qPitch := quat.FromAxisAngle(&qRight, pitchChange)

    // Apply rotation only to front and up vectors
    qPitch.RotateVec3(&qFront)
    qPitch.RotateVec3(&qUp)
    
    // Normalize to prevent drift
    qFront.Normalize()
    qUp.Normalize()

    // Update camera vectors with thread safety
    c.frontMutex.Lock()
    c.upMutex.Lock()
    c.Front = &lin.Vec3{float32(qFront[0]), float32(qFront[1]), float32(qFront[2])}
    c.Up = &lin.Vec3{float32(qUp[0]), float32(qUp[1]), float32(qUp[2])}
    c.upMutex.Unlock()
    c.frontMutex.Unlock()
}

func (c *Camera) ApplyRotation(yawRate, pitchRate, rollRate float64) {
	// Convert lin.vec3 to quat.vec3
	qUp := vec3.T{float64(c.Up[0]), float64(c.Up[1]), float64(c.Up[2])}
	qFront := vec3.T{float64(c.Front[0]), float64(c.Front[1]), float64(c.Front[2])}
	qRight := vec3.T{float64(c.Right[0]), float64(c.Right[1]), float64(c.Right[2])}

	// Create rotation quaternions for each axis
	qYaw := quat.FromAxisAngle(&Q_START_CAMERA_UP, -yawRate)
	qRoll := quat.FromAxisAngle(&qFront, -rollRate)
	qPitch := quat.FromAxisAngle(&qRight, pitchRate)

	// Combine rotations
	rotQuat := quat.Mul3(&qYaw, &qPitch, &qRoll)

	// Apply rotation to direction vectors
	rotQuat.RotateVec3(&qFront)
	rotQuat.RotateVec3(&qRight)
	rotQuat.RotateVec3(&qUp)

	// Normalize to prevent drift over time
	qFront.Normalize()
	qRight.Normalize()
	qUp.Normalize()

	// Update camera vectors with thread safety
	c.frontMutex.Lock()
	c.upMutex.Lock()
	c.Front = &lin.Vec3{float32(qFront[0]), float32(qFront[1]), float32(qFront[2])}
	c.Right = &lin.Vec3{float32(qRight[0]), float32(qRight[1]), float32(qRight[2])}
	c.Up = &lin.Vec3{float32(qUp[0]), float32(qUp[1]), float32(qUp[2])}
	c.upMutex.Unlock()
	c.frontMutex.Unlock()
}

func (c *Camera) UpdatePosition() {
	// Calculate movement based on current speed and front vector
	moveAmount := lin.Vec3{}
	moveAmount.Scale(c.Front, c.CurrentSpeed*c.DeltaTime)

	// Apply movement to position
	c.AddPos(moveAmount)
}

func (c *Camera) GetPositionString() string {
	return fmt.Sprintf("%v, %s", c.Pos, CoordinateFloatToGPS(c.Pos[0], c.Pos[2]))
}

func (c *Camera) GetViewVectors() string {
	return fmt.Sprintf("(Up: %v, Front: %v, Right: %v)", c.Up, c.Front, c.Right)
}

func (c *Camera) UpdateFront(front lin.Vec3) {
	c.frontMutex.Lock()
	c.Front = &front
	c.frontMutex.Unlock()
}

func (c *Camera) UpdatePos(newX, newZ, newY float32) {
	c.posMutex.Lock()
	(*c.Pos)[0] = newX
	(*c.Pos)[2] = newZ
	(*c.Pos)[1] = newY
	c.posMutex.Unlock()
}

func (c *Camera) AddPos(tmp lin.Vec3) {
	c.posMutex.Lock()
	c.Pos.Add(c.Pos, &tmp)
	c.posMutex.Unlock()
}

func (c *Camera) SubPos(tmp lin.Vec3) {
	c.posMutex.Lock()
	c.Pos.Sub(c.Pos, &tmp)
	c.posMutex.Unlock()
}

// Calculates rotations based on the original up, fornt, and right vectors
func (c *Camera) QuatRotateAbs() {
	// Calculate the rotation quaternions for each component
	qYaw := quat.FromAxisAngle(&Q_START_CAMERA_UP, -c.Yaw)
	qRoll := quat.FromAxisAngle(&Q_START_CAMERA_FRONT, -c.Roll)
	qPitch := quat.FromAxisAngle(&Q_START_CAMERA_RIGHT, -c.Pitch)

	// Calculate the rotation quaternion from Yaw, Roll, and Pitch
	rotQuat := quat.Mul3(&qYaw, &qPitch, &qRoll)

	// Rotate each vector based on the above quaternion rotation
	qFront := rotQuat.RotatedVec3(&Q_START_CAMERA_FRONT)
	qRight := rotQuat.RotatedVec3(&Q_START_CAMERA_RIGHT)
	qUp := rotQuat.RotatedVec3(&Q_START_CAMERA_UP)
	qFront.Normalize()
	qRight.Normalize()
	qUp.Normalize()

	// Save the newly rotated vectors back into the camera information
	c.frontMutex.Lock()
	c.upMutex.Lock()
	c.Front = &lin.Vec3{float32(qFront[0]), float32(qFront[1]), float32(qFront[2])}
	c.Right = &lin.Vec3{float32(qRight[0]), float32(qRight[1]), float32(qRight[2])}
	c.Up = &lin.Vec3{float32(qUp[0]), float32(qUp[1]), float32(qUp[2])}
	c.upMutex.Unlock()
	c.frontMutex.Unlock()
}
