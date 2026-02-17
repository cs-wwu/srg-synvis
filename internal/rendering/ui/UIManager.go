package ui

type UIComponent interface {
	Render()
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
