package renderer

import (
	"image/color"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

type buttonx struct {
	xlow          float64
	xhigh         float64
	ylow          float64
	yhigh         float64
	click         bool
	lastClickTime time.Time
	text          []string
	bgc           []color.RGBA
}

func (app *Application) prepareButtons() {
	buttons := []buttonx{
		{
			xlow:          0.5,
			xhigh:         0.75,
			ylow:          -0.75,
			yhigh:         -0.5,
			click:         false,
			lastClickTime: time.Now(),
			text:          []string{"on", "off"},
			bgc:		   []color.RGBA{{0, 0, 0, 128},{0, 0, 0, 0}},
		},
	}
	app.buttons = buttons
}

// Processes input from the glfw mouse
// updates buttons if over them
func (app *Application) CheckMouse(window *glfw.Window) {
	if window.GetAttrib(glfw.Focused) == glfw.True {
		xpos, ypos := window.GetCursorPos()
		// convert back to vulkan coordinates
		xpos = 2*xpos/float64(app.width) - 1
		ypos = 2*ypos/float64(app.height) - 1

		leftclick := window.GetMouseButton(glfw.MouseButtonLeft)
		currtime := time.Now()
		if leftclick == glfw.Press {
			for i := range app.buttons {
				button := &app.buttons[i]
				if currtime.Sub(button.lastClickTime).Milliseconds() > 200 && OverButton(xpos, ypos, button.xlow, button.xhigh, button.ylow, button.yhigh) {
					// update info
					button.click = !button.click
					button.lastClickTime = currtime
					ind := boolIndex(button.click)
					app.updateText(button.text[ind], 72, 200, 125, 1, button.bgc[ind])
					return
				}
			}
		}
	}
}

func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

func OverButton(mousex, mousey, buttonxlow, buttonxhigh, buttonylow, buttonyhigh float64) bool {
	if buttonxlow < mousex && mousex < buttonxhigh && buttonylow < mousey && mousey < buttonyhigh {
		return true
	} else {
		return false
	}
}
