#ifndef UI_RENDERER_H
#define UI_RENDERER_H

#include <vulkan/vulkan.h>
#include <GLFW/glfw3.h>

class UIRenderer {
public:
    UIRenderer(VkInstance instance, VkDevice device, VkPhysicalDevice physical_device);
    ~UIRenderer();
    
    bool InitForWindow(GLFWwindow* window);
    void NewFrame();
    void Render();
    void SetWindowTitle(const char* title);
    void AddText(const char* text);
    
private:
    VkInstance m_instance;
    VkDevice m_device;
    VkPhysicalDevice m_physical_device;
    GLFWwindow* m_window;
    bool m_initialized;
};

#endif