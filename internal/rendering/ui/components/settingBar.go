package components

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
)

type SettingBar struct {
	ui.UIComponent
	Open      bool
	DebugMode bool
}

func NewSettingBar() *SettingBar {
	return &SettingBar{
		Open:      false,
		DebugMode: false,
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

	// Closed state: small settings button
	if !bar.Open {
		imgui.SetNextWindowSize(imgui.NewVec2(100, 40))

		imgui.BeginV(
			"SettingsButton",
			nil,
			imgui.WindowFlagsNoTitleBar|
				imgui.WindowFlagsNoResize|
				imgui.WindowFlagsNoMove|
				imgui.WindowFlagsNoScrollbar,
		)

		if imgui.Button("Settings") {
			bar.Open = true
		}

		imgui.End()
		return
	}

	// Open state: settings panel
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

	// Header row
	imgui.Text("Settings")
	imgui.SameLine()
	if imgui.SmallButton("X") {
		bar.Open = false
	}

	imgui.Separator()

	// Debug mode toggle
	if imgui.Checkbox("Debug Mode", &bar.DebugMode) {
		// Optional: react immediately when debug mode changes
	}

	// Visual indicator
	if bar.DebugMode {
		imgui.TextColored(imgui.NewVec4(0, 1, 0, 1), "Debug: ON")
	} else {
		imgui.TextColored(imgui.NewVec4(1, 0, 0, 1), "Debug: OFF")
	}

	imgui.End()
	imgui.PopStyleColor()
}
