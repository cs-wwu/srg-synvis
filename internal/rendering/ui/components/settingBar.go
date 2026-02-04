package components

import (
	"github.com/cs-wwu/srg-synvis/internal/models"
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
)

type SettingBar struct {
	ui.UIComponent
	AnchorPoint         imgui.Vec2
	Position            ui.UDim
	Size                ui.UDim
	DebugMode      bool
	DisableFog     bool
	UseGPS		   bool
	UseHat		   bool
	RenderDistance float32
	app *rendering.Application
}

func NewSettingBar(position ui.UDim, size ui.UDim) *SettingBar {
	var config models.AppConfig
	return &SettingBar{
		AnchorPoint:         imgui.NewVec2(0, 0),
		Position:            position,
		Size:                size,
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
	imgui.SetNextWindowPos(bar.Position.ToPosWithAnchor(bar.AnchorPoint, &bar.Size))

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
