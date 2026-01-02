package menus

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/cs-wwu/srg-synvis/internal/rendering/ui/components"
)

type TestWindow struct {
	ui.UIComponent
	Gauge *components.RectangleGauge
}

func NewTestWindow() *TestWindow {
	return &TestWindow{
		Gauge: components.NewRectangleGauge(),
	}
}

func (window *TestWindow) Render() {
	window.Gauge.Render()
}

func init() {
	window := NewTestWindow()
	ui.RegisterUIComponent(window)
}
