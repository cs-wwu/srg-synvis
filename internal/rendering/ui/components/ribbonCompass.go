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

/*
void ribbon_compass(std::string label, ImVec2 pos, float meter_value, float meter_display_range, float value_interval, float major_value_interval) {
    ImGuiWindowFlags flags = ImGuiWindowFlags_NoTitleBar | ImGuiWindowFlags_NoResize;

    ImVec2 window_size = ImVec2(300, 45);
    float half_width = window_size.x / 2;
    float value_per_pixel = meter_display_range / window_size.x;

    ImGui::SetNextWindowPos(pos);
    ImGui::SetNextWindowSize(window_size);
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.5f, 0.5f, 0.5f, 0.6f));
    ImGui::Begin(label.c_str(), nullptr, flags);
    ImVec2 canvas_pos = ImGui::GetWindowPos();

    float value_at_left = map_to_360_range(meter_value - value_per_pixel * half_width);
    float rounded_left_value = map_to_360_range(ceil(value_at_left / value_interval) * value_interval);
    float left_dash_pos_x = canvas_pos.x + map_to_360_range(rounded_left_value - value_at_left) / value_per_pixel;
    float dash_count = meter_display_range / value_interval;
    float dash_spacing = window_size.x / dash_count;

    ImDrawList* draw_list = ImGui::GetWindowDrawList();
    ImFont* font13 = ImGui::GetIO().Fonts->Fonts[0];
    float font_size = 13.0f;

    for (int x = 0; x < dash_count; x++) {
        ImU32 color = IM_COL32(200, 200, 200, 255);
        float valueHere = map_to_360_range(rounded_left_value + (x * value_interval));
        bool is_major = ((int)valueHere % (int)major_value_interval) == 0;

        int startX = left_dash_pos_x + (x * dash_spacing);
        int length = 10;
        int dash_thickness = 1;
        font_size = 13;

        if (is_major) {
            length = 15;
            dash_thickness = 2;
            color = IM_COL32(255, 255, 255, 255);
            ImU32 textColor = IM_COL32(255, 255, 255, 255);
            std::string text = std::to_string((int)valueHere);

            if (valueHere == 0.0 || valueHere == 360.0) {
                text = "N";
                font_size = 16;
                textColor = IM_COL32(255, 0, 0, 255);
            } else if (valueHere == 90.0) {
                text = "E";
                font_size = 16;
            } else if (valueHere == 180.0) {
                text = "S";
                font_size = 16;
            } else if (valueHere == 270.0) {
                text = "W";
                font_size = 16;
            }

            ImVec2 textBounds = ImGui::CalcTextSize(text.c_str());
            float posX = startX + dash_thickness - textBounds.x / 2;
            float posY = canvas_pos.y + window_size.y - length - font_size - 5;

            draw_list->AddText(
                font13,
                font_size,
                ImVec2(posX, posY),
                textColor,
                text.c_str()
            );
        }

        float posY1 = canvas_pos.y + window_size.y;
        float posY2 = posY1 - length;

        draw_list->AddRectFilled(
            ImVec2(startX, posY1),
            ImVec2(startX + dash_thickness, posY2),
            color
        );
    }

    ImGui::End();
    ImGui::PopStyleColor();

    ImGuiWindowFlags second_flags = ImGuiWindowFlags_NoTitleBar
        | ImGuiWindowFlags_NoResize | ImGuiWindowFlags_NoMove | ImGuiWindowFlags_NoBackground;
    ImVec2 second_window_size = ImVec2(window_size.x, 40);
    ImVec2 second_window_pos = canvas_pos + ImVec2(0, window_size.y);
    ImGui::SetNextWindowSize(second_window_size);
    ImGui::SetNextWindowPos(second_window_pos);
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.0f, 0.0f, 0.0f, 0.0f));
    ImGui::Begin((label + "HeadingReader").c_str(), nullptr, second_flags);
    draw_list = ImGui::GetWindowDrawList();

    ImVec2 p1 = second_window_pos + ImVec2(second_window_size.x / 2, 0);
    draw_list->AddTriangleFilled(
        p1,
        p1 + ImVec2(10, 10),
        p1 + ImVec2(-10, 10),
        IM_COL32(255, 255, 255, 255)
    );

    ImVec2 c1 = p1 + ImVec2(-40, 10);
    ImVec2 c2 = p1 + ImVec2(40, 35);
    draw_list->AddRectFilled(
        c1,
        c2,
        IM_COL32(0, 0, 0, 255),
        5.0f
    );

    char heading_text[8];
    snprintf(heading_text, sizeof(heading_text), "%.1f°\n", meter_value);
    ImVec2 heading_text_size = ImGui::CalcTextSize(heading_text);
    ImVec2 heading_text_pos = (c1 + c2) / 2 - (heading_text_size / 2);

    draw_list->AddText(heading_text_pos, IM_COL32(255, 255, 255, 255), heading_text);

    ImGui::End();
    ImGui::PopStyleColor();
}
*/
