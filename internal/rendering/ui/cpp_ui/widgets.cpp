#include "imgui.h"
#include "math.h"
#include "widgets.h"
#include <string>

#define sALT_RANGE 500.0
#define sALT_INTERVAL 20
#define sALT_INTERVAL_MAJOR 10

void rect_meter_dashed(std::string label, MeterDashSide dash_side, float meter_value, float meter_display_range, float value_interval, float major_value_interval) {
    ImGuiWindowFlags flags = ImGuiWindowFlags_NoTitleBar |
                            ImGuiWindowFlags_NoResize;

    ImVec2 window_size = ImVec2(75, 300);
    float half_height = window_size.y / 2;
    float value_per_pixel = meter_display_range / window_size.y;
    
    ImGui::SetNextWindowSize(window_size);
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.5f, 0.5f, 0.5f, 0.6f));
    ImGui::Begin(label.c_str(), nullptr, flags);
    ImVec2 canvas_pos = ImGui::GetWindowPos();
    
    float value_at_top = meter_value + value_per_pixel * half_height;
    float rounded_top_value = floor(value_at_top / value_interval) * value_interval;
    float top_dash_pos_y = canvas_pos.y + abs(rounded_top_value - value_at_top) / value_per_pixel;
    float dash_count = meter_display_range / value_interval;
    float dash_spacing = window_size.y / dash_count;

    ImDrawList* draw_list = ImGui::GetWindowDrawList();
    ImFont* font13 = ImGui::GetIO().Fonts->Fonts[0];
    float font_size = 13.0f;

    for (int x = 0; x < dash_count; x++) {
        ImU32 color = IM_COL32(200, 200, 200, 255);
        float value_here = rounded_top_value - (x * value_interval);
        bool is_major = ((int)value_here % (int)major_value_interval) == 0;

        int startY = top_dash_pos_y + (x * dash_spacing);
        int length = 10;
        int dash_thickness = 1;

        if (is_major) {
            length = 15;
            dash_thickness = 2;
            color = IM_COL32(255, 255, 255, 255);
            std::string text = std::to_string((int)value_here);
            int textWidth = ImGui::CalcTextSize(text.c_str()).x;
            float posX = (dash_side == LEFT) ? canvas_pos.x + length + 5 : canvas_pos.x + window_size.x - length - 5 - textWidth;

            draw_list->AddText(
                font13,
                font_size,
                ImVec2(posX, startY - (font_size) / 2),
                IM_COL32(255, 255, 255, 255),
                text.c_str()
            );
        }

        float posX1 = (dash_side == LEFT) ? canvas_pos.x + 1 : canvas_pos.x + window_size.x - 1;
        float posX2 = (dash_side == LEFT) ? canvas_pos.x + length : canvas_pos.x + window_size.x - length;
        
        draw_list->AddRectFilled(
            ImVec2(posX1, startY),
            ImVec2(posX2, startY + dash_thickness),
            color
        );        
    }

    float triPosX1 = (dash_side == LEFT) ? canvas_pos.x + 12 : canvas_pos.x + window_size.x - 12;
    float triPosX2 = (dash_side == LEFT) ? canvas_pos.x + 18 : canvas_pos.x + window_size.x - 18;
    draw_list->AddTriangleFilled(
        ImVec2(triPosX1, canvas_pos.y + half_height),
        ImVec2(triPosX2, canvas_pos.y + half_height + 10),
        ImVec2(triPosX2, canvas_pos.y + half_height - 10),
        IM_COL32(0, 0, 0, 255)
    );

    float rectPosX1 = (dash_side == LEFT) ? canvas_pos.x + 18 : canvas_pos.x + window_size.x - 18;
    float rectPosX2 = (dash_side == LEFT) ? canvas_pos.x + window_size.x : canvas_pos.x;
    draw_list->AddRectFilled(
        ImVec2(rectPosX1, canvas_pos.y + half_height - 16),
        ImVec2(rectPosX2, canvas_pos.y + half_height + 16),
        IM_COL32(0, 0, 0, 255)
    );

    std::string meterText = std::to_string((int)meter_value);
    int meterTextWidth = ImGui::CalcTextSize(meterText.c_str()).x;
    float textPosX = (dash_side == LEFT) ? canvas_pos.x + 22 : canvas_pos.x + window_size.x - 27 - meterTextWidth;

    draw_list->AddText(
        font13,
        15,
        ImVec2(textPosX, canvas_pos.y + half_height - 15 / 2),
        IM_COL32(255, 255, 255, 255),
        meterText.c_str()
    );

    ImGui::End();
    ImGui::PopStyleColor();

    ImGuiWindowFlags label_window_flags = ImGuiWindowFlags_NoTitleBar | ImGuiWindowFlags_NoResize | ImGuiWindowFlags_NoMove;
    ImVec2 label_window_size = ImVec2(window_size.x, 45);
    ImGui::SetNextWindowSize(label_window_size);
    ImGui::SetNextWindowPos(ImVec2(canvas_pos.x, canvas_pos.y + window_size.y));
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.5f, 0.5f, 0.5f, 0.6f));
    ImGui::Begin((label + "TitleWindow").c_str(), nullptr, label_window_flags);

    ImGui::TextWrapped(label.c_str());

    ImGui::End();
    ImGui::PopStyleColor();
}