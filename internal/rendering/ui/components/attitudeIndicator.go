package components

import (
	"math"

	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
	"github.com/xlab/linmath"
)

type AttitudeIndicator struct {
	ui.UIComponent
	Size                 ui.UDim
	AttitudeAngleDegrees float32
	PitchAngleDegrees    float32
}

func toRadians(degrees float32) float32 {
	return degrees * (math.Pi / 180)
}

func NewAttitudeIndicator(size ui.UDim) *AttitudeIndicator {
	return &AttitudeIndicator{
		Size:                 size,
		AttitudeAngleDegrees: 0.0,
		PitchAngleDegrees:    0.0,
	}
}

func getVerticalOffset(pitch float32) float32 {
	screenHeight := ui.UDimFromScale(0, 1).ToSize().Y
	var fov float64 = float64(linmath.DegreesToRadians(60))
	focalLengthPixels := screenHeight / (2 * float32(math.Tan(fov/2)))
	return -focalLengthPixels * float32(math.Tan(float64(pitch)))
}

func (indicator *AttitudeIndicator) Render() {
	windowFlags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoBackground
	windowSize := ui.UDimFromScale(1, 1).ToSize()

	imgui.SetNextWindowSize(windowSize)
	imgui.SetNextWindowPos(ui.UDimFromScale(0, 0).ToPosition())
	imgui.BeginV("AttitudeWindow", nil, windowFlags)

	angleRadians := toRadians(indicator.AttitudeAngleDegrees)
	drawList := imgui.WindowDrawList()
	lineDirection := imgui.NewVec2(float32(math.Cos(float64(angleRadians))), float32(math.Sin(float64(angleRadians))))
	sizePixels := indicator.Size.ToSize()
	endpointVector := lineDirection.Mul(sizePixels.X / 2)
	lineColor := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(200, 200, 200, 255))

	yOffset := getVerticalOffset(indicator.PitchAngleDegrees)
	position := ui.NewUDim(0.5, 0, 0.5, -yOffset)
	p1 := position.ToPosition().Sub(endpointVector)
	p2 := position.ToPosition().Add(endpointVector)
	drawList.AddLineV(p1, p2, lineColor, sizePixels.Y)

	imgui.End()
}
