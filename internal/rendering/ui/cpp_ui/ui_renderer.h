#ifndef UI_RENDERER_H
#define UI_RENDERER_H

#include <vulkan/vulkan.h>
#include <GLFW/glfw3.h>
#include <vector>
#include <functional>
#include <unordered_map>
#include <string>

struct UIOperation {
    int id;
    std::function<void()> fn;
};

class UIRenderer {
public:
    UIRenderer(VkInstance instance, VkDevice device, VkPhysicalDevice physical_device, VkQueue graphics_queue, 
        VkPipelineCache pipeline_cache, uint32_t graphics_queue_family);
    ~UIRenderer();
    
    bool AttachToWindow(GLFWwindow* window);
    bool InitRenderPass(std::vector<VkImageView> image_views, std::vector<VkImage> swapchain_images, VkFormat swapchain_format, uint32_t swapchain_image_count, int width, int height);
    void Render(int image_index);
    int DebugFloatSlider(std::string label, float* slider_value, float min_value, float max_value, std::string format);
private:
    bool CreateRenderPass();
    bool CreateCommandBuffers();
    VkInstance m_instance;
    VkDevice m_device;
    VkPhysicalDevice m_physical_device;
    GLFWwindow* m_window;
    VkQueue m_graphics_queue;
    uint32_t m_graphics_queue_family;
    VkPipelineCache m_pipeline_cache;
    VkDescriptorPool m_descriptor_pool;
    VkRenderPass m_imgui_render_pass;
    VkFormat m_swapchain_format;
    uint32_t m_swapchain_image_count;
    std::vector<VkFramebuffer> m_framebuffers;
    std::vector<VkImage> m_swapchain_images;
    std::vector<int> m_operation_order;
    std::unordered_map<int, UIOperation> m_ui_operations;

    VkCommandPool m_command_pool;
    std::vector<VkCommandBuffer> m_command_buffers;
    std::vector<VkFence> m_fences;

    int m_next_op_id;
    bool m_initialized;
};

#endif