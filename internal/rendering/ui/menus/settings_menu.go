package menus

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui/components"
)

type SettingsMenu struct {
	ui.UIComponent
	Bar *components.SettingBar
}

func NewSettingsMenu() *SettingsMenu {
	return &SettingsMenu{
		Bar: components.NewSettingBar(),
	}
}

func (window *SettingsMenu) Render() {
	window.Bar.Render()
}

func (window *SettingsMenu) UseApp(app *rendering.Application) {
	window.Bar.UpdateSettingBar(app)
}

func init() {
	window := NewSettingsMenu()
	ui.RegisterUIComponent(window)
}
