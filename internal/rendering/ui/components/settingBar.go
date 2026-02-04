package components

import (
	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
)

type SettingBar struct {
	ui.UIComponent
	DebugMode      bool
	DisableFog     bool
	UseGPS		   bool
	UseHat		   bool
	RenderDistance float32
	app *rendering.Application
}

func NewSettingBar() *SettingBar {
	var config models.AppConfig
	return &SettingBar{
		DebugMode:      config.DebugMode,
		DisableFog:     config.DisableFog,
		RenderDistance: config.RenderDistance,
		UseGPS:			config.UseGPS,
		UseHat:			config.UseHat,
		app: nil,
	}
}

func (bar *SettingBar) UpdateSettingBar(app *rendering.Application) {
	bar.app = app
	bar.DebugMode = app.Cfg.DebugMode
	bar.DisableFog = app.Cfg.DisableFog
	bar.RenderDistance = app.Cfg.RenderDistance
	bar.UseGPS = app.Cfg.UseHat
	bar.UseHat = app.Cfg.UseHat
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
		bar.app.Cfg.DebugMode = bar.DebugMode
	}
	if imgui.Checkbox("Disable Fog", &bar.DisableFog) {
		bar.app.Cfg.DisableFog = bar.DisableFog
		// todo this needs to update app.ubo
	}
	imgui.Checkbox("Use GPS", &bar.UseGPS)
	imgui.Checkbox("Use Hat", &bar.UseHat)
	if imgui.InputFloat("Render Distance", &bar.RenderDistance) {
		bar.app.Cfg.RenderDistance = bar.RenderDistance
	}

	// Text indicator
	// if bar.DebugMode {
	// 	imgui.TextColored(imgui.NewVec4(0, 1, 0, 1), "Debug: ON")
	// } else {
	// 	imgui.TextColored(imgui.NewVec4(1, 0, 0, 1), "Debug: OFF")
	// }

	imgui.End()
	imgui.PopStyleColor()
}
