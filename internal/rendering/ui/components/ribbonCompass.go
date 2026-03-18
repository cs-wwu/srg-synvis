package components

import (
	"fmt"
	"math"
	"strconv"

	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
)

type RibbonCompass struct {
	ui.UIComponent
	AnchorPoint         imgui.Vec2
	Position            ui.UDim
	Size                ui.UDim
	HeadingDegrees      float32
	DegreeInterval      float32
	DegreeMajorInterval float32
	DisplayRange        float32
}

func NewRibbonCompass(position ui.UDim, size ui.UDim) *RibbonCompass {
	return &RibbonCompass{
		AnchorPoint:         imgui.NewVec2(0, 0),
		Position:            position,
		Size:                size,
		HeadingDegrees:      0.0,
		DegreeInterval:      5,
		DegreeMajorInterval: 10,
		DisplayRange:        100,
	}
}

func map_to_360_range(x float32) float32 {
	if x < 0 {
		return float32(math.Mod(360+float64(x), 360))
	} else {
		return float32(math.Mod(float64(x), 360))
	}
}

func (compass *RibbonCompass) Render() {
	windowFlags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize
	windowSize := compass.Size.ToSize()
	var halfWidth float32 = windowSize.X / 2
	var degreesPerPixel float32 = compass.DisplayRange / windowSize.X

	imgui.SetNextWindowPos(compass.Position.ToPosWithAnchor(compass.AnchorPoint, &compass.Size))
	imgui.SetNextWindowSize(windowSize)
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.5, 0.5, 0.5, 0.6))
	imgui.BeginV("RibbonCompass", nil, windowFlags)
	canvasPos := imgui.WindowPos()

	value_at_left := map_to_360_range(compass.HeadingDegrees - degreesPerPixel*halfWidth)
	rounded_left_value := map_to_360_range(float32(math.Ceil(float64(value_at_left/compass.DegreeInterval))) * compass.DegreeInterval)
	left_dash_pos_x := canvasPos.X + map_to_360_range(rounded_left_value-value_at_left)/degreesPerPixel
	dash_count := compass.DisplayRange / compass.DegreeInterval
	dash_spacing := windowSize.X / dash_count

	drawList := imgui.WindowDrawList()
	color := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(200, 200, 200, 255))
	majorColor := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 255, 255))
	majorTextColor := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 255, 255))
	redColor := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 0, 0, 255))
	currentFont := drawList.Data().Font()

	for i := 0; i < int(dash_count); i++ {
		var valueHere float32 = map_to_360_range(rounded_left_value + (float32(i) * compass.DegreeInterval))
		isMajor := (int(valueHere) % int(compass.DegreeMajorInterval)) == 0

		var startX int = int(left_dash_pos_x) + (i * int(dash_spacing))
		var length int = 10
		var dash_thickness int = 1
		var font_size int = 13

		if isMajor {
			length = 15
			dash_thickness = 2
			color = majorColor
			var text string = strconv.FormatInt(int64(valueHere), 10)
			textColor := majorTextColor

			if valueHere == 0.0 || valueHere == 360.0 {
				text = "N"
				font_size = 16
				textColor = redColor
			} else if valueHere == 90.0 {
				text = "E"
				font_size = 16
			} else if valueHere == 180.0 {
				text = "S"
				font_size = 16
			} else if valueHere == 270.0 {
				text = "W"
				font_size = 16
			}

			var textBounds imgui.Vec2 = imgui.CalcTextSize(text)
			var posX float32 = float32(startX) + float32(dash_thickness) - textBounds.X/2
			var posY float32 = canvasPos.Y + windowSize.Y - float32(length) - float32(font_size) - 5

			drawList.AddTextFontPtr(
				currentFont,
				float32(font_size),
				imgui.NewVec2(posX, posY),
				textColor,
				text,
			)
		}

		var posY1 float32 = canvasPos.Y + windowSize.Y
		var posY2 float32 = posY1 - float32(length)

		drawList.AddRectFilled(
			imgui.NewVec2(float32(startX), posY1),
			imgui.NewVec2(float32(startX+dash_thickness), posY2),
			color,
		)
	}

	imgui.End()
	imgui.PopStyleColor()

	second_flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoBackground
	second_window_size := imgui.NewVec2(windowSize.X, 40)
	second_window_pos := canvasPos.Add(imgui.NewVec2(0, windowSize.Y))

	imgui.SetNextWindowSize(second_window_size)
	imgui.SetNextWindowPos(second_window_pos)
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.0, 0.0, 0.0, 0.0))
	imgui.BeginV("CompassHeadingReader", nil, second_flags)
	drawList = imgui.WindowDrawList()

	p1 := second_window_pos.Add(imgui.NewVec2(second_window_size.X/2, 0))
	drawList.AddTriangleFilled(
		p1,
		p1.Add(imgui.NewVec2(10, 10)),
		p1.Add(imgui.NewVec2(-10, 10)),
		majorColor,
	)

	c1 := p1.Add(imgui.NewVec2(-40, 10))
	c2 := p1.Add(imgui.NewVec2(40, 35))

	drawList.AddRectFilledV(
		c1,
		c2,
		imgui.ColorConvertFloat4ToU32(imgui.NewVec4(0, 0, 0, 255)),
		5,
		imgui.DrawFlagsNone,
	)

	var headingText string = fmt.Sprintf("%.1f°\n", map_to_360_range(compass.HeadingDegrees))
	heading_text_size := imgui.CalcTextSize(headingText)
	heading_text_pos := c1.Add(c2).Div(2).Sub(heading_text_size.Div(2))

	drawList.AddTextVec2(
		heading_text_pos,
		majorColor,
		headingText,
	)

	imgui.End()
	imgui.PopStyleColor()
}
