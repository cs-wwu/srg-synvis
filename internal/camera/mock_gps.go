package camera

import (
	"log"
	"math"
	"time"

	lin "github.com/xlab/linmath"
)

// StartMockGPS simulates GPS updates for testing
func (sm *SensorManager) StartMockGPS(path []lin.Vec3, updateInterval time.Duration) {
	log.Println("Starting mock GPS with synthetic path")

	// Initialize if empty
	if len(sm.gpsRecordings) == 0 {
		sm.gpsRecordings = append(sm.gpsRecordings, GPSReading{
			position: path[0],
			occurred: time.Now(),
			velocity: lin.Vec3{0, 0, 0},
		})
	}

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

		// Push it like real GPS (maintain rolling buffer of 3)
		sm.gpsRecordings = append(sm.gpsRecordings, reading)
		if len(sm.gpsRecordings) > 3 {
			sm.gpsRecordings = sm.gpsRecordings[1:]
		}

		// For mock GPS, we know the interval exactly
		mh := sm.camera.motionHandler
		if mh != nil && mh.splineManager != nil {
			mh.splineManager.averageGPSInterval = float32(updateInterval.Seconds())
		}

		// Trigger spline update
		if len(sm.gpsRecordings) >= 3 {
			sm.camera.motionHandler.FeedGPSUpdate()
		}

		log.Printf("Mock GPS [%d/%d]: pos=(%.1f, %.1f, %.1f), vel=(%.1f, %.1f, %.1f)",
			i, len(path)-1,
			path[i][0], path[i][1], path[i][2],
			velocity[0], velocity[1], velocity[2])
	}

	log.Println("Mock GPS path completed")
}

// GenerateTestFlightPath creates a realistic flight path with various maneuvers
func GenerateTestFlightPath() []lin.Vec3 {
	path := []lin.Vec3{}

	// Takeoff and initial climb (0-10 seconds)
	for i := 0; i < 10; i++ {
		path = append(path, lin.Vec3{
			float32(i * 50),
			float32(100 + i*20),
			0,
		})
	}

	// Level flight heading east (10-30 seconds)
	for i := 10; i < 30; i++ {
		path = append(path, lin.Vec3{
			float32(i * 50),
			300,
			0,
		})
	}

	// Gradual right turn to south (30-50 seconds)
	turnCenter := lin.Vec3{1500, 300, -500}
	radius := float32(500)
	for i := 0; i < 20; i++ {
		angle := float32(i) * (math.Pi / 2) / 20
		path = append(path, lin.Vec3{
			turnCenter[0] + radius*float32(math.Cos(float64(angle))),
			300,
			turnCenter[2] + radius*float32(math.Sin(float64(angle))),
		})
	}

	// Level flight heading south (50-70 seconds)
	for i := 0; i < 20; i++ {
		path = append(path, lin.Vec3{
			1500,
			300,
			float32(-500 + i*50),
		})
	}

	// Sharp left turn to east (70-85 seconds)
	turnCenter2 := lin.Vec3{1800, 300, 500}
	radius2 := float32(300)
	for i := 0; i < 15; i++ {
		angle := float32(math.Pi/2 + float64(i)*(math.Pi/2)/15)
		path = append(path, lin.Vec3{
			turnCenter2[0] + radius2*float32(math.Cos(float64(angle))),
			300,
			turnCenter2[2] + radius2*float32(math.Sin(float64(angle))),
		})
	}

	// Climbing spiral (85-115 seconds)
	spiralCenter := lin.Vec3{2100, 300, 500}
	spiralRadius := float32(400)
	for i := 0; i < 30; i++ {
		angle := float64(i) * (2 * math.Pi) / 30
		path = append(path, lin.Vec3{
			spiralCenter[0] + spiralRadius*float32(math.Cos(angle)),
			300 + float32(i*10),
			spiralCenter[2] + spiralRadius*float32(math.Sin(angle)),
		})
	}

	// Level flight with deceleration (115-135 seconds)
	lastPos := path[len(path)-1]
	for i := 0; i < 20; i++ {
		speed := float32(50 - i)
		if speed < 30 {
			speed = 30
		}
		lastPos = lin.Vec3{lastPos[0] + speed, lastPos[1], lastPos[2]}
		path = append(path, lastPos)
	}

	// Sharp zigzag (135-155 seconds)
	startZigzag := path[len(path)-1]
	for i := 0; i < 20; i++ {
		zigzagOffset := float32(100)
		if i%2 == 0 {
			zigzagOffset = -100
		}
		path = append(path, lin.Vec3{
			startZigzag[0] + float32(i*30),
			startZigzag[1],
			startZigzag[2] + zigzagOffset,
		})
	}

	// Final approach (155-175 seconds)
	finalStart := path[len(path)-1]
	for i := 0; i < 20; i++ {
		path = append(path, lin.Vec3{
			finalStart[0] + float32(i*40),
			finalStart[1] - float32(i*10),
			finalStart[2],
		})
	}

	// Touchdown and rollout (175-185 seconds)
	touchdownStart := path[len(path)-1]
	for i := 0; i < 10; i++ {
		decel := float32(40 - i*4)
		path = append(path, lin.Vec3{
			touchdownStart[0] + decel,
			100,
			touchdownStart[2],
		})
	}

	return path
}

// GenerateSimpleTestPath creates a simple square pattern
func GenerateSimpleTestPath() []lin.Vec3 {
	path := []lin.Vec3{}

	// Square pattern
	for i := 0; i < 20; i++ {
		path = append(path, lin.Vec3{0, 200, float32(i * 50)})
	}
	for i := 0; i < 20; i++ {
		path = append(path, lin.Vec3{float32(i * 50), 200, 1000})
	}
	for i := 0; i < 20; i++ {
		path = append(path, lin.Vec3{1000, 200, float32(1000 - i*50)})
	}
	for i := 0; i < 20; i++ {
		path = append(path, lin.Vec3{float32(1000 - i*50), 200, 0})
	}

	return path
}

// GenerateFigure8Path creates a figure-8 pattern
func GenerateFigure8Path() []lin.Vec3 {
	path := []lin.Vec3{}
	radius := float32(500)

	// First loop (clockwise)
	for i := 0; i < 40; i++ {
		angle := float64(i) * (2 * math.Pi) / 40
		path = append(path, lin.Vec3{
			radius*float32(math.Cos(angle)) - radius,
			200,
			radius * float32(math.Sin(angle)),
		})
	}

	// Second loop (counter-clockwise)
	for i := 0; i < 40; i++ {
		angle := float64(i) * (2 * math.Pi) / 40
		path = append(path, lin.Vec3{
			radius*float32(math.Cos(angle)) + radius,
			200,
			-radius * float32(math.Sin(angle)),
		})
	}

	return path
}

// GenerateTestFlightPathFrom creates a test path starting from a specific position
func GenerateTestFlightPathFrom(startPos lin.Vec3) []lin.Vec3 {
	path := []lin.Vec3{}

	// Start from the given position
	path = append(path, startPos)

	// Simple path: fly in a square from starting position
	for i := 1; i < 20; i++ {
		path = append(path, lin.Vec3{
			startPos[0] + float32(i*50),
			startPos[1],
			startPos[2],
		})
	}

	for i := 1; i < 20; i++ {
		path = append(path, lin.Vec3{
			startPos[0] + 1000,
			startPos[1],
			startPos[2] + float32(i*50),
		})
	}

	for i := 1; i < 20; i++ {
		path = append(path, lin.Vec3{
			startPos[0] + float32(1000-i*50),
			startPos[1],
			startPos[2] + 1000,
		})
	}

	for i := 1; i < 20; i++ {
		path = append(path, lin.Vec3{
			startPos[0],
			startPos[1],
			startPos[2] + float32(1000-i*50),
		})
	}

	return path
}
