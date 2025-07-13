#ifndef UI_C_INTERFACE_H
#define UI_C_INTERFACE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque pointer to C++ class
typedef struct UIRenderer UIRenderer;

// C interface functions
UIRenderer* ui_create(void* vulkan_instance, void* vulkan_device, void* vulkan_physical_device, void* vulkan_graphics_queue, void* vulkan_pipeline_cache, uint32_t graphics_queue_family);
void ui_destroy(UIRenderer* renderer);
int ui_create_frame_buffers(UIRenderer* renderer, void** image_views, uint32_t swapchain_format, uint32_t swapchain_image_count, int width, int height);
int ui_init_for_window(UIRenderer* renderer, void* window_handle);
void ui_new_frame(UIRenderer* renderer);
void ui_render(UIRenderer* renderer, void* vulkan_command_buffer, int image_index);

#ifdef __cplusplus
}
#endif

#endif