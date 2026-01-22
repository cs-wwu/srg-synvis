package menus

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui/components"
)

type FlightInstruments struct {
	ui.UIComponent
	Gauge   *components.RectangleGauge
	Compass *components.RibbonCompass
	app     *rendering.Application
}

func NewFlightInstruments() *FlightInstruments {
	return &FlightInstruments{
		Gauge:   components.NewRectangleGauge("Altitude (m)"),
		Compass: components.NewRibbonCompass(),
	}
}

func (window *FlightInstruments) Render() {
	window.Gauge.Value = window.app.GetCamera().Position()[1]
	window.Compass.HeadingDegrees += 0.1
	window.Gauge.Render()
	window.Compass.Render()
}

func (window *FlightInstruments) UseApp(app *rendering.Application) {
	window.app = app
}

func init() {
	window := NewFlightInstruments()
	ui.RegisterUIComponent(window)
}
