package ui

import (
	"github.com/cs-wwu/srg-synvis/internal/rendering"
)


type UIComponent interface {
	Render()
	UseApp(app *rendering.Application)
}

var components []UIComponent

func RegisterUIComponent(component UIComponent) {
	components = append(components, component)
}

func RenderUI() {
	for _, component := range components {
		component.Render()
	}
}

func DeliverApp(app *rendering.Application) {
	for _, component := range components {
		component.UseApp(app)
	}	
}
