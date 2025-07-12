#include "ui_renderer.h"
#include "ui_c_interface.h"
#include "imgui.h"
#include "imgui_impl_glfw.h"
#include "imgui_impl_vulkan.h"
#include <iostream>

UIRenderer::UIRenderer(VkInstance instance, VkDevice device, VkPhysicalDevice physical_device)
    : m_instance(instance), m_device(device), m_physical_device(physical_device), 
      m_window(nullptr), m_initialized(false) {
}

UIRenderer::~UIRenderer() {
    if (m_initialized) {
        ImGui_ImplVulkan_Shutdown();
        ImGui_ImplGlfw_Shutdown();
        ImGui::DestroyContext();
    }
}

bool UIRenderer::InitForWindow(GLFWwindow* window) {
    m_window = window;
    
    // Setup Dear ImGui context
    IMGUI_CHECKVERSION();
    ImGui::CreateContext();
    ImGuiIO& io = ImGui::GetIO();
    io.ConfigFlags |= ImGuiConfigFlags_NavEnableKeyboard;
    
    // Setup Dear ImGui style
    ImGui::StyleColorsDark();
    
    // Setup Platform/Renderer backends
    ImGui_ImplGlfw_InitForVulkan(window, true);
    
    // TODO: Initialize ImGui_ImplVulkan_Init with your Vulkan setup
    // This requires render pass, command pool, etc.
    
    m_initialized = true;
    return true;
}

void UIRenderer::NewFrame() {
    ImGui_ImplVulkan_NewFrame();
    ImGui_ImplGlfw_NewFrame();
    ImGui::NewFrame();
}

void UIRenderer::Render() {
    ImGui::Render();
    // TODO: Submit ImGui draw data to your Vulkan command buffer
}

void UIRenderer::SetWindowTitle(const char* title) {
    if (m_window) {
        glfwSetWindowTitle(m_window, title);
    }
}

void UIRenderer::AddText(const char* text) {
    ImGui::Text("%s", text);
}

// C interface implementations
extern "C" {
    UIRenderer* ui_create(void* vulkan_instance, void* vulkan_device, void* vulkan_physical_device) {
        return new UIRenderer(
            static_cast<VkInstance>(vulkan_instance),
            static_cast<VkDevice>(vulkan_device), 
            static_cast<VkPhysicalDevice>(vulkan_physical_device)
        );
    }
    
    void ui_destroy(UIRenderer* renderer) {
        delete renderer;
    }
    
    int ui_init_for_window(UIRenderer* renderer, void* window_handle) {
        return renderer->InitForWindow(static_cast<GLFWwindow*>(window_handle)) ? 1 : 0;
    }
    
    void ui_new_frame(UIRenderer* renderer) {
        renderer->NewFrame();
    }
    
    void ui_render(UIRenderer* renderer) {
        renderer->Render();
    }
    
    void ui_set_window_title(UIRenderer* renderer, const char* title) {
        renderer->SetWindowTitle(title);
    }
    
    void ui_add_text(UIRenderer* renderer, const char* text) {
        renderer->AddText(text);
    }
}