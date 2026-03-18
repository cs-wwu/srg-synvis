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
	Gauge       *components.RectangleGauge
	Speedometer *components.RectangleGauge
	Compass     *components.RibbonCompass
	app         *rendering.Application
}

var NORTH_VECTOR = linmath.Vec3{0, 0, -1}

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

	return &FlightInstruments{
		Gauge:       altimeter,
		Compass:     compass,
		Speedometer: speedometer,
	}
}

func (window *FlightInstruments) Render() {
	cam := window.app.Camera
	frontVector := cam.GetBasis()[1]
	window.Compass.HeadingDegrees = linmath.RadiansToDegrees(float32(getCompassHeadingRadians(frontVector)))

	window.Speedometer.Value = cam.MeasureSpeed()

	window.Gauge.Value = window.app.Camera.Position()[1]
	window.Gauge.Render()
	window.Compass.Render()
	window.Speedometer.Render()
}

func (window *FlightInstruments) UseApp(app *rendering.Application) {
	window.app = app
}

func init() {
	window := NewFlightInstruments()
	ui.RegisterUIComponent(window)
}
