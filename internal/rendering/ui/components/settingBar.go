package components

import (
	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
)

// TODO what other values do we want?
type SettingBar struct {
	ui.UIComponent
	DebugMode      bool
	DisableFog     bool
	RenderDistance float32
}

func NewSettingBar() *SettingBar {
	var config models.AppConfig
	return &SettingBar{
		DebugMode:      config.DebugMode,
		DisableFog:     config.DisableFog,
		RenderDistance: config.RenderDistance,
	}
}

// Called every frame
func (bar *SettingBar) Render() {
	// Base positioning
	canvasPos := imgui.WindowPos()
	// TODO change hard-coded values: trying to get on top left of screen
	imgui.SetNextWindowPos(imgui.NewVec2(
		canvasPos.X+500,
		canvasPos.Y-50,
	))

	// Open state: settings panel dropdown
	imgui.SetNextWindowSize(imgui.NewVec2(180, 120))
	imgui.PushStyleColorVec4(
		imgui.ColWindowBg,
		imgui.NewVec4(0.1, 0.1, 0.1, 0.9),
	)
	imgui.BeginV(
		"SettingsPanel",
		nil,
		imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove,
	)

	imgui.Separator()

	// Setting Options
	if imgui.Checkbox("Debug Mode", &bar.DebugMode) {
		// possibility: react immediately when debug mode changes
	}
	imgui.Checkbox("Disable Fog", &bar.DisableFog)
	imgui.InputFloat("Render Distance", &bar.RenderDistance)

	// Text indicator
	// if bar.DebugMode {
	// 	imgui.TextColored(imgui.NewVec4(0, 1, 0, 1), "Debug: ON")
	// } else {
	// 	imgui.TextColored(imgui.NewVec4(1, 0, 0, 1), "Debug: OFF")
	// }

	imgui.End()
	imgui.PopStyleColor()
}
