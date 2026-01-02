package components

import (
	"math"
	"strconv"

	"github.com/cs-wwu/srg-synvis/internal/rendering/ui"
	"github.com/elokore/cimgui-go-vulkan/imgui"
)

type DashSide int

const (
	Left DashSide = iota
	Right
)

type RectangleGauge struct {
	ui.UIComponent
	TestValue     float32
	MeterRange    float32
	Interval      float32
	MajorInterval float32
	DashSide      DashSide
}

func NewRectangleGauge() *RectangleGauge {
	return &RectangleGauge{
		TestValue:     0.0,
		MeterRange:    500,
		Interval:      20,
		MajorInterval: 100,
		DashSide:      Left,
	}
}

// Called every frame
func (gauge *RectangleGauge) Render() {
	gauge.TestValue += 5
	window_size := imgui.Vec2{X: 75, Y: 300}
	half_height := window_size.Y / 2
	value_per_pixel := float32(gauge.MeterRange) / window_size.Y

	imgui.SetNextWindowSize(window_size)
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.5, 0.5, 0.5, 0.6))
	imgui.BeginV("RectangleGauge", nil, imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsNoResize)

	canvas_pos := imgui.WindowPos()
	value_at_top := gauge.TestValue + value_per_pixel*half_height
	rounded_top_value := float32(math.Floor(float64(value_at_top/gauge.Interval))) * gauge.Interval
	top_dash_pos_y := canvas_pos.Y + float32(math.Abs(float64(rounded_top_value-value_at_top)))/value_per_pixel
	dash_count := gauge.MeterRange / gauge.Interval
	dash_spacing := window_size.Y / dash_count

	draw_list := imgui.WindowDrawList()
	font_size := float32(13)

	for i := 0; i < int(dash_count); i++ {
		color := imgui.ColorConvertFloat4ToU32(imgui.NewVec4(200, 200, 200, 255))
		value_here := rounded_top_value - (float32(i) * gauge.Interval)
		is_major := (int32(value_here) % int32(gauge.MajorInterval)) == 0
		startY := top_dash_pos_y + (float32(i) * dash_spacing)
		length := float32(10)
		dash_thickness := 1

		if is_major {
			length = float32(15)
			dash_thickness = 2
			color = imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 255, 255))
			text := strconv.FormatInt(int64(value_here), 10)
			textWidth := imgui.CalcTextSize(text).X
			var posX float32

			if gauge.DashSide == Left {
				posX = canvas_pos.X + length + 5
			} else {
				posX = canvas_pos.X + window_size.X - length - 5 - textWidth
			}

			draw_list.AddTextVec2(
				imgui.NewVec2(posX, startY-float32(font_size)/2),
				imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 255, 255)),
				text,
			)
		}

		var posX1 float32
		var posX2 float32

		if gauge.DashSide == Left {
			posX1 = canvas_pos.X + 1
			posX2 = canvas_pos.X + length
		} else {
			posX1 = canvas_pos.X + window_size.X - 1
			posX2 = canvas_pos.X + window_size.X - length
		}

		draw_list.AddRectFilled(
			imgui.NewVec2(posX1, startY),
			imgui.NewVec2(posX2, startY+float32(dash_thickness)),
			color,
		)
	}

	imgui.End()
	imgui.PopStyleColor()

	var triPosX1 float32
	var triPosX2 float32
	var rectPosX1 float32
	var rectPosX2 float32

	if gauge.DashSide == Left {
		triPosX1 = canvas_pos.X + 12
		triPosX2 = canvas_pos.X + 18

		rectPosX1 = canvas_pos.X + 18
		rectPosX2 = canvas_pos.X + window_size.X
	} else {
		triPosX1 = canvas_pos.X + window_size.X - 12
		triPosX2 = canvas_pos.X + window_size.X - 18

		rectPosX1 = canvas_pos.X + window_size.X - 18
		rectPosX2 = canvas_pos.X
	}

	draw_list.AddTriangleFilled(
		imgui.NewVec2(triPosX1, canvas_pos.Y+half_height),
		imgui.NewVec2(triPosX2, canvas_pos.Y+half_height+10),
		imgui.NewVec2(triPosX2, canvas_pos.Y+half_height-10),
		imgui.ColorConvertFloat4ToU32(imgui.NewVec4(0, 0, 0, 255)),
	)

	draw_list.AddRectFilled(
		imgui.NewVec2(rectPosX1, canvas_pos.Y+half_height-16),
		imgui.NewVec2(rectPosX2, canvas_pos.Y+half_height+16),
		imgui.ColorConvertFloat4ToU32(imgui.NewVec4(0, 0, 0, 255)),
	)

	meterText := strconv.FormatInt(int64(gauge.TestValue), 10)
	meterTextWidth := imgui.CalcTextSize(meterText).X
	var textPosX float32

	if gauge.DashSide == Left {
		textPosX = canvas_pos.X + 22
	} else {
		textPosX = canvas_pos.X + window_size.X - 27 - meterTextWidth
	}

	currentFont := draw_list.Data().Font()
	draw_list.AddTextFontPtr(
		currentFont,
		15,
		imgui.NewVec2(textPosX, canvas_pos.Y+half_height-15/2),
		imgui.ColorConvertFloat4ToU32(imgui.NewVec4(255, 255, 255, 255)),
		meterText,
	)

	label_window_flags := imgui.WindowFlagsNoTitleBar | imgui.WindowFlagsNoResize | imgui.WindowFlagsNoMove
	label_window_size := imgui.NewVec2(window_size.X, 45)

	imgui.SetNextWindowSize(label_window_size)
	imgui.SetNextWindowPos(imgui.NewVec2(canvas_pos.X, canvas_pos.Y+window_size.Y))
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.5, 0.5, 0.5, 0.6))

	imgui.BeginV("TitleWindow", nil, label_window_flags)
	imgui.TextWrapped("Altitude (Ft)")
	imgui.End()
	imgui.PopStyleColor()

	// ImVec2 label_window_size = ImVec2(window_size.x, 45);
	// ImGui::SetNextWindowSize(label_window_size);
	// ImGui::SetNextWindowPos(ImVec2(canvas_pos.x, canvas_pos.y + window_size.y));
	// ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.5f, 0.5f, 0.5f, 0.6f));
	// ImGui::Begin((label + "TitleWindow").c_str(), nullptr, label_window_flags);

	// ImGui::TextWrapped(label.c_str());

	// ImGui::End();
	// ImGui::PopStyleColor();
}
