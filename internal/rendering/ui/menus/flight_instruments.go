package menus

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui/components"
)

type FlightInstruments struct {
	ui.UIComponent
	Gauge *components.RectangleGauge
	Bar *components.SettingBar
}

func NewFlightInstruments() *FlightInstruments {
	return &FlightInstruments{
		Gauge: components.NewRectangleGauge(),
		Bar: components.NewSettingBar(),
	}
}

func (window *FlightInstruments) Render() {
	window.Gauge.Render()
	window.Bar.Render()
}

func (window *FlightInstruments) UseApp(app *rendering.Application) {
	window.Bar.UpdateSettingBar(app)
}

func init() {
	window := NewFlightInstruments()
	ui.RegisterUIComponent(window)
}
