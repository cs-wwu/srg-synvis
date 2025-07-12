#ifndef UI_C_INTERFACE_H
#define UI_C_INTERFACE_H

#ifdef __cplusplus
extern "C" {
#endif

// Opaque pointer to C++ class
typedef struct UIRenderer UIRenderer;

// C interface functions
UIRenderer* ui_create(void* vulkan_instance, void* vulkan_device, void* vulkan_physical_device);
void ui_destroy(UIRenderer* renderer);
int ui_init_for_window(UIRenderer* renderer, void* window_handle);
void ui_new_frame(UIRenderer* renderer);
void ui_render(UIRenderer* renderer);
void ui_set_window_title(UIRenderer* renderer, const char* title);
void ui_add_text(UIRenderer* renderer, const char* text);

#ifdef __cplusplus
}
#endif

#endif