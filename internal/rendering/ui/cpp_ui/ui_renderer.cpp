#include "ui_renderer.h"
#include "ui_c_interface.h"
#include "imgui.h"
#include "imgui_impl_glfw.h"
#include "imgui_impl_vulkan.h"
#include <iostream>

UIRenderer::UIRenderer(VkInstance instance, VkDevice device, VkPhysicalDevice physical_device, 
    VkQueue graphics_queue, VkPipelineCache pipeline_cache, uint32_t graphics_queue_family)
    : m_instance(instance), m_device(device), m_physical_device(physical_device), 
    m_graphics_queue(graphics_queue), m_pipeline_cache(pipeline_cache), m_graphics_queue_family(graphics_queue_family),
    m_window(nullptr), m_initialized(false), m_next_op_id(0) {
}

UIRenderer::~UIRenderer() {
    if (m_initialized) {
        // Wait for everything to finish before cleaning up
        vkDeviceWaitIdle(m_device);
        vkWaitForFences(m_device, m_fences.size(), m_fences.data(), VK_TRUE, UINT64_MAX);

        ImGui_ImplVulkan_Shutdown();
        ImGui_ImplGlfw_Shutdown();
        ImGui::DestroyContext();

        // Destroy Fences
        for (int i = 0; i < m_swapchain_image_count; i++) {
            vkDestroyFence(m_device, m_fences[i], nullptr);
            m_fences[i] = VK_NULL_HANDLE;
        }
        m_fences.clear();

        // Command buffers and command pool
        vkDestroyCommandPool(m_device, m_command_pool, nullptr); // Also frees command buffers
        m_command_buffers.clear();

        // Frame buffers
        for (int i = 0; i < m_framebuffers.size(); i++) {
            vkDestroyFramebuffer(m_device, m_framebuffers[i], nullptr);
            m_framebuffers[i] = VK_NULL_HANDLE;
        }
        m_framebuffers.clear();

        vkDestroyRenderPass(m_device, m_imgui_render_pass, nullptr);
        m_imgui_render_pass = VK_NULL_HANDLE;

        vkDestroyDescriptorPool(m_device, m_descriptor_pool, nullptr);
        m_descriptor_pool = VK_NULL_HANDLE;

        printf("UI Renderer Cleanup Complete!\n");
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

    VkAttachmentReference color_attachment_ref = {};
        color_attachment_ref.attachment = 0;
        color_attachment_ref.layout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL;

    VkSubpassDescription subpass = {};
        subpass.pipelineBindPoint = VK_PIPELINE_BIND_POINT_GRAPHICS;
        subpass.colorAttachmentCount = 1;
        subpass.pColorAttachments = &color_attachment_ref;

    VkSubpassDependency dependency = {};
        dependency.srcSubpass = VK_SUBPASS_EXTERNAL;
        dependency.dstSubpass = 0;
        dependency.srcStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
        dependency.srcAccessMask = 0;
        dependency.dstStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
        dependency.dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT;

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

bool UIRenderer::AttachToWindow(GLFWwindow* window) {
    m_window = window;

    // Setup Dear ImGui context
    IMGUI_CHECKVERSION();
    ImGui::CreateContext();
    ImGuiIO& io = ImGui::GetIO();
    io.ConfigFlags |= ImGuiConfigFlags_NavEnableKeyboard;
    
    // Setup Platform/Renderer backends
    ImGui_ImplGlfw_InitForVulkan(window, true);
    
    // The pool is very oversized, an example I found said to do this, idk what else to set it to
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

	vkCreateDescriptorPool(this->m_device, &pool_info, nullptr, &m_descriptor_pool);

    ImGui_ImplVulkan_InitInfo init_info = {};
    init_info.Instance = m_instance;
    init_info.PhysicalDevice = m_physical_device;
    init_info.Device = m_device;
    init_info.Queue = m_graphics_queue;
    init_info.QueueFamily = m_graphics_queue_family;
    init_info.PipelineCache = m_pipeline_cache;
    init_info.DescriptorPool = m_descriptor_pool;
    init_info.RenderPass = m_imgui_render_pass;
    init_info.Subpass = 0;
    init_info.MinImageCount = 2;
	init_info.ImageCount = m_swapchain_image_count;
	init_info.MSAASamples = VK_SAMPLE_COUNT_1_BIT;
    init_info.Allocator = nullptr;
    init_info.CheckVkResultFn = nullptr;

    ImGui_ImplVulkan_Init(&init_info);
    
    m_initialized = true;
    return true;
}

bool UIRenderer::CreateCommandBuffers() {
    VkCommandPoolCreateInfo pool_info = {
        .sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO,
        .flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT,
        .queueFamilyIndex = m_graphics_queue_family
    };

    VkResult result = vkCreateCommandPool(m_device, &pool_info, nullptr, &m_command_pool);
    if (result != VK_SUCCESS) {
        printf("IMGUI Command pool allocation failed!\n");
        return false;
    }

    VkCommandBufferAllocateInfo alloc_info = {
        .sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO,
        .commandPool = m_command_pool,
        .level = VK_COMMAND_BUFFER_LEVEL_PRIMARY,
        .commandBufferCount = m_swapchain_image_count
    };

    m_command_buffers.resize(m_swapchain_image_count);
    result = vkAllocateCommandBuffers(m_device, &alloc_info, m_command_buffers.data());

    if (result != VK_SUCCESS) {
        printf("IMGUI Command buffer allocations failed!\n");
        return false;
    }

    // Create fences
    m_fences.resize(m_swapchain_image_count);
    VkFenceCreateInfo fence_info = {};
    fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
    fence_info.flags = VK_FENCE_CREATE_SIGNALED_BIT;

    for (int i = 0; i < m_swapchain_image_count; i++) {
        result = vkCreateFence(m_device, &fence_info, nullptr, &m_fences[i]);
        
        if (result != VK_SUCCESS) {
            printf("ERROR: Failed to create fence %u! Result: %d\n", i, result);
            return false;
        }
    }

    return true;
}

bool UIRenderer::InitRenderPass(std::vector<VkImageView> image_views, std::vector<VkImage> swapchain_images, VkFormat swapchain_format, uint32_t swapchain_image_count, int width, int height) {
    m_framebuffers.resize(image_views.size());
    m_swapchain_format = swapchain_format;
    m_swapchain_image_count = swapchain_image_count;
    m_swapchain_images = swapchain_images;
        
    CreateRenderPass();
    CreateCommandBuffers();

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

void UIRenderer::Render(int image_index) {
    VkCommandBuffer command_buffer = m_command_buffers[image_index];
    VkFence fence = m_fences[image_index];

    // Wait for command buffer to finish its last cycle
    vkWaitForFences(m_device, 1, &fence, VK_TRUE, UINT64_MAX);
    vkResetFences(m_device, 1, &fence);

    VkCommandBufferBeginInfo cmd_begin_info = {};
    cmd_begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    cmd_begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    cmd_begin_info.pInheritanceInfo = nullptr;
    cmd_begin_info.pNext = nullptr;

    vkResetCommandBuffer(command_buffer, 0);
    vkBeginCommandBuffer(command_buffer, &cmd_begin_info);

    ImGui_ImplVulkan_NewFrame();
    ImGui_ImplGlfw_NewFrame();
    ImGui::NewFrame();

    for (size_t i = 0; i < m_operation_order.size(); i++) {
        UIOperation* op = m_ui_operations[i];
        op->execute();
    }

    ImGui::Render();
    ImDrawData* draw_data = ImGui::GetDrawData();

    VkImageMemoryBarrier barrier1 = {};
    barrier1.sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER;
    barrier1.srcAccessMask = VK_ACCESS_MEMORY_READ_BIT;
    barrier1.dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT;
    barrier1.oldLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR;
    barrier1.newLayout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL;
    barrier1.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier1.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier1.image = m_swapchain_images[image_index];
    barrier1.subresourceRange.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT;
    barrier1.subresourceRange.baseMipLevel = 0;
    barrier1.subresourceRange.levelCount = 1;
    barrier1.subresourceRange.baseArrayLayer = 0;
    barrier1.subresourceRange.layerCount = 1;

    vkCmdPipelineBarrier(
        command_buffer,
        VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT,
        VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
        0,
        0, nullptr,
        0, nullptr,
        1, &barrier1
    );

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
    ImGui_ImplVulkan_RenderDrawData(draw_data, command_buffer); 
    vkCmdEndRenderPass(command_buffer);
    
    VkSubmitInfo submit_info = {};
    submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submit_info.commandBufferCount = 1;
    submit_info.pCommandBuffers = &command_buffer;
    
    vkEndCommandBuffer(command_buffer);
    VkResult result = vkQueueSubmit(m_graphics_queue, 1, &submit_info, fence);
}

/*
Creates a slider for float values.
*/
int UIRenderer::DebugFloatSlider(std::string label, float* slider_value, float min_value, float max_value, std::string format) {
    int id = m_next_op_id;
    m_next_op_id++;

    UIOperation* floatOp = new UIOperation();
    floatOp->id = id;
    floatOp->type = UIOperation::FLOAT_SLIDER;
    floatOp->label = label;
    floatOp->formatStr = format;
    floatOp->min_value_float = min_value;
    floatOp->max_value_float = max_value;
    floatOp->value_float = slider_value;
    floatOp->execute = [floatOp](){
        ImGui::SliderFloat(floatOp->label.c_str(), floatOp->value_float, floatOp->min_value_float, floatOp->max_value_float, floatOp->formatStr.c_str());
    };

    m_ui_operations[id] = floatOp;
    m_operation_order.push_back(id);

    return id;
}

int UIRenderer::DebugCheckbox(std::string label, bool* toggled) {
    int id = m_next_op_id;
    m_next_op_id++;

    UIOperation* checkOp = new UIOperation();
    checkOp->type = UIOperation::CHECKBOX;
    checkOp->id = id;
    checkOp->label = label;
    checkOp->toggled = toggled;
    checkOp->execute = [checkOp](){
        ImGui::Checkbox(checkOp->label.c_str(), checkOp->toggled);
    };

    m_ui_operations[id] = checkOp;
    m_operation_order.push_back(id);

    return id;
}

int UIRenderer::DebugTextLabel(std::string label) {
    int id = m_next_op_id;
    m_next_op_id++;

    UIOperation* op = new UIOperation();
    op->type = UIOperation::TEXT_LABEL;
    op->id = id;
    op->label = label;
    op->execute = [op](){
        ImGui::Text(op->label.c_str());
    };

    m_ui_operations[id] = op;
    m_operation_order.push_back(id);

    return id;
}

void UIRenderer::DebugUpdateLabel(int id, std::string newLabel) {
    UIOperation* op = m_ui_operations[id];
    op->label = newLabel;
}

// C interface implementations
extern "C" {
    UIRenderer* ui_create(void* vulkan_instance, void* vulkan_device, void* vulkan_physical_device, void* vulkan_graphics_queue,
    void* vulkan_pipeline_cache, uint32_t graphics_queue_family) {
        return new UIRenderer(
            static_cast<VkInstance>(vulkan_instance),
            static_cast<VkDevice>(vulkan_device), 
            static_cast<VkPhysicalDevice>(vulkan_physical_device),
            static_cast<VkQueue>(vulkan_graphics_queue),
            static_cast<VkPipelineCache>(vulkan_pipeline_cache),
            graphics_queue_family
        );
    }
    
    void ui_destroy(UIRenderer* renderer) {
        delete renderer;
    }
    
    int ui_attach_to_window(UIRenderer* renderer, void* window_handle) {
        return renderer->AttachToWindow(static_cast<GLFWwindow*>(window_handle)) ? 1 : 0;
    }

    int ui_init_render_pass(UIRenderer* renderer, void** image_views, void** swapchain_images, uint32_t swapchain_format, uint32_t swapchain_image_count, int width, int height) {
        std::vector<VkImageView> views(swapchain_image_count);
        std::vector<VkImage> images(swapchain_image_count);
        for (unsigned int i = 0; i < swapchain_image_count; i++) {
            views[i] = static_cast<VkImageView>(image_views[i]);
            images[i] = static_cast<VkImage>(swapchain_images[i]);
        }
        
        return renderer->InitRenderPass(
            views,
            images,
            static_cast<VkFormat>(swapchain_format),
            swapchain_image_count,
            width,
            height
        );
    }
    
    void ui_render(UIRenderer* renderer, int image_index) {
        renderer->Render(
            image_index
        );
    }

    int ui_debug_float_slider(UIRenderer* renderer, const char* label, float* slider_value, float min_value, float max_value, const char* format) {
        std::string labelStr(label ? label : "");
        std::string formatStr(format ? format : "%.2f");
        
        return renderer->DebugFloatSlider(labelStr, slider_value, min_value, max_value, formatStr);
    }

    int ui_debug_checkbox(UIRenderer* renderer, const char* label, bool* toggled) {
        std::string labelStr(label ? label : "");
        return renderer->DebugCheckbox(labelStr, toggled);
    }

    int ui_debug_text_label(UIRenderer* renderer, const char* label) {
        std::string labelStr(label ? label : "");
        return renderer->DebugTextLabel(labelStr);
    }

    void ui_debug_update_label(UIRenderer* renderer, int id, const char* new_label) {
        std::string labelStr(new_label ? new_label : "");
        renderer->DebugUpdateLabel(id, labelStr);
    }
}