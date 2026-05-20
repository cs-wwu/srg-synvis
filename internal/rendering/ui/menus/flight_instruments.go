package menus

import (
	"math"

	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui/components"
	"github.com/elokore/cimgui-go-vulkan/imgui"
	"github.com/xlab/linmath"
)

type FlightInstruments struct {
	ui.UIComponent
	Gauge             *components.RectangleGauge
	Speedometer       *components.RectangleGauge
	Compass           *components.RibbonCompass
	Attitude          *components.AttitudeIndicator
	AttitudeDial      *components.AttitudeDial
	AirspaceIndicator *components.AirspaceIndicator
	PitchIndicator    *components.PitchIndicator
	currentAirspace   *rendering.Feature // nil when not in one

	app *rendering.Application
}

var NORTH_VECTOR = linmath.Vec3{0, 0, -1}
var GLOBAL_UP = linmath.Vec3{0, 1, 0}

func getCompassHeadingRadians(frontVector linmath.Vec3) float64 {
	frontVectorFlat := linmath.Vec3{frontVector[0], 0, frontVector[2]}
	frontVectorFlat.Norm(&frontVectorFlat)
	dotProduct := linmath.Vec3MultInner(&NORTH_VECTOR, &frontVectorFlat)
	angleRad := math.Acos(float64(dotProduct))

	if frontVectorFlat[0] < 0 {
		return (2 * math.Pi) - angleRad
	} else {
		return angleRad
	}
}

func getCameraRoll(cameraBasis []linmath.Vec3) float32 {
	forwardVector := cameraBasis[1]
	upVector := cameraBasis[0]

	var scaledForwardVector linmath.Vec3
	dot := linmath.Vec3MultInner(&GLOBAL_UP, &upVector)
	scaledForwardVector.Scale(&forwardVector, dot)

	var projectedWorldUp linmath.Vec3
	projectedWorldUp.Sub(&GLOBAL_UP, &scaledForwardVector)
	projectedWorldUp.Norm(&projectedWorldUp)

	var projectUpCross linmath.Vec3
	projectUpCross.MultCross(&projectedWorldUp, &upVector)
	x := linmath.Vec3MultInner(&projectedWorldUp, &upVector)
	y := linmath.Vec3MultInner(&forwardVector, &projectUpCross)

	return float32(math.Atan2(float64(y), float64(x)))
}

func getCameraPitch(cameraBasis []linmath.Vec3) float32 {
	forwardVector := cameraBasis[1]

	y := linmath.Vec3MultInner(&forwardVector, &GLOBAL_UP)
	x := math.Sqrt(1 - math.Pow(float64(y), 2))

	return float32(math.Atan2(float64(y), x))
}

func NewFlightInstruments() *FlightInstruments {
	compass := components.NewRibbonCompass(*ui.NewUDim(0.5, 0, 0, 10), *ui.UDimFromPixels(300, 45))
	compass.AnchorPoint = imgui.NewVec2(0.5, 0)

	altimeter := components.NewRectangleGauge("Altitude (u)", *ui.NewUDim(0, 25, 0.5, 0), *ui.UDimFromPixels(75, 300))
	altimeter.AnchorPoint = imgui.NewVec2(0, 0.5)

	speedometer := components.NewRectangleGauge("Speed (u/s)", *ui.NewUDim(1, -25, 0.5, 0), *ui.UDimFromPixels(75, 300))
	speedometer.AnchorPoint = imgui.NewVec2(1, 0.5)
	speedometer.Interval = 5
	speedometer.MajorInterval = 10
	speedometer.MeterRange = 80

	attitude := components.NewAttitudeIndicator(*ui.NewUDim(2, 0, 0, 2))
	attitude.AttitudeAngleDegrees = 45

	attitudeDial := components.NewAttitudeDial()
	pitchIndicator := components.NewPitchIndicator()

	return &FlightInstruments{
		Gauge:             altimeter,
		Compass:           compass,
		Speedometer:       speedometer,
		Attitude:          attitude,
		AttitudeDial:      attitudeDial,
		PitchIndicator:    pitchIndicator,
		AirspaceIndicator: components.NewAirspaceIndicator(),
	}
}

func (window *FlightInstruments) Render() {
	cam := window.app.Camera
	frontVector := cam.GetBasis()[1]
	window.Compass.HeadingDegrees = linmath.RadiansToDegrees(float32(getCompassHeadingRadians(frontVector)))
	window.Attitude.AttitudeAngleDegrees = linmath.RadiansToDegrees(-getCameraRoll(cam.GetBasis()))
	window.AttitudeDial.AttitudeAngleDegrees = window.Attitude.AttitudeAngleDegrees
	window.Attitude.PitchAngleDegrees = getCameraPitch(cam.GetBasis())
	window.PitchIndicator.PitchAngleDegrees = window.Attitude.PitchAngleDegrees

	window.Speedometer.Value = cam.MeasureSpeed()

	window.Attitude.Render()
	window.Gauge.Value = window.app.Camera.Position()[1]
	window.Gauge.Render()
	window.Compass.Render()
	window.Speedometer.Render()
	window.AttitudeDial.Render()
	window.PitchIndicator.Render()

	window.currentAirspace = window.app.CurrentAirspace
	if window.currentAirspace != nil {
		window.AirspaceIndicator.Airspace = window.currentAirspace
		window.AirspaceIndicator.Render()
	}
}

func (window *FlightInstruments) UseApp(app *rendering.Application) {
	window.app = app
}

func init() {
	window := NewFlightInstruments()
	ui.RegisterUIComponent(window)
}
