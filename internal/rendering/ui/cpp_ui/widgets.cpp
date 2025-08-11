#define IMGUI_DEFINE_MATH_OPERATORS
#include "imgui.h"
#include "math.h"
#include "widgets.h"
#include <string>
#include "imgui_internal.h"

//internal bounding boxes
// BB = outermost 'true' bounding box (including bounding box for handles)
// DR = bounding box for drawing
// IN = inner (ie inside bufferzone for boundaries handles
// CE = centron (prob central 10%)
// TL...B = bb for handles for grabbing the various edges/corners
enum bbEnum { BB_BB, BB_DR, BB_IN, BB_CE, BB_TL, BB_TR, BB_BL, BB_BR, BB_L, BB_R, BB_T, BB_B };

int rotation_start_index;
void ImRotateStart()
{
    rotation_start_index = ImGui::GetWindowDrawList()->VtxBuffer.Size;
}

ImVec2 ImRotationCenter()
{
    ImVec2 l(FLT_MAX, FLT_MAX), u(-FLT_MAX, -FLT_MAX); // bounds

    const auto& buf = ImGui::GetWindowDrawList()->VtxBuffer;
    for (int i = rotation_start_index; i < buf.Size; i++)
        l = ImMin(l, buf[i].pos), u = ImMax(u, buf[i].pos);

    return ImVec2((l.x + u.x) / 2, (l.y + u.y) / 2); // or use _ClipRectStack?
}

void ImRotateEnd(float rad, ImVec2 center = ImRotationCenter())
{
    // Adjust the angle to match the standard horizontal orientation
    rad -= IM_PI / 2.0f; // Subtract 90 degrees (PI/2 radians)

    float s = sin(rad), c = cos(rad);
    center = ImRotate(center, s, c) - center;

    auto& buf = ImGui::GetWindowDrawList()->VtxBuffer;
    for (int i = rotation_start_index; i < buf.Size; i++)
        buf[i].pos = ImRotate(buf[i].pos, s, c) - center;
}

//Initial code for this comes from: https://gist.github.com/carasuca/e72aacadcf6cf8139de46f97158f790f
//  It doesn't seem to have a license statement, so I'm assuming it's public domain
void drawRotatedText(std::string textToRotate, ImU32 color, float font_size, ImVec2 pos, float angleToRotate = 0, bbEnum rotationCentre = BB_CE)
{
    float rad = angleToRotate * IM_PI / 180.0f;
    ImVec2 textSize = ImGui::CalcTextSize(textToRotate.c_str());
    ImVec2 textStartPosition = pos;
    ImVec2 rotationCenter;

    // Calculate rotation center based on bbEnum value
    switch (rotationCentre)
    {
    case BB_CE:
        rotationCenter = ImVec2(textStartPosition.x + textSize.x * 0.5f, textStartPosition.y + textSize.y * 0.5f);
        break;
    case BB_TL:
        rotationCenter = textStartPosition;
        break;
    case BB_TR:
        rotationCenter = ImVec2(textStartPosition.x + textSize.x, textStartPosition.y);
        break;
    case BB_BL:
        rotationCenter = ImVec2(textStartPosition.x, textStartPosition.y + textSize.y);
        break;
    case BB_BR:
        rotationCenter = ImVec2(textStartPosition.x + textSize.x, textStartPosition.y + textSize.y);
        break;
    case BB_L:
        rotationCenter = ImVec2(textStartPosition.x, textStartPosition.y + textSize.y / 2);
        break;
    case BB_R:
        rotationCenter = ImVec2(textStartPosition.x + textSize.x, textStartPosition.y + textSize.y / 2);
        break;
    case BB_T:
        rotationCenter = ImVec2(textStartPosition.x + textSize.x / 2, textStartPosition.y);
        break;
    case BB_B:
        rotationCenter = ImVec2(textStartPosition.x + textSize.x / 2, textStartPosition.y + textSize.y);
        break;
    default:
        printf("ERROR: invalid rotationCentre in drawRotatedText\n");
        return;
    }

    // Start rotation
    ImRotateStart();

    // Render the text
    ImDrawList* drawList = ImGui::GetWindowDrawList();
    //drawList->AddText(pos, IM_COL32(255, 255, 255, 255), textToRotate.c_str());
    ImFont* font13 = ImGui::GetIO().Fonts->Fonts[0];
    drawList->AddText(font13, font_size, pos, color, textToRotate.c_str());

    // Apply the rotation
    ImRotateEnd(rad, rotationCenter);
}

void rect_meter_dashed(std::string label, MeterDashSide dash_side, float meter_value, float meter_display_range, float value_interval, float major_value_interval) {
    ImGuiWindowFlags flags = ImGuiWindowFlags_NoTitleBar | ImGuiWindowFlags_NoResize;

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

float to_radians(float degrees) {
    return degrees * (M_PI / 180);
}

int textAngle = 0;

void draw_compass_dash(float angle, float relative_angle, ImVec2 dash_size, ImVec2 compass_center, float compass_radius) {
    ImU32 color = IM_COL32(200, 200, 200, 255);
    float rads = to_radians(angle);
    float cos_a = cosf(rads);
    float sin_a = sinf(rads);

    ImVec2 half_size = ImVec2(dash_size.x * 0.5f, dash_size.y * 0.5f);
    ImVec2 corners[4] = {
        ImVec2(-half_size.x, -half_size.y), // Top-left
        ImVec2( half_size.x, -half_size.y), // Top-right
        ImVec2( half_size.x,  half_size.y), // Bottom-right
        ImVec2(-half_size.x,  half_size.y)  // Bottom-left
    };

    // Rotate and translate each corner
    ImVec2 direction = ImVec2(cos_a, sin_a);
    float dist = compass_radius - dash_size.x / 2;
    ImVec2 dash_center = compass_center + direction * dist;


    for (int i = 0; i < 4; i++) {
        float x = corners[i].x * cos_a - corners[i].y * sin_a;
        float y = corners[i].x * sin_a + corners[i].y * cos_a;
        corners[i] = ImVec2(dash_center.x + x, dash_center.y + y);
    }

    ImDrawList* draw_list = ImGui::GetWindowDrawList();
    draw_list->AddQuadFilled(corners[0], corners[1], corners[2], corners[3], color);

    if ((int)relative_angle % 30 == 0 && relative_angle != 360) {
        std::string text = std::to_string((int)relative_angle).substr(0, 2);
        ImU32 color = IM_COL32(255, 255, 255, 255);
        float font_size = 18;

        if (relative_angle == 0) {
            text = "N";
            font_size = 22;
            color = IM_COL32(255, 0, 0, 255);
        } else if (relative_angle == 90) {
            text = "E";
            font_size = 22;
        } else if (relative_angle == 180) {
            text = "S";
            font_size = 22;
        } else if (relative_angle == 270) {
            text = "W";
            font_size = 22;
        }

        ImVec2 textSize = ImGui::CalcTextSize(text.c_str());
        ImVec2 textPos = dash_center - (direction * (textSize.x + 5)) - (textSize / 2);

        drawRotatedText(text, color, font_size, textPos, 180 - angle - 90);
    }
}

void compass(std::string label, float radius, float heading_angle) {
    ImGuiWindowFlags flags = ImGuiWindowFlags_NoTitleBar | ImGuiWindowFlags_NoResize | ImGuiWindowFlags_NoBackground;
    ImVec2 window_size = ImVec2(radius * 2, radius * 2);

    ImGui::SetNextWindowSize(window_size);
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.0f, 0.0f, 0.0f, 0.0f));
    ImGui::Begin(label.c_str(), nullptr, flags);

    ImVec2 window_pos = ImGui::GetWindowPos();
    ImVec2 window_center = ImVec2(window_pos.x + window_size.x / 2, window_pos.y + window_size.y / 2);

    ImDrawList* draw_list = ImGui::GetWindowDrawList();
    draw_list->AddCircleFilled(window_center, radius, IM_COL32(25, 25, 25, 153), 0);
    
    textAngle += 1;
    for (int x = 0; x < 360; x += 5) {
        ImVec2 dashSize = (x % 10 == 0) ? ImVec2(10, 2) : ImVec2(4, 2);
        std::string angle_label = (x % 30 == 0) ? std::to_string(x) : "";
        
        draw_compass_dash(-heading_angle - 90 + x, x, dashSize, window_center, radius);
    }

    ImGui::End();
    ImGui::PopStyleColor();

    ImGuiWindowFlags second_flags = ImGuiWindowFlags_NoTitleBar 
        | ImGuiWindowFlags_NoResize | ImGuiWindowFlags_NoMove | ImGuiWindowFlags_NoBackground;
    ImVec2 second_window_size = ImVec2(radius * 2, 50);
    ImVec2 second_window_pos = window_pos - ImVec2(0, second_window_size.y);
    ImGui::SetNextWindowSize(second_window_size);
    ImGui::SetNextWindowPos(second_window_pos);
    ImGui::PushStyleColor(ImGuiCol_WindowBg, ImVec4(0.0f, 0.0f, 0.0f, 0.0f));
    ImGui::Begin((label + "HeadingReader").c_str(), nullptr, second_flags);
    draw_list = ImGui::GetWindowDrawList();

    ImVec2 p1 = second_window_pos + ImVec2(second_window_size.x / 2, second_window_size.y);
    draw_list->AddTriangleFilled(
        p1,
        p1 + ImVec2(10, -15),
        p1 + ImVec2(-10, -15),
        IM_COL32(255, 255, 255, 255)
    );

    ImVec2 c1 = p1 + ImVec2(-40, -50);
    ImVec2 c2 = p1 + ImVec2(40, -15);
    draw_list->AddRectFilled(
        c1,
        c2,
        IM_COL32(0, 0, 0, 255),
        5.0f
    );

    char heading_text[6];
    snprintf(heading_text, sizeof(heading_text), "%.1f°", heading_angle);
    ImVec2 heading_text_size = ImGui::CalcTextSize(heading_text);
    ImVec2 heading_text_pos = (c1 + c2) / 2 - (heading_text_size / 2);

    draw_list->AddText(heading_text_pos, IM_COL32(255, 255, 255, 255), heading_text);

    ImGui::End();
    ImGui::PopStyleColor();
}