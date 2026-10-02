package components

import (
	"math"
	"strconv"

	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
	"github.com/xlab/linmath"
)

type PitchIndicator struct {
	ui.UIComponent
	AttitudeAngleDegrees float32
	PitchAngleDegrees    float32
	IntervalPixelSpacing float32
	PitchInterval        float32
}

func NewPitchIndicator() *PitchIndicator {
	return &PitchIndicator{
		AttitudeAngleDegrees: 0.0,
		PitchAngleDegrees:    0.0,
		IntervalPixelSpacing: 80.0,
		PitchInterval:        5.0,
	}
}

func (indicator *PitchIndicator) Render() {
	lineColor := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(200, 200, 200, 255))
	windowFlags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoBackground
	windowSize := ui.UDimFromPixels(300, 300)
	center := ui.NewUDim(0.5, 0, 0.5, 0)

	imgui.SetNextWindowSize(windowSize.ToSize())
	imgui.SetNextWindowPos(center.ToPosWithAnchor(imgui.NewVec2(0.5, 0.5), windowSize))
	imgui.BeginV("PitchIndicator", nil, windowFlags)

	attitudeRadians := linmath.DegreesToRadians(-indicator.AttitudeAngleDegrees + 90.0)
	upDir := imgui.NewVec2(float32(math.Cos(float64(attitudeRadians))), -float32(math.Sin(float64(attitudeRadians))))
	lineDir := imgui.NewVec2(-upDir.Y, upDir.X)
	drawList := imgui.WindowDrawList()

	pixelsPerDegree := indicator.IntervalPixelSpacing / indicator.PitchInterval
	iterations := 360.0 / indicator.PitchInterval

	for i := iterations; i > -iterations; i-- {
		barAngle := float32(i) * (indicator.PitchInterval / 2)
		lineLength := 120.0
		lineThickness := 2

		isMajor := math.Mod(float64(i), 2.0) == 0
		if !isMajor {
			lineLength = 80.0
			lineThickness = 1
		}

		yPos := (barAngle - indicator.PitchAngleDegrees) * pixelsPerDegree

		if math.Abs(float64(yPos)) > 100 {
			continue
		}
		lineCenter := center.ToPosition().Add(upDir.Mul(yPos))

		p1 := lineCenter.Add(lineDir.Mul(float32(lineLength / 2)))
		p2 := lineCenter.Sub(lineDir.Mul(float32(lineLength / 2)))

		currentFont := drawList.Data().Font()

		if isMajor {
			drawList.AddTextFontPtr(
				currentFont,
				15,
				lineCenter.Add(lineDir.Mul(float32(lineLength)*0.75)).Add(upDir.Mul(7)),
				imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 255, 255)),
				strconv.FormatFloat(float64(barAngle), 'f', 0, 64),
			)

			drawList.AddTextFontPtr(
				currentFont,
				15,
				lineCenter.Sub(lineDir.Mul(float32(lineLength)*0.9)).Add(upDir.Mul(7)),
				imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 255, 255)),
				strconv.FormatFloat(float64(barAngle), 'f', 0, 64),
			)
		}

		drawList.AddLineV(p1, p2, lineColor, float32(lineThickness))
	}

	tri1P1 := center.ToPosition().Add(lineDir.Mul(65))
	tri1P2 := tri1P1.Add(upDir.Mul(10)).Add(lineDir.Mul(10))
	tri1P3 := tri1P2.Sub(upDir.Mul(20))

	tri2P1 := center.ToPosition().Sub(lineDir.Mul(65))
	tri2P2 := tri2P1.Add(upDir.Mul(10)).Sub(lineDir.Mul(10))
	tri2P3 := tri2P2.Sub(upDir.Mul(20))

	drawList.AddTriangleFilled(tri1P1, tri1P2, tri1P3, imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 0, 255)))
	drawList.AddTriangleFilled(tri2P1, tri2P2, tri2P3, imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 0, 255)))

	imgui.End()

}
