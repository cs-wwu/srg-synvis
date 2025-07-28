#ifndef UI_C_INTERFACE_H
#define UI_C_INTERFACE_H

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct UIRenderer UIRenderer;

UIRenderer* ui_create(void* vulkan_instance, void* vulkan_device, void* vulkan_physical_device, void* vulkan_graphics_queue, void* vulkan_pipeline_cache, uint32_t graphics_queue_family);
void ui_destroy(UIRenderer* renderer);
int ui_init_render_pass(UIRenderer* renderer, void** image_views, void** swapchain_images, uint32_t swapchain_format, uint32_t swapchain_image_count, int width, int height);
int ui_attach_to_window(UIRenderer* renderer, void* window_handle);
void ui_render(UIRenderer* renderer, int image_index);

int ui_altimeter(UIRenderer* renderer, const char* label, float* altitude);

// Debug UI
int ui_debug_float_slider(UIRenderer* renderer, const char* label, float* slider_value, float min_value, float max_value, const char* format);
int ui_debug_checkbox(UIRenderer* renderer, const char* label, bool* toggled);
int ui_debug_text_label(UIRenderer* renderer, const char* label);
void ui_debug_update_label(UIRenderer* renderer, int id, const char* new_label);

#ifdef __cplusplus
}
#endif

#endif