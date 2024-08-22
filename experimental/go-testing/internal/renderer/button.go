package renderer

import (
	// "fmt"
	"log"
	"time"
	// "math"

	"github.com/go-gl/glfw/v3.3/glfw"
	lin "github.com/xlab/linmath"
)

func (app *Application) initUIDimensions() {
	vertices := []UIVertex{
		// fps box
		{Position: lin.Vec2{-1.0, -1.0}, TexCoord: lin.Vec2{0, 0}},
		{Position: lin.Vec2{0.0, -1.0}, TexCoord: lin.Vec2{1, 0}},
		{Position: lin.Vec2{0.0, 0.0}, TexCoord: lin.Vec2{1, 1}},
		{Position: lin.Vec2{-1.0, 0.0}, TexCoord: lin.Vec2{0, 1}},

		// button
		{Position: lin.Vec2{0.5, -0.75}, TexCoord: lin.Vec2{0, 0}},
		{Position: lin.Vec2{0.75, -0.75}, TexCoord: lin.Vec2{1, 0}},
		{Position: lin.Vec2{0.75, -0.5}, TexCoord: lin.Vec2{1, 1}},
		{Position: lin.Vec2{0.5, -0.5}, TexCoord: lin.Vec2{0, 1}},
	}
	app.vertices = vertices

	app.indices = []Index{
		// Front face (CLOCKWISE)
		0, 1, 2, 2, 3, 0, // fps box
		4, 5, 6, 6, 7, 4, // button
	}
}

// Processes input from the glfw mouse
func (app *Application) CheckMouse(window *glfw.Window) {
	if window.GetAttrib(glfw.Focused) == glfw.True {
		xpos, ypos := window.GetCursorPos()
		// convert back to vulkan coordinates
		xpos = 2*xpos/float64(app.width) - 1
		ypos = 2*ypos/float64(app.height) - 1

		leftclick := window.GetMouseButton(glfw.MouseButtonLeft)
		currtime := time.Now()
		if leftclick == glfw.Press && (currtime.Sub(app.lastClickTime).Milliseconds() > 200) {
			app.click = !app.click
			app.lastClickTime = currtime
			if OverButton1(xpos, ypos, 0.5, 0.75, -0.75, 0.5) {
				log.Printf("clicked button")
			} // todo hardcoded - get vals from app.vertices
		}
	}
}

func OverButton1(mousex, mousey, buttonxlow, buttonxhigh, buttonylow, buttonyhigh float64) bool {
	if buttonxlow < mousex && mousex < buttonxhigh && buttonylow < mousey && mousey < buttonyhigh {
		return true
	} else {
		return false
	}
}
