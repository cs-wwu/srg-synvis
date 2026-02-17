package menus

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui/components"
)

type FlightInstruments struct {
	ui.UIComponent
	Gauge *components.RectangleGauge
}

func NewFlightInstruments() *FlightInstruments {
	return &FlightInstruments{
		Gauge: components.NewRectangleGauge(),
	}
}

func (window *FlightInstruments) Render() {
	window.Gauge.Render()
}

func init() {
	window := NewFlightInstruments()
	ui.RegisterUIComponent(window)
}
