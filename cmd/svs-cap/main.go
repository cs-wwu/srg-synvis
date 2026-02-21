package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"runtime"

	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/ungerik/go3d/float64/quaternion"
	dvec3 "github.com/ungerik/go3d/float64/vec3"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	lin "github.com/xlab/linmath"

	"golang.org/x/term"
)

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0

	METERS_TO_WORLD = 1852.0 / ARC_DEGREES_TO_MINUTES
	DEG_TO_RAD      = math.Pi / 180.0
)

func main() {
	var config models.AppConfig

	config.DebugMode = true

	config.CameraStartPos = lin.Vec3{-441970, 700, -171900}
	config.CameraStartDir = lin.Vec3{0, 0, -1}

	config.CaptureOnly = true
	config.CaptureWidth = 1024
	config.CaptureHeight = 1024

	config.RenderDistance = (150.0 * 1000.0) / METERS_TO_WORLD
	config.Fov = 70
	captures := configFromJson(&config)

	err := vk.SetDefaultGetInstanceProcAddr()
	if err != nil {
		log.Fatalf("error: failed to set default GetInstanceProcAddr: %v", err)
	}
	if err := vk.Init(); err != nil {
		log.Fatalf("error: failed to initialize Vulkan: %v", err)
	}

	app := rendering.NewApplication(config)
	as.NewPlatform(app)

	for i, capture := range captures {
		app.Camera.SetPosition(capture.position)
		app.Camera.SetOrientation(capture.yaw, capture.pitch, capture.roll)
		app.RenderCapture(fmt.Sprintf("capture_%d.png", i))
	}
}

type settingsInput struct {
	Fog            *bool    `json:"fog"`
	Fov            *float64 `json:"fov"`
	TerrainWarning *bool    `json:"terrain_warning"`
	Width          *uint32  `json:"width"`
	Height         *uint32  `json:"height"`
	RenderDistance *float64 `json:"render_distance"`
}

type captureInput struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Altitude  *float64 `json:"altitude"`
	Pitch     *float64 `json:"pitch"`
	Roll      *float64 `json:"roll"`
	Yaw       *float64 `json:"yaw"`
}

type requestInput struct {
	Settings *settingsInput `json:"settings"`
	Captures []captureInput `json:"captures"`
}

type capture struct {
	position lin.Vec3
	yaw      float64
	pitch    float64
	roll     float64
}

func configFromJson(config *models.AppConfig) []capture {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		log.Fatalf("error: stdin is a terminal, please pipe json to stdin")
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("error: failed to read stdin, using configured defaults: %v", err)
	}
	if len(input) == 0 {
		log.Fatal("error: stdin was empty, using configured defaults")
	}

	var payload requestInput
	if err := json.Unmarshal(input, &payload); err != nil {
		log.Fatalf("error: invalid json on stdin, using configured defaults: %v", err)
	}

	if payload.Settings != nil {
		if payload.Settings.Fog != nil {
			config.DisableFog = !*payload.Settings.Fog
		}
		if payload.Settings.Fov != nil {
			config.Fov = float32(*payload.Settings.Fov)
		}
		if payload.Settings.TerrainWarning != nil {
			config.DisableTerrainWarnings = !*payload.Settings.TerrainWarning
		}
		if payload.Settings.Width != nil && *payload.Settings.Width > 0 {
			config.CaptureWidth = *payload.Settings.Width
		}
		if payload.Settings.Height != nil && *payload.Settings.Height > 0 {
			config.CaptureHeight = *payload.Settings.Height
		}
		if payload.Settings.RenderDistance != nil && *payload.Settings.RenderDistance > 0 {
			config.RenderDistance = float32(*payload.Settings.RenderDistance / METERS_TO_WORLD)
		}
	}

	if len(payload.Captures) == 0 {
		log.Fatalf("error: no captures in stdin json")
	}

	captures := make([]capture, 0, len(payload.Captures))

	for _, cap := range payload.Captures {
		if cap.Latitude == nil || cap.Altitude == nil || cap.Longitude == nil {
			log.Fatalf("error: capture missing latitude/longitude/altitude")
		}
		if cap.Pitch == nil || cap.Roll == nil || cap.Yaw == nil {
			log.Fatalf("error: capture missing pitch/roll/yaw")
		}

		captures = append(captures, capture{
			position: lin.Vec3{
				float32(*cap.Longitude * ARC_DEGREES_TO_SECONDS),
				float32(*cap.Altitude / METERS_TO_WORLD),
				float32(-*cap.Latitude * ARC_DEGREES_TO_SECONDS),
			},
			yaw:   *cap.Yaw * DEG_TO_RAD,
			pitch: *cap.Pitch * DEG_TO_RAD,
			roll:  *cap.Roll * DEG_TO_RAD,
		})
	}

	firstCapture := captures[0]
	config.CameraStartPos = firstCapture.position

	rotationToDirection(
		firstCapture.yaw,
		firstCapture.pitch,
		firstCapture.roll,
		&config.CameraStartDir,
		&lin.Vec3{},
		&lin.Vec3{},
	)

	return captures
}

var (
	qStartCameraUp    = dvec3.T{0.0, 1.0, 0.0}
	qStartCameraFront = dvec3.T{0, 0, -1.0}
	qStartCameraRight = dvec3.Cross(&qStartCameraUp, &qStartCameraFront)
)

func rotationToDirection(yaw, pitch, roll float64, front, right, up *lin.Vec3) {
	// Calculate the rotation quaternions for each component
	qYaw := quaternion.FromAxisAngle(&qStartCameraUp, -yaw)
	qRoll := quaternion.FromAxisAngle(&qStartCameraFront, -roll)
	qPitch := quaternion.FromAxisAngle(&qStartCameraRight, -pitch)

	// Calculate the combined rotation quaternion from Yaw, Roll, and Pitch
	rotQuat := quaternion.Mul3(&qYaw, &qPitch, &qRoll)

	qFront := rotQuat.RotatedVec3(&qStartCameraFront)
	qRight := rotQuat.RotatedVec3(&qStartCameraRight)
	qUp := rotQuat.RotatedVec3(&qStartCameraUp)

	qFront.Normalize()
	qRight.Normalize()
	qUp.Normalize()

	*front = lin.Vec3{float32(qFront[0]), float32(qFront[1]), float32(qFront[2])}
	*right = lin.Vec3{float32(qRight[0]), float32(qRight[1]), float32(qRight[2])}
	*up = lin.Vec3{float32(qUp[0]), float32(qUp[1]), float32(qUp[2])}
}
