#ifndef UI_WIDGETS_H
#define UI_WIDGETS_H
#include <string>

enum MeterDashSide {
    LEFT,
    RIGHT
};

void rect_meter_dashed(std::string label, MeterDashSide dash_side, float meter_value, float meter_display_range, float value_interval, float major_value_interval);
void compass(std::string label, float radius, float heading_angle);

#endif