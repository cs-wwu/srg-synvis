package components

import (
	"math"

	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
	"github.com/xlab/linmath"
)

type AttitudeDial struct {
	ui.UIComponent
	AttitudeAngleDegrees float32
}

func NewAttitudeDial() *AttitudeDial {
	return &AttitudeDial{
		AttitudeAngleDegrees: 0.0,
	}
}

func (indicator *AttitudeDial) Render() {
	windowFlags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoBackground
	windowSize := ui.UDimFromScale(1, 1).ToSize()

	imgui.SetNextWindowSize(windowSize)
	imgui.SetNextWindowPos(ui.UDimFromScale(0, 0).ToPosition())
	imgui.BeginV("AttitudeDial", nil, windowFlags)

	angleRadians := toRadians(indicator.AttitudeAngleDegrees)
	drawList := imgui.WindowDrawList()
	lineColor := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(200, 200, 200, 255))

	position := ui.NewUDim(0.5, 0, 0.5, 50)
	startAngleRad := linmath.DegreesToRadians(40) - angleRadians
	angleInterval := linmath.DegreesToRadians(100 / 11)
	arcRadius := 150

	for i := 0; i < 12; i++ {
		dashAngle := startAngleRad + angleInterval*float32(i)
		dashDirection := imgui.NewVec2(float32(math.Cos(float64(dashAngle))), -float32(math.Sin(float64(dashAngle))))
		dashP1 := position.ToPosition().Add(dashDirection.Mul(float32(arcRadius)))
		dashP2 := dashP1.Add(dashDirection.Mul(6))
		drawList.AddLineV(dashP1, dashP2, lineColor, 2)
	}

	drawList.PathArcTo(position.ToPosition(), float32(arcRadius), linmath.DegreesToRadians(-40)+angleRadians, linmath.DegreesToRadians(-140)+angleRadians)
	drawList.PathStrokeV(lineColor, imgui.DrawFlagsNone, 2)

	// Calculate Center triangle pointer
	centerAngle := startAngleRad + linmath.DegreesToRadians(50)
	dirToCenter := imgui.NewVec2(float32(math.Cos(float64(centerAngle))), -float32(math.Sin(float64(centerAngle))))
	perpendicularCenterDir := imgui.NewVec2(-dirToCenter.Y, dirToCenter.X)

	centerTriHeight := 10
	centerTriWidth := 14
	centerTriP1 := position.ToPosition().Add(dirToCenter.Mul(float32(arcRadius - 2)))
	centerTriBottom := centerTriP1.Sub(dirToCenter.Mul(float32(centerTriHeight)))
	centerTriP2 := centerTriBottom.Add(perpendicularCenterDir.Mul(float32(centerTriWidth) / 2))
	centerTriP3 := centerTriBottom.Sub(perpendicularCenterDir.Mul(float32(centerTriWidth) / 2))

	// Calculate Top triangle pointer
	pointingAngle := centerAngle + angleRadians
	pointingDir := imgui.NewVec2(float32(math.Cos(float64(pointingAngle))), -float32(math.Sin(float64(pointingAngle))))
	perpendicularPointingDir := imgui.NewVec2(-pointingDir.Y, pointingDir.X)
	pointingTriHeight := 14
	pointingTriWidth := 10

	pointingTriP1 := position.ToPosition().Add(pointingDir.Mul(float32(arcRadius + 2)))
	pointingTriBottom := pointingTriP1.Add(pointingDir.Mul(float32(pointingTriHeight)))
	pointingTriP2 := pointingTriBottom.Add(perpendicularPointingDir.Mul(float32(pointingTriWidth) / 2))
	pointingTriP3 := pointingTriBottom.Sub(perpendicularPointingDir.Mul(float32(pointingTriWidth) / 2))

	drawList.AddTriangleFilled(pointingTriP1, pointingTriP2, pointingTriP3, lineColor)
	drawList.AddTriangleFilled(centerTriP1, centerTriP2, centerTriP3, lineColor)

	imgui.End()
}
