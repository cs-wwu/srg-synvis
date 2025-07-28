#include "imgui.h"
#include "math.h"
#include <string>

#define ALT_RANGE 500.0
#define ALT_INTERVAL 20
#define ALT_INTERVAL_MAJOR 100

void altimeter(std::string label, float* altitudePtr) {
    ImGuiWindowFlags flags = ImGuiWindowFlags_NoTitleBar |
                            ImGuiWindowFlags_NoResize;

    ImVec2 window_size = ImVec2(75, 300);
    float half_height = window_size.y / 2;
    float altitude_per_pixel = ALT_RANGE / window_size.y;
    
    ImGui::SetNextWindowSize(window_size);
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.5f, 0.5f, 0.5f, 0.6f));
    ImGui::Begin("Altimeter", nullptr, flags);
    ImVec2 canvas_pos = ImGui::GetWindowPos();
    
    float altitude = *altitudePtr;
    float altitude_at_top = altitude + altitude_per_pixel * half_height;
    float rounded_top_altitude = floor(altitude_at_top / ALT_INTERVAL) * ALT_INTERVAL;
    float top_dash_pos_y = canvas_pos.y + abs(rounded_top_altitude - altitude_at_top) / altitude_per_pixel;
    float dash_count = ALT_RANGE / ALT_INTERVAL;
    float dash_spacing = window_size.y / dash_count;

    ImDrawList* draw_list = ImGui::GetWindowDrawList();
    ImFont* font13 = ImGui::GetIO().Fonts->Fonts[0];
    float font_size = 13.0f;

    for (int x = 0; x < dash_count; x++) {
        ImU32 color = IM_COL32(200, 200, 200, 255);
        float altitude_here = rounded_top_altitude - (x * ALT_INTERVAL);
        bool is_major = ((int)altitude_here % ALT_INTERVAL_MAJOR) == 0;

        int startY = top_dash_pos_y + (x * dash_spacing);
        int length = 10;
        int dash_thickness = 1;

        if (is_major) {
            length = 15;
            dash_thickness = 2;
            color = IM_COL32(255, 255, 255, 255);

            draw_list->AddText(
                font13,
                font_size,
                ImVec2(canvas_pos.x + length + 5, startY - (font_size) / 2),
                IM_COL32(255, 255, 255, 255),
                std::to_string((int)altitude_here).c_str()
            );
        }

        draw_list->AddRectFilled(
            ImVec2(canvas_pos.x + 1, startY),
            ImVec2(canvas_pos.x + length, startY + dash_thickness),
            color
        );        
    }

    draw_list->AddTriangleFilled(
        ImVec2(canvas_pos.x + 12, canvas_pos.y + half_height),
        ImVec2(canvas_pos.x + 18, canvas_pos.y + half_height + 10),
        ImVec2(canvas_pos.x + 18, canvas_pos.y + half_height - 10),
        IM_COL32(0, 0, 0, 255)
    );

    draw_list->AddRectFilled(
        ImVec2(canvas_pos.x + 18, canvas_pos.y + half_height - 16),
        ImVec2(canvas_pos.x + 75, canvas_pos.y + half_height + 16),
        IM_COL32(0, 0, 0, 255)
    );

    draw_list->AddText(
        font13,
        15,
        ImVec2(canvas_pos.x + 22, canvas_pos.y + half_height - 15 / 2),
        IM_COL32(255, 255, 255, 255),
        std::to_string((int)altitude).c_str()
    );

    ImGui::End();
    ImGui::PopStyleColor();

    ImGuiWindowFlags label_window_flags = ImGuiWindowFlags_NoTitleBar | ImGuiWindowFlags_NoResize | ImGuiWindowFlags_NoMove;
    ImVec2 label_window_size = ImVec2(window_size.x, 45);
    ImGui::SetNextWindowSize(label_window_size);
    ImGui::SetNextWindowPos(ImVec2(canvas_pos.x, canvas_pos.y + window_size.y));
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.5f, 0.5f, 0.5f, 0.6f));
    ImGui::Begin("AltimeterTitleWindow", nullptr, label_window_flags);

    ImGui::TextWrapped(label.c_str());

    ImGui::End();
    ImGui::PopStyleColor();
}