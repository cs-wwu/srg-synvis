#include "ui_renderer.h"
#include "ui_c_interface.h"
#include "imgui.h"
#include "imgui_impl_glfw.h"
#include "imgui_impl_vulkan.h"
#include <iostream>

UIRenderer::UIRenderer(VkInstance instance, VkDevice device, VkPhysicalDevice physical_device, 
    VkQueue graphics_queue, uint32_t graphics_queue_family, VkPipelineCache pipeline_cache)
    : m_instance(instance), m_device(device), m_physical_device(physical_device), 
    m_graphics_queue(graphics_queue), m_graphics_queue_family(graphics_queue_family), m_pipeline_cache(pipeline_cache),
    m_window(nullptr), m_initialized(false) {
}

UIRenderer::~UIRenderer() {
    if (m_initialized) {
        ImGui_ImplVulkan_Shutdown();
        ImGui_ImplGlfw_Shutdown();
        ImGui::DestroyContext();
    }
}

bool UIRenderer::CreateRenderPass() {
    VkAttachmentDescription color_attachment = {};
        color_attachment.format = m_swapchain_format;
        color_attachment.samples = VK_SAMPLE_COUNT_1_BIT;
        color_attachment.loadOp = VK_ATTACHMENT_LOAD_OP_LOAD;
        color_attachment.storeOp = VK_ATTACHMENT_STORE_OP_STORE;
        color_attachment.stencilLoadOp = VK_ATTACHMENT_LOAD_OP_DONT_CARE;
        color_attachment.stencilStoreOp = VK_ATTACHMENT_STORE_OP_DONT_CARE;
        color_attachment.initialLayout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL;
        color_attachment.finalLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR;

        // Color attachment reference
        VkAttachmentReference color_attachment_ref = {};
        color_attachment_ref.attachment = 0;
        color_attachment_ref.layout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL;

        // Single subpass
        VkSubpassDescription subpass = {};
        subpass.pipelineBindPoint = VK_PIPELINE_BIND_POINT_GRAPHICS;
        subpass.colorAttachmentCount = 1;
        subpass.pColorAttachments = &color_attachment_ref;

        // Subpass dependency for layout transitions
        VkSubpassDependency dependency = {};
        dependency.srcSubpass = VK_SUBPASS_EXTERNAL;
        dependency.dstSubpass = 0;
        dependency.srcStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
        dependency.srcAccessMask = 0;
        dependency.dstStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
        dependency.dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT;

        // Create render pass
        VkRenderPassCreateInfo render_pass_info = {};
        render_pass_info.sType = VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO;
        render_pass_info.attachmentCount = 1;
        render_pass_info.pAttachments = &color_attachment;
        render_pass_info.subpassCount = 1;
        render_pass_info.pSubpasses = &subpass;
        render_pass_info.dependencyCount = 1;
        render_pass_info.pDependencies = &dependency;

        VkResult result = vkCreateRenderPass(m_device, &render_pass_info, nullptr, &m_imgui_render_pass);
        return result == VK_SUCCESS;
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
    
    // The pool is very oversized, but its mentioned explicitly it needs to be this large.
	VkDescriptorPoolSize pool_sizes[] =
	{
		{ VK_DESCRIPTOR_TYPE_SAMPLER, 1000 },
		{ VK_DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, 1000 },
		{ VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE, 1000 },
		{ VK_DESCRIPTOR_TYPE_STORAGE_IMAGE, 1000 },
		{ VK_DESCRIPTOR_TYPE_UNIFORM_TEXEL_BUFFER, 1000 },
		{ VK_DESCRIPTOR_TYPE_STORAGE_TEXEL_BUFFER, 1000 },
		{ VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER, 1000 },
		{ VK_DESCRIPTOR_TYPE_STORAGE_BUFFER, 1000 },
		{ VK_DESCRIPTOR_TYPE_UNIFORM_BUFFER_DYNAMIC, 1000 },
		{ VK_DESCRIPTOR_TYPE_STORAGE_BUFFER_DYNAMIC, 1000 },
		{ VK_DESCRIPTOR_TYPE_INPUT_ATTACHMENT, 1000 }
	};

    VkDescriptorPoolCreateInfo pool_info = {};
	pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
	pool_info.flags = VK_DESCRIPTOR_POOL_CREATE_FREE_DESCRIPTOR_SET_BIT;
	pool_info.maxSets = 1000;
	pool_info.poolSizeCount = std::size(pool_sizes);
	pool_info.pPoolSizes = pool_sizes;

    VkDescriptorPool imguiPool;
	vkCreateDescriptorPool(this->m_device, &pool_info, nullptr, &imguiPool);
    CreateRenderPass();

    ImGui_ImplVulkan_InitInfo init_info = {};
    init_info.Instance = m_instance;
    init_info.PhysicalDevice = m_physical_device;
    init_info.Device = m_device;
    init_info.Queue = m_graphics_queue;
    init_info.QueueFamily = m_graphics_queue_family;
    init_info.PipelineCache = m_pipeline_cache;
    init_info.DescriptorPool = imguiPool;
    init_info.RenderPass = m_imgui_render_pass;
    init_info.Subpass = 0;
    init_info.MinImageCount = 2;
	init_info.ImageCount = m_swapchain_image_count;
	init_info.MSAASamples = VK_SAMPLE_COUNT_1_BIT;
    

    init_info.Allocator = nullptr;
    init_info.CheckVkResultFn = nullptr;
    ImGui_ImplVulkan_Init(&init_info);


    // TODO: Initialize ImGui_ImplVulkan_Init with your Vulkan setup
    // This requires render pass, command pool, etc.
    
    m_initialized = true;
    return true;
}

bool UIRenderer::CreateFrameBuffers(std::vector<VkImageView> image_views, VkFormat swapchain_format, uint32_t swapchain_image_count, int width, int height) {
    m_framebuffers.resize(image_views.size());
    m_swapchain_format = swapchain_format;
    m_swapchain_image_count = swapchain_image_count;
        
    for (size_t i = 0; i < image_views.size(); i++) {
        VkImageView attachments[] = { image_views[i] };
        
        VkFramebufferCreateInfo framebuffer_info = {};
        framebuffer_info.sType = VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO;
        framebuffer_info.renderPass = m_imgui_render_pass;
        framebuffer_info.attachmentCount = 1;
        framebuffer_info.pAttachments = attachments;
        framebuffer_info.width = width;
        framebuffer_info.height = height;
        framebuffer_info.layers = 1;
        
        VkResult result = vkCreateFramebuffer(m_device, &framebuffer_info, nullptr, &m_framebuffers[i]);
        if (result != VK_SUCCESS) {
            return false;
        }
    }
    return true;
}

void UIRenderer::NewFrame() {
    ImGui_ImplVulkan_NewFrame();
    ImGui_ImplGlfw_NewFrame();
    ImGui::NewFrame();

    bool open = true;
    ImGui::ShowDemoWindow(&open);
}

void UIRenderer::Render(VkCommandBuffer command_buffer, int image_index) {
    ImGui::Render();
    ImDrawData* draw_data = ImGui::GetDrawData();

    VkRenderPassBeginInfo render_pass_begin_info = {};
    render_pass_begin_info.sType = VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO;
    render_pass_begin_info.renderPass = m_imgui_render_pass;
    render_pass_begin_info.framebuffer = m_framebuffers[image_index];
    render_pass_begin_info.renderArea.extent.width = static_cast<uint32_t>(draw_data->DisplaySize.x);
    render_pass_begin_info.renderArea.extent.height = static_cast<uint32_t>(draw_data->DisplaySize.y);

    VkClearValue clear_values[1] = {};
    clear_values[0].color = {{0.0f, 0.0f, 0.0f, 0.0f}};  // Transparent
    render_pass_begin_info.clearValueCount = 1;
    render_pass_begin_info.pClearValues = clear_values;

    vkCmdBeginRenderPass(command_buffer, &render_pass_begin_info, VK_SUBPASS_CONTENTS_INLINE);
    ImGui_ImplVulkan_RenderDrawData(draw_data, command_buffer);  // ← Uses same command buffer
    vkCmdEndRenderPass(command_buffer);
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
    UIRenderer* ui_create(void* vulkan_instance, void* vulkan_device, void* vulkan_physical_device, void* vulkan_graphics_queue,
    uint32_t graphics_queue_family, void* vulkan_pipeline_cache) {
        return new UIRenderer(
            static_cast<VkInstance>(vulkan_instance),
            static_cast<VkDevice>(vulkan_device), 
            static_cast<VkPhysicalDevice>(vulkan_physical_device),
            static_cast<VkQueue>(vulkan_graphics_queue),
            graphics_queue_family,
            static_cast<VkPipelineCache>(vulkan_pipeline_cache)
        );
    }
    
    void ui_destroy(UIRenderer* renderer) {
        delete renderer;
    }
    
    int ui_init_for_window(UIRenderer* renderer, void* window_handle) {
        return renderer->InitForWindow(static_cast<GLFWwindow*>(window_handle)) ? 1 : 0;
    }

    int ui_create_frame_buffers(UIRenderer* renderer, void** image_views, uint32_t swapchain_format, uint32_t swapchain_image_count, int width, int height) {
        std::vector<VkImageView> views(swapchain_image_count);
        for (unsigned int i = 0; i < swapchain_image_count; i++) {
            views[i] = static_cast<VkImageView>(image_views[i]);
        }
        
        return renderer->CreateFrameBuffers(
            views,
            static_cast<VkFormat>(swapchain_format),
            swapchain_image_count,
            width,
            height
        );
    }
    
    void ui_new_frame(UIRenderer* renderer) {
        renderer->NewFrame();
    }
    
    void ui_render(UIRenderer* renderer, void* vulkan_command_buffer, int image_index) {
        renderer->Render(
            static_cast<VkCommandBuffer>(vulkan_command_buffer),
            image_index
        );
    }
    
    void ui_set_window_title(UIRenderer* renderer, const char* title) {
        renderer->SetWindowTitle(title);
    }
    
    void ui_add_text(UIRenderer* renderer, const char* text) {
        renderer->AddText(text);
    }
}