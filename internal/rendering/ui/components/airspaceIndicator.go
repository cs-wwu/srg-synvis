package components

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
)

type AirspaceIndicator struct {
	ui.UIComponent
	Position    ui.UDim
	Size        ui.UDim
	AnchorPoint imgui.Vec2
	Airspace    *rendering.Feature
}

func NewAirspaceIndicator() *AirspaceIndicator {
	return &AirspaceIndicator{
		Position:    *ui.NewUDim(0.5, 0, 1, -70),
		Size:        *ui.UDimFromPixels(250, 60),
		AnchorPoint: imgui.NewVec2(0.5, 1),
	}
}

func (a *AirspaceIndicator) Render() {
	if a.Airspace == nil {
		return
	}

	windowSize := a.Size.ToSize()
	text := a.Airspace.Properties.Name

	imgui.SetNextWindowSize(windowSize)
	imgui.SetNextWindowPos(a.Position.ToPosWithAnchor(a.AnchorPoint, &a.Size))
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.8, 0.2, 0.2, 0.7))
	imgui.BeginV("AirspaceIndicator", nil,
		imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove|imgui.WindowFlagsNoScrollbar)

	label := "Inside airspace:"
	labelSize := imgui.CalcTextSize(label)
	textSize := imgui.CalcTextSize(text)

	totalHeight := labelSize.Y + 4 + textSize.Y

	imgui.SetCursorPos(imgui.NewVec2(
		(windowSize.X-labelSize.X)/2,
		(windowSize.Y-totalHeight)/2,
	))
	imgui.Text(label)

	imgui.SetCursorPos(imgui.NewVec2(
		(windowSize.X-textSize.X)/2,
		(windowSize.Y-totalHeight)/2+labelSize.Y+4,
	))
	imgui.Text(text)

	imgui.End()
	imgui.PopStyleColor()
}
