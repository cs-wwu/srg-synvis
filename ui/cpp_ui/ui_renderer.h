#ifndef UI_RENDERER_H
#define UI_RENDERER_H

#include <vulkan/vulkan.h>
#include <GLFW/glfw3.h>
#include <vector>

class UIRenderer {
public:
    UIRenderer(VkInstance instance, VkDevice device, VkPhysicalDevice physical_device, VkQueue graphics_queue,
        uint32_t graphics_queue_family, VkPipelineCache pipeline_cache);
    ~UIRenderer();
    
    bool InitForWindow(GLFWwindow* window);
    bool CreateFrameBuffers(std::vector<VkImageView> image_views, VkFormat swapchain_format, uint32_t swapchain_image_count, int width, int height);
    void NewFrame();
    void Render(VkCommandBuffer command_buffer, int image_index);
    void SetWindowTitle(const char* title);
    void AddText(const char* text);
    
private:
    bool CreateRenderPass();
    VkInstance m_instance;
    VkDevice m_device;
    VkPhysicalDevice m_physical_device;
    GLFWwindow* m_window;
    VkQueue m_graphics_queue;
    uint32_t m_graphics_queue_family;
    VkPipelineCache m_pipeline_cache;
    VkRenderPass m_imgui_render_pass;
    VkFormat m_swapchain_format;
    uint32_t m_swapchain_image_count;
    std::vector<VkFramebuffer> m_framebuffers;
    bool m_initialized;
};

#endif