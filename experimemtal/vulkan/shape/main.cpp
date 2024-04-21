#define GLFW_INCLUDE_VULKAN // GLFW will automatically load the Vulkan header and include its own definitions
#include <GLFW/glfw3.h>

#include <iostream>
#include <fstream>
#include <stdexcept>
#include <algorithm>
#include <vector>
#include <cstring>
#include <cstdlib>
#include <cstdint>
#include <limits>
#include <optional>
#include <set>

const uint32_t WIDTH = 800;
const uint32_t HEIGHT = 600;

const int MAX_FRAMES_IN_FLIGHT = 2; // 1 for gpu, 1 for cpu

// for validation layers
const std::vector<const char*> validationLayers = {
    "VK_LAYER_KHRONOS_validation"
};

// declare a list of required device extensions
const std::vector<const char*> deviceExtensions = {
    VK_KHR_SWAPCHAIN_EXTENSION_NAME
};

#ifdef NDEBUG
const bool enableValidationLayers = false;
#else
const bool enableValidationLayers = true;
#endif


// for debug messages
VkResult CreateDebugUtilsMessengerEXT(VkInstance instance, const VkDebugUtilsMessengerCreateInfoEXT* pCreateInfo, const VkAllocationCallbacks* pAllocator, VkDebugUtilsMessengerEXT* pDebugMessenger) {
    auto func = (PFN_vkCreateDebugUtilsMessengerEXT) vkGetInstanceProcAddr(instance, "vkCreateDebugUtilsMessengerEXT");
    if (func != nullptr) {
        return func(instance, pCreateInfo, pAllocator, pDebugMessenger);
    } else {
        return VK_ERROR_EXTENSION_NOT_PRESENT;
    }
}
// cleans debug messenger
void DestroyDebugUtilsMessengerEXT(VkInstance instance, VkDebugUtilsMessengerEXT debugMessenger, const VkAllocationCallbacks* pAllocator) {
    auto func = (PFN_vkDestroyDebugUtilsMessengerEXT) vkGetInstanceProcAddr(instance, "vkDestroyDebugUtilsMessengerEXT");
    if (func != nullptr) {
        func(instance, debugMessenger, pAllocator);
    }
}

struct QueueFamilyIndices {
    // std::optional is a wrapper that contains no value until you assign something to it
    // has_value() will tell if value exists for not
    std::optional<uint32_t> graphicsFamily;

    // take into account the queue family drawing commands and ones supporting presentation do not overlap
    std::optional<uint32_t> presentFamily;

    bool isComplete() {
        return graphicsFamily.has_value() && presentFamily.has_value();
    }
};

struct SwapChainSupportDetails {
    VkSurfaceCapabilitiesKHR capabilities;
    std::vector<VkSurfaceFormatKHR> formats;
    std::vector<VkPresentModeKHR> presentModes;
};

class HelloTriangleApplication {
public:
    void run() {
        initWindow();
        initVulkan();   // store and initiate vulkan objects
        mainLoop();     // once everything is prepared enter to render frames
        cleanup();      // deallocate resources
    }

private:
    GLFWwindow* window;

    VkInstance instance;
    VkDebugUtilsMessengerEXT debugMessenger;
    VkSurfaceKHR surface;

    VkPhysicalDevice physicalDevice = VK_NULL_HANDLE;
    VkDevice device;

    VkQueue graphicsQueue;
    VkQueue presentQueue;

    VkSwapchainKHR swapChain;
    std::vector<VkImage> swapChainImages;
    VkFormat swapChainImageFormat;
    VkExtent2D swapChainExtent;
    std::vector<VkImageView> swapChainImageViews;
    std::vector<VkFramebuffer> swapChainFramebuffers;

    VkRenderPass renderPass;
    VkPipelineLayout pipelineLayout;
    VkPipeline graphicsPipeline;

    VkCommandPool commandPool;
    std::vector<VkCommandBuffer> commandBuffers;

    std::vector<VkSemaphore> imageAvailableSemaphores;
    std::vector<VkSemaphore> renderFinishedSemaphores;
    std::vector<VkFence> inFlightFences;
    uint32_t currentFrame = 0;

    bool framebufferResized = false;

    void initWindow() {
        glfwInit(); // initialize GLFW library

        glfwWindowHint(GLFW_CLIENT_API, GLFW_NO_API); // GLFW designed to create an OpenGL context - disable that

        window = glfwCreateWindow(WIDTH, HEIGHT, "Vulkan", nullptr, nullptr); // (width, height, windowTitle, windowMoniter, OpenGLparam)

        // window is resizeable + detection of resize
        glfwSetWindowUserPointer(window, this);
        glfwSetFramebufferSizeCallback(window, framebufferResizeCallback);
    }

    static void framebufferResizeCallback(GLFWwindow* window, int width, int height) {
        auto app = reinterpret_cast<HelloTriangleApplication*>(glfwGetWindowUserPointer(window));
        app->framebufferResized = true;
    }

    void initVulkan() {
        createInstance();
        setupDebugMessenger();
        createSurface();
        pickPhysicalDevice();
        createLogicalDevice();
        createSwapChain();
        createImageViews();
        createRenderPass();
        createGraphicsPipeline();
        createFramebuffers();
        createCommandPool();
        createCommandBuffers();
        createSyncObjects();
    }

    void mainLoop() {
        // runs until window is closed
        while (!glfwWindowShouldClose(window)) {
            glfwPollEvents();
            drawFrame();
        }
        // to exit w/o problems when closing window
        vkDeviceWaitIdle(device);
    }

    // special fctn for recreating swapchain
    void cleanupSwapChain() {
        for (auto framebuffer : swapChainFramebuffers) {
            vkDestroyFramebuffer(device, framebuffer, nullptr);
        }

        for (auto imageView : swapChainImageViews) {
            vkDestroyImageView(device, imageView, nullptr);
        }

        vkDestroySwapchainKHR(device, swapChain, nullptr);
    }

    void cleanup() {
        cleanupSwapChain();

        vkDestroyPipeline(device, graphicsPipeline, nullptr);
        vkDestroyPipelineLayout(device, pipelineLayout, nullptr);

        vkDestroyRenderPass(device, renderPass, nullptr);

        for (size_t i = 0; i < MAX_FRAMES_IN_FLIGHT; i++) {
            vkDestroySemaphore(device, renderFinishedSemaphores[i], nullptr);
            vkDestroySemaphore(device, imageAvailableSemaphores[i], nullptr);
            vkDestroyFence(device, inFlightFences[i], nullptr);
        }

        vkDestroyCommandPool(device, commandPool, nullptr);

        vkDestroyDevice(device, nullptr);
        // device queues are implicitly cleaned up when device destroyed

        if (enableValidationLayers) {
            DestroyDebugUtilsMessengerEXT(instance, debugMessenger, nullptr);
        }

        vkDestroySurfaceKHR(instance, surface, nullptr);
        vkDestroyInstance(instance, nullptr); // only destroy right before program exits

        glfwDestroyWindow(window);

        glfwTerminate();
    }

    void recreateSwapChain() {
        // handle minimization
        int width = 0, height = 0;
        glfwGetFramebufferSize(window, &width, &height);
        while (width == 0 || height == 0) {
            glfwGetFramebufferSize(window, &width, &height);
            glfwWaitEvents();
        }

        vkDeviceWaitIdle(device);   // don't touch resources still in use

        cleanupSwapChain();         // make sure old versions are cleaned up
        // we could also recreate render pass
        // this approach stops all rendering before creating the swapchan - there is a way to do this while rendering is in-flight
        //  by passing prev swapchain to oldSwapChain field in the VkSwapchainCreateInfoKHR struct and then destroy it later

        createSwapChain();
        createImageViews();
        createFramebuffers();
    }

    /*
    initialize Vulkan library by creating an instance
    the instance is the connection between application and Vulkan library
    */
    void createInstance() {
        if (enableValidationLayers && !checkValidationLayerSupport()) {
            throw std::runtime_error("validation layers requested, but not available!");
        }

        // provides info about the driver to optimize our application (optional)
        VkApplicationInfo appInfo{};
        appInfo.sType = VK_STRUCTURE_TYPE_APPLICATION_INFO;
        appInfo.pApplicationName = "Hello Triangle";
        appInfo.applicationVersion = VK_MAKE_VERSION(1, 0, 0);
        appInfo.pEngineName = "No Engine";
        appInfo.engineVersion = VK_MAKE_VERSION(1, 0, 0);
        appInfo.apiVersion = VK_API_VERSION_1_0;

        // (required) tells vulkan driver which global extensions and validation layers we want to use
        VkInstanceCreateInfo createInfo{};
        createInfo.sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO;
        createInfo.pApplicationInfo = &appInfo;

        // message callback for extension errors
        auto extensions = getRequiredExtensions();
        createInfo.enabledExtensionCount = static_cast<uint32_t>(extensions.size());
        createInfo.ppEnabledExtensionNames = extensions.data();

        VkDebugUtilsMessengerCreateInfoEXT debugCreateInfo{};
        // include Validation layer names if enabled
        if (enableValidationLayers) {
            createInfo.enabledLayerCount = static_cast<uint32_t>(validationLayers.size());
            createInfo.ppEnabledLayerNames = validationLayers.data();

            // create debug instance
            populateDebugMessengerCreateInfo(debugCreateInfo);
            createInfo.pNext = (VkDebugUtilsMessengerCreateInfoEXT*) &debugCreateInfo;
        } else {
            createInfo.enabledLayerCount = 0;

            createInfo.pNext = nullptr;
        }

        // if all went well, handle to instance stored in VkInstance
        if (vkCreateInstance(&createInfo, nullptr, &instance) != VK_SUCCESS) {
            throw std::runtime_error("failed to create instance!");
        }
    }

    void populateDebugMessengerCreateInfo(VkDebugUtilsMessengerCreateInfoEXT& createInfo) {
        // struct about details of messenger and its callback
        // passed to vkCreateDebugUtilsMessengerEXT, but we have to look up this address using vkGetInstanceProcAddr
        createInfo = {};
        createInfo.sType = VK_STRUCTURE_TYPE_DEBUG_UTILS_MESSENGER_CREATE_INFO_EXT;
        // event has happened unrelated to the specification
        // event has violated specification or indicated mistake
        // non-optimal vulkan use
        createInfo.messageSeverity = VK_DEBUG_UTILS_MESSAGE_SEVERITY_VERBOSE_BIT_EXT | VK_DEBUG_UTILS_MESSAGE_SEVERITY_WARNING_BIT_EXT | VK_DEBUG_UTILS_MESSAGE_SEVERITY_ERROR_BIT_EXT;
        createInfo.messageType = VK_DEBUG_UTILS_MESSAGE_TYPE_GENERAL_BIT_EXT | VK_DEBUG_UTILS_MESSAGE_TYPE_VALIDATION_BIT_EXT | VK_DEBUG_UTILS_MESSAGE_TYPE_PERFORMANCE_BIT_EXT;
        // specifies ptr to callback fctn
        createInfo.pfnUserCallback = debugCallback;
    }

    // debug callback is managed with a handle
    void setupDebugMessenger() {
        if (!enableValidationLayers) return;

        VkDebugUtilsMessengerCreateInfoEXT createInfo;
        populateDebugMessengerCreateInfo(createInfo);

        if (CreateDebugUtilsMessengerEXT(instance, &createInfo, nullptr, &debugMessenger) != VK_SUCCESS) {
            throw std::runtime_error("failed to set up debug messenger!");
        }
    }

    void createSurface() {
        if (glfwCreateWindowSurface(instance, window, nullptr, &surface) != VK_SUCCESS) {
            throw std::runtime_error("failed to create window surface!");
        }
    }

    // select a graphics card that supports necessary features
    // the graphics card we select will be stored in VkPhysicalDEvice, and will be implicitly destroyed with VkInstance
    void pickPhysicalDevice() {
        // list graphics cards
        uint32_t deviceCount = 0;
        vkEnumeratePhysicalDevices(instance, &deviceCount, nullptr);

        if (deviceCount == 0) {
            throw std::runtime_error("failed to find GPUs with Vulkan support!");
        }

        // allocate array to hold handles
        std::vector<VkPhysicalDevice> devices(deviceCount);
        vkEnumeratePhysicalDevices(instance, &deviceCount, devices.data());

        // evaluate and check all devices
        for (const auto& device : devices) {
            if (isDeviceSuitable(device)) {
                physicalDevice = device;
                break;
            }
        }

        if (physicalDevice == VK_NULL_HANDLE) {
            throw std::runtime_error("failed to find a suitable GPU!");
        }
    }

    void createLogicalDevice() {
        QueueFamilyIndices indices = findQueueFamilies(physicalDevice);

        // struct describes the # of queues we want for a qf
        std::vector<VkDeviceQueueCreateInfo> queueCreateInfos;
        std::set<uint32_t> uniqueQueueFamilies = {indices.graphicsFamily.value(), indices.presentFamily.value()};

        float queuePriority = 1.0f; // must be between 0 and 1
        for (uint32_t queueFamily : uniqueQueueFamilies) {
            VkDeviceQueueCreateInfo queueCreateInfo{};
            queueCreateInfo.sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
            queueCreateInfo.queueFamilyIndex = queueFamily;
            queueCreateInfo.queueCount = 1;
            queueCreateInfo.pQueuePriorities = &queuePriority;
            queueCreateInfos.push_back(queueCreateInfo);
        }

        VkPhysicalDeviceFeatures deviceFeatures{};

        VkDeviceCreateInfo createInfo{};
        createInfo.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO;

        // modify VkDeviceCreateInfo to pt to the vector
        createInfo.queueCreateInfoCount = static_cast<uint32_t>(queueCreateInfos.size());
        createInfo.pQueueCreateInfos = queueCreateInfos.data();

        createInfo.pEnabledFeatures = &deviceFeatures;

        // enable swapchain extension
        createInfo.enabledExtensionCount = static_cast<uint32_t>(deviceExtensions.size());
        createInfo.ppEnabledExtensionNames = deviceExtensions.data();

        if (enableValidationLayers) {
            createInfo.enabledLayerCount = static_cast<uint32_t>(validationLayers.size());
            createInfo.ppEnabledLayerNames = validationLayers.data();
        } else {
            createInfo.enabledLayerCount = 0;
        }

        // now we can create the logical device
        // (physDevice, queue/usageInfo, allocationCallbackptr, ptr to tore the logical device handle in)
        if (vkCreateDevice(physicalDevice, &createInfo, nullptr, &device) != VK_SUCCESS) {
            throw std::runtime_error("failed to create logical device!");
        }

        // get queue handles for each family
        vkGetDeviceQueue(device, indices.graphicsFamily.value(), 0, &graphicsQueue);
        // retreive queue handle
        vkGetDeviceQueue(device, indices.presentFamily.value(), 0, &presentQueue);
    }

    void createSwapChain() {
        SwapChainSupportDetails swapChainSupport = querySwapChainSupport(physicalDevice);

        VkSurfaceFormatKHR surfaceFormat = chooseSwapSurfaceFormat(swapChainSupport.formats);
        VkPresentModeKHR presentMode = chooseSwapPresentMode(swapChainSupport.presentModes);
        VkExtent2D extent = chooseSwapExtent(swapChainSupport.capabilities);

        // how many images in swapchain -1 above minimum, below maximum
        uint32_t imageCount = swapChainSupport.capabilities.minImageCount + 1;
        if (swapChainSupport.capabilities.maxImageCount > 0 && imageCount > swapChainSupport.capabilities.maxImageCount) {
            imageCount = swapChainSupport.capabilities.maxImageCount;
        }

        // filling large struct to create swapchain
        VkSwapchainCreateInfoKHR createInfo{};
        createInfo.sType = VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR;
        createInfo.surface = surface;

        createInfo.minImageCount = imageCount;
        createInfo.imageFormat = surfaceFormat.format;
        createInfo.imageColorSpace = surfaceFormat.colorSpace;
        createInfo.imageExtent = extent;
        createInfo.imageArrayLayers = 1;    // 1 unless stereographic 3d
        // directly render to imageUsage
        createInfo.imageUsage = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT;

        QueueFamilyIndices indices = findQueueFamilies(physicalDevice);
        uint32_t queueFamilyIndices[] = {indices.graphicsFamily.value(), indices.presentFamily.value()};
        // how to handle if swap chain images that will be used across multiple queue families - this uses concurrent mode
        if (indices.graphicsFamily != indices.presentFamily) {
            createInfo.imageSharingMode = VK_SHARING_MODE_CONCURRENT;
            createInfo.queueFamilyIndexCount = 2;
            createInfo.pQueueFamilyIndices = queueFamilyIndices;
        } else {
            createInfo.imageSharingMode = VK_SHARING_MODE_EXCLUSIVE;
        }

        createInfo.preTransform = swapChainSupport.capabilities.currentTransform;
        // specifies that we do not want transformations
        createInfo.compositeAlpha = VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR;
        createInfo.presentMode = presentMode;
        createInfo.clipped = VK_TRUE;

        // create swap chain
        if (vkCreateSwapchainKHR(device, &createInfo, nullptr, &swapChain) != VK_SUCCESS) {
            throw std::runtime_error("failed to create swap chain!");
        }

        // retreive handles
        vkGetSwapchainImagesKHR(device, swapChain, &imageCount, nullptr);
        swapChainImages.resize(imageCount);
        vkGetSwapchainImagesKHR(device, swapChain, &imageCount, swapChainImages.data());

        swapChainImageFormat = surfaceFormat.format;
        swapChainExtent = extent;
    }

    void createImageViews() {
        // resize the list to fit all of the image views we will be creating
        swapChainImageViews.resize(swapChainImages.size());
        // loop that iterates over the swap chain images
        for (size_t i = 0; i < swapChainImages.size(); i++) {
            VkImageViewCreateInfo createInfo{};
            createInfo.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO;
            createInfo.image = swapChainImages[i];
            // how data should be interpreted
            createInfo.viewType = VK_IMAGE_VIEW_TYPE_2D;    // 2d, 3d, cube map available
            createInfo.format = swapChainImageFormat;
            // components for color channels
            createInfo.components.r = VK_COMPONENT_SWIZZLE_IDENTITY;
            createInfo.components.g = VK_COMPONENT_SWIZZLE_IDENTITY;
            createInfo.components.b = VK_COMPONENT_SWIZZLE_IDENTITY;
            createInfo.components.a = VK_COMPONENT_SWIZZLE_IDENTITY;
            // describes image purpose and which part of image is accessed
            createInfo.subresourceRange.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT;
            createInfo.subresourceRange.baseMipLevel = 0;
            createInfo.subresourceRange.levelCount = 1;
            createInfo.subresourceRange.baseArrayLayer = 0;
            createInfo.subresourceRange.layerCount = 1;

            // if creating stereographic, then create swap chain with multiple layers

            if (vkCreateImageView(device, &createInfo, nullptr, &swapChainImageViews[i]) != VK_SUCCESS) {
                throw std::runtime_error("failed to create image views!");
            }
        }
    }

    void createRenderPass() {
        // single buffer attachment represented by one of the images from the swap chain
        VkAttachmentDescription colorAttachment{};
        colorAttachment.format = swapChainImageFormat;      // should match swapchain images
        colorAttachment.samples = VK_SAMPLE_COUNT_1_BIT;    // not doing multisampling
        colorAttachment.loadOp = VK_ATTACHMENT_LOAD_OP_CLEAR;   // can preserve, clear, or not care about existing constents
        colorAttachment.storeOp = VK_ATTACHMENT_STORE_OP_STORE; // rendered contents stored in memory and can be read later
        colorAttachment.stencilLoadOp = VK_ATTACHMENT_LOAD_OP_DONT_CARE;    // store operation here - we dont do anything with stencil for now
        colorAttachment.stencilStoreOp = VK_ATTACHMENT_STORE_OP_DONT_CARE;
        colorAttachment.initialLayout = VK_IMAGE_LAYOUT_UNDEFINED;  // tectures and framebuffers are represented by VkImage objects with certain pixel formats
        colorAttachment.finalLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR;  // imgs to be presented to the swap chain
                // """COLOR_ATTACHMENT_OPTIONAL imgs used as color attachment
                // """TRANSFER_DST_OPTIMAL imgs used as destination for a memory copy op

        // subpasses depend on the contents of framebuffers in previous passes
        // for now, stick to a single subpass
        VkAttachmentReference colorAttachmentRef{};
        colorAttachmentRef.attachment = 0;  // index 0 since we have a single VkAttachmentDescription
        colorAttachmentRef.layout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL;

        // explicit graphics subpass
        VkSubpassDescription subpass{};
        subpass.pipelineBindPoint = VK_PIPELINE_BIND_POINT_GRAPHICS;
        subpass.colorAttachmentCount = 1;   // index from the fragment shader with layout(location = 0) out vec4 outColor
        subpass.pColorAttachments = &colorAttachmentRef;
        // there are 4 more things that can be referenced by a subpass

        // subpasses take care of img layout transitions; we need to handle start and end of render pass
        VkSubpassDependency dependency{};
        // indicies of dependency and dependent subpass
        dependency.srcSubpass = VK_SUBPASS_EXTERNAL;
        dependency.dstSubpass = 0;
        // what ops to wait on and the stages in which these ops occur
        dependency.srcStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
        dependency.srcAccessMask = 0;
        dependency.dstStageMask = VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT;
        dependency.dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT;

        // the VkAttachmentReference objects reference attachments using indices of this array
        VkRenderPassCreateInfo renderPassInfo{};
        renderPassInfo.sType = VK_STRUCTURE_TYPE_RENDER_PASS_CREATE_INFO;
        renderPassInfo.attachmentCount = 1;
        renderPassInfo.pAttachments = &colorAttachment;
        renderPassInfo.subpassCount = 1;
        renderPassInfo.pSubpasses = &subpass;
        renderPassInfo.dependencyCount = 1;
        renderPassInfo.pDependencies = &dependency;

        if (vkCreateRenderPass(device, &renderPassInfo, nullptr, &renderPass) != VK_SUCCESS) {
            throw std::runtime_error("failed to create render pass!");
        }
    }

    void createGraphicsPipeline() {
        auto vertShaderCode = readFile("shaders/vert.spv");
        auto fragShaderCode = readFile("shaders/frag.spv");
        // local vars because we can destroy shader modules once pipeline created
        VkShaderModule vertShaderModule = createShaderModule(vertShaderCode);
        VkShaderModule fragShaderModule = createShaderModule(fragShaderCode);

        VkPipelineShaderStageCreateInfo vertShaderStageInfo{};
        vertShaderStageInfo.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
        // which pipeline stage the shader is going to be used
        vertShaderStageInfo.stage = VK_SHADER_STAGE_VERTEX_BIT;
        vertShaderStageInfo.module = vertShaderModule;
        // entry point - possible to have different ones
        vertShaderStageInfo.pName = "main";
        // pSpecializationInfo allows you to specify values for shader constants

        // modify structure to suit fragment shader
        VkPipelineShaderStageCreateInfo fragShaderStageInfo{};
        fragShaderStageInfo.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
        fragShaderStageInfo.stage = VK_SHADER_STAGE_FRAGMENT_BIT;
        fragShaderStageInfo.module = fragShaderModule;
        fragShaderStageInfo.pName = "main";

        VkPipelineShaderStageCreateInfo shaderStages[] = {vertShaderStageInfo, fragShaderStageInfo};

        // vertex data to be passed to vertex shader with two descriptions
        VkPipelineVertexInputStateCreateInfo vertexInputInfo{};
        vertexInputInfo.sType = VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO;
        // bindings: space between data and whether data is per-vertex or per-instance
        vertexInputInfo.vertexBindingDescriptionCount = 0;
        // attribute descriptions: types of attributes passed
        vertexInputInfo.vertexAttributeDescriptionCount = 0;

        // what kind of geometry will be drawn from the vertices and if primitive restart should be enabled (topology)
        VkPipelineInputAssemblyStateCreateInfo inputAssembly{};
        inputAssembly.sType = VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO;
        inputAssembly.topology = VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST;
        //inputAssembly.topology = VK_PRIMITIVE_TOPOLOGY_LINE_STRIP;
        // normally vertices are loaded into buffer in sequential order, but with an element buffer you can specify the indices
        //      this allows for reusing vertices -> if this was set to VK_TRUE, its possible yo break up lines and triangles
        inputAssembly.primitiveRestartEnable = VK_FALSE;

        // w/o dynamic state, viewport and scissor for this pipeline would be immutable
        VkPipelineViewportStateCreateInfo viewportState{};
        viewportState.sType = VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO;
        viewportState.viewportCount = 1;
        viewportState.scissorCount = 1;

        // for coloring
        VkPipelineRasterizationStateCreateInfo rasterizer{};
        rasterizer.sType = VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO;
        rasterizer.depthClampEnable = VK_FALSE;     // if set to true, fragments out of planes are clamped rather than discarded -> shadow maps
        rasterizer.rasterizerDiscardEnable = VK_FALSE;
        rasterizer.polygonMode = VK_POLYGON_MODE_FILL;  // could be line or point, reqruies enabling gpu feature
        rasterizer.lineWidth = 1.0f;    // any line thicker than 1.0 requires enabling wideLines GPU feature
        rasterizer.cullMode = VK_CULL_MODE_BACK_BIT;
        rasterizer.frontFace = VK_FRONT_FACE_CLOCKWISE;
        rasterizer.depthBiasEnable = VK_FALSE;  // alters depth value -> shadow mapping

        // multisampling performs anti-aliasing 
        VkPipelineMultisampleStateCreateInfo multisampling{};
        multisampling.sType = VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO;
        multisampling.sampleShadingEnable = VK_FALSE;
        multisampling.rasterizationSamples = VK_SAMPLE_COUNT_1_BIT;

        // after a fragment shader has returned a color, it needs to be combined with the color that is already in the frame buffer
        //  either mix old and new or combine old and new with bitwise
        // VkPipelineColorBlendAttachmentStateCreateInfo contains global color blending settings (multiple framebuffers)
        VkPipelineColorBlendAttachmentState colorBlendAttachment{};
        colorBlendAttachment.colorWriteMask = VK_COLOR_COMPONENT_R_BIT | VK_COLOR_COMPONENT_G_BIT | VK_COLOR_COMPONENT_B_BIT | VK_COLOR_COMPONENT_A_BIT;
        colorBlendAttachment.blendEnable = VK_FALSE;    // new color passed through unmodified
        // alpha blending: new color blended with old based on opacity (not done here)

        // sets blend constants to use to calculations
        VkPipelineColorBlendStateCreateInfo colorBlending{};
        colorBlending.sType = VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO;
        colorBlending.logicOpEnable = VK_FALSE;
        colorBlending.logicOp = VK_LOGIC_OP_COPY;
        colorBlending.attachmentCount = 1;
        colorBlending.pAttachments = &colorBlendAttachment;
        colorBlending.blendConstants[0] = 0.0f;
        colorBlending.blendConstants[1] = 0.0f;
        colorBlending.blendConstants[2] = 0.0f;
        colorBlending.blendConstants[3] = 0.0f;

        // for using dynamic state - ignore special configurations of pipeline
        std::vector<VkDynamicState> dynamicStates = {
            VK_DYNAMIC_STATE_VIEWPORT,
            VK_DYNAMIC_STATE_SCISSOR
        };
        VkPipelineDynamicStateCreateInfo dynamicState{};
        dynamicState.sType = VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO;
        dynamicState.dynamicStateCount = static_cast<uint32_t>(dynamicStates.size());
        dynamicState.pDynamicStates = dynamicStates.data();

        // uniform values in shaders can be changed at drawing time to alter behavior of shaders
        //  used to pass transformation matrix or create texture samplers in the fragment shader
        VkPipelineLayoutCreateInfo pipelineLayoutInfo{};
        pipelineLayoutInfo.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
        pipelineLayoutInfo.setLayoutCount = 0;
        pipelineLayoutInfo.pushConstantRangeCount = 0;

        if (vkCreatePipelineLayout(device, &pipelineLayoutInfo, nullptr, &pipelineLayout) != VK_SUCCESS) {
            throw std::runtime_error("failed to create pipeline layout!");
        }

        // finally we can fill everything in this struct
        VkGraphicsPipelineCreateInfo pipelineInfo{};
        // referencing array of VkPipelineShaderStageCreateInfo structs
        pipelineInfo.sType = VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO;
        pipelineInfo.stageCount = 2;
        pipelineInfo.pStages = shaderStages;
        // reference all structs describing fixed-fctn stage
        pipelineInfo.pVertexInputState = &vertexInputInfo;
        pipelineInfo.pInputAssemblyState = &inputAssembly;
        pipelineInfo.pViewportState = &viewportState;
        pipelineInfo.pRasterizationState = &rasterizer;
        pipelineInfo.pMultisampleState = &multisampling;
        pipelineInfo.pColorBlendState = &colorBlending;
        pipelineInfo.pDynamicState = &dynamicState;
        pipelineInfo.layout = pipelineLayout;       // pipeline layout: vulkan handle rather than struct ptr
        // reference to render and subpass
        pipelineInfo.renderPass = renderPass;
        pipelineInfo.subpass = 0;
        pipelineInfo.basePipelineHandle = VK_NULL_HANDLE;   // vulkan allows you to create a new graphics pipeline by deriving from an existing one

        // creation!!
        // designed to take multiple pipelineCreateInfo objects and create multiple VkPipeline objects in one call
        // VK_NULL_HANDLE references VkPipelineCache obj
        if (vkCreateGraphicsPipelines(device, VK_NULL_HANDLE, 1, &pipelineInfo, nullptr, &graphicsPipeline) != VK_SUCCESS) {
            throw std::runtime_error("failed to create graphics pipeline!");
        }

        vkDestroyShaderModule(device, fragShaderModule, nullptr);
        vkDestroyShaderModule(device, vertShaderModule, nullptr);
    }

    void createFramebuffers() {
        // resize container to hold all framebuffers
        swapChainFramebuffers.resize(swapChainImageViews.size());

        // iterate through image views and create framebuffers for them
        for (size_t i = 0; i < swapChainImageViews.size(); i++) {
            VkImageView attachments[] = {
                swapChainImageViews[i]
            };

            VkFramebufferCreateInfo framebufferInfo{};
            framebufferInfo.sType = VK_STRUCTURE_TYPE_FRAMEBUFFER_CREATE_INFO;
            framebufferInfo.renderPass = renderPass;    // specify which rednerPass the framebuffer needs to be compatible
            framebufferInfo.attachmentCount = 1;    // specify VkImageView objs
            framebufferInfo.pAttachments = attachments;
            framebufferInfo.width = swapChainExtent.width;
            framebufferInfo.height = swapChainExtent.height;
            framebufferInfo.layers = 1; // our swap chaim images are single images, so no. of layers is 1

            if (vkCreateFramebuffer(device, &framebufferInfo, nullptr, &swapChainFramebuffers[i]) != VK_SUCCESS) {
                throw std::runtime_error("failed to create framebuffer!");
            }
        }
    }

    void createCommandPool() {
        QueueFamilyIndices queueFamilyIndices = findQueueFamilies(physicalDevice);

        VkCommandPoolCreateInfo poolInfo{};
        poolInfo.sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO;
        poolInfo.flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT;   // allow command buffers to be rerecorded individually
        poolInfo.queueFamilyIndex = queueFamilyIndices.graphicsFamily.value();

        // command buffers are executed by submitting them on one of the device queues
        if (vkCreateCommandPool(device, &poolInfo, nullptr, &commandPool) != VK_SUCCESS) {
            throw std::runtime_error("failed to create command pool!");
        }
    }

    // automatically freed
    void createCommandBuffers() {
        commandBuffers.resize(MAX_FRAMES_IN_FLIGHT);

        VkCommandBufferAllocateInfo allocInfo{};
        allocInfo.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
        allocInfo.commandPool = commandPool;
        allocInfo.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;  // primary can be submitted to a queue for execution, but cannot be called from other command buffers
            // seconday usefil to reuse common operations
        allocInfo.commandBufferCount = (uint32_t) commandBuffers.size();

        if (vkAllocateCommandBuffers(device, &allocInfo, commandBuffers.data()) != VK_SUCCESS) {
            throw std::runtime_error("failed to allocate command buffers!");
        }
    }

    // writes commands we want to execute into a command buffer
    void recordCommandBuffer(VkCommandBuffer commandBuffer, uint32_t imageIndex) {
        VkCommandBufferBeginInfo beginInfo{};
        beginInfo.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
        // beginInfo.flags = 0; // how we will use the cmd buffer - 3 options
        // beginInfo.pInheritanceInfo = nullptr; // relevant for secondary command buffers

        if (vkBeginCommandBuffer(commandBuffer, &beginInfo) != VK_SUCCESS) {
            throw std::runtime_error("failed to begin recording command buffer!");
        }

        VkRenderPassBeginInfo renderPassInfo{};
        renderPassInfo.sType = VK_STRUCTURE_TYPE_RENDER_PASS_BEGIN_INFO;
        renderPassInfo.renderPass = renderPass;
        renderPassInfo.framebuffer = swapChainFramebuffers[imageIndex];
        renderPassInfo.renderArea.offset = {0, 0};  // size of render area
        renderPassInfo.renderArea.extent = swapChainExtent;

        // background color
        VkClearValue clearColor = {{{0.0f, 0.0f, 0.0f, 1.0f}}};
        renderPassInfo.clearValueCount = 1;
        renderPassInfo.pClearValues = &clearColor;

        // (commandBuffer, details of render pass, how drawing commands w/in the render pass will be provided)
        // inline for primary, secondary_command_buffers for secondary
        vkCmdBeginRenderPass(commandBuffer, &renderPassInfo, VK_SUBPASS_CONTENTS_INLINE);
            // drawing !!
            vkCmdBindPipeline(commandBuffer, VK_PIPELINE_BIND_POINT_GRAPHICS, graphicsPipeline);

            // viewport describes region of the framebuffer that output will be rendered to
            VkViewport viewport{};
            viewport.x = 0.0f;
            viewport.y = 0.0f;
            viewport.width = (float) swapChainExtent.width;
            viewport.height = (float) swapChainExtent.height;
            viewport.minDepth = 0.0f;
            viewport.maxDepth = 1.0f;
            vkCmdSetViewport(commandBuffer, 0, 1, &viewport);

            // while viewports define the tranformation of the image to the frambuffer, (stretch)
            // scissor rectangles define what regions pixels will actually be stored (visible)
            // this set tup uses entire framebuffer
            VkRect2D scissor{};
            scissor.offset = {0, 0};
            scissor.extent = swapChainExtent;
            vkCmdSetScissor(commandBuffer, 0, 1, &scissor);

            vkCmdDraw(commandBuffer, 4, 1, 0, 0);   // traingle cmd (vertexCnt, instanceCt, firstVertex, firstInstance)

        vkCmdEndRenderPass(commandBuffer);

        if (vkEndCommandBuffer(commandBuffer) != VK_SUCCESS) {
            throw std::runtime_error("failed to record command buffer!");
        }
    }

    // synchronization for rendering
    void createSyncObjects() {
        imageAvailableSemaphores.resize(MAX_FRAMES_IN_FLIGHT);
        renderFinishedSemaphores.resize(MAX_FRAMES_IN_FLIGHT);
        inFlightFences.resize(MAX_FRAMES_IN_FLIGHT);

        VkSemaphoreCreateInfo semaphoreInfo{};
        semaphoreInfo.sType = VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO;

        VkFenceCreateInfo fenceInfo{};
        fenceInfo.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
        fenceInfo.flags = VK_FENCE_CREATE_SIGNALED_BIT;

        for (size_t i = 0; i < MAX_FRAMES_IN_FLIGHT; i++) {
            if (vkCreateSemaphore(device, &semaphoreInfo, nullptr, &imageAvailableSemaphores[i]) != VK_SUCCESS ||
                vkCreateSemaphore(device, &semaphoreInfo, nullptr, &renderFinishedSemaphores[i]) != VK_SUCCESS ||
                vkCreateFence(device, &fenceInfo, nullptr, &inFlightFences[i]) != VK_SUCCESS) {
                throw std::runtime_error("failed to create synchronization objects for a frame!");
            }
        }
    }

    void drawFrame() {
        // wait to draw frame until previous is finished
        vkWaitForFences(device, 1, &inFlightFences[currentFrame], VK_TRUE, UINT64_MAX);

        // acquire img from swapchain
        uint32_t imageIndex;
        // third param is a timeout for an img to become available, last param is for the index of the swap chain img that becomes available
        // vkAcquireNextImageKHR tells us when we need to recreate the swapchain w/ ERROR_OUT_OF_DATE or SUBOPTIMAL_KHR
        VkResult result = vkAcquireNextImageKHR(device, swapChain, UINT64_MAX, imageAvailableSemaphores[currentFrame], VK_NULL_HANDLE, &imageIndex);

        // avoiding deadlock
        if (result == VK_ERROR_OUT_OF_DATE_KHR) {
            recreateSwapChain();
            return;
        } else if (result != VK_SUCCESS && result != VK_SUBOPTIMAL_KHR) {
            throw std::runtime_error("failed to acquire swap chain image!");
        }

        vkResetFences(device, 1, &inFlightFences[currentFrame]);

        vkResetCommandBuffer(commandBuffers[currentFrame], /*VkCommandBufferResetFlagBits*/ 0);
        recordCommandBuffer(commandBuffers[currentFrame], imageIndex);

        // submitting cmd buffer info
        VkSubmitInfo submitInfo{};
        submitInfo.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;

        VkSemaphore waitSemaphores[] = {imageAvailableSemaphores[currentFrame]};
        VkPipelineStageFlags waitStages[] = {VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT};
        submitInfo.waitSemaphoreCount = 1;
        submitInfo.pWaitSemaphores = waitSemaphores;
        submitInfo.pWaitDstStageMask = waitStages;

        submitInfo.commandBufferCount = 1;
        submitInfo.pCommandBuffers = &commandBuffers[currentFrame];

        VkSemaphore signalSemaphores[] = {renderFinishedSemaphores[currentFrame]};
        submitInfo.signalSemaphoreCount = 1;
        submitInfo.pSignalSemaphores = signalSemaphores;

        // submission
        if (vkQueueSubmit(graphicsQueue, 1, &submitInfo, inFlightFences[currentFrame]) != VK_SUCCESS) {
            throw std::runtime_error("failed to submit draw command buffer!");
        }

        // submit result back to swapchain to that it will shown on screen
        VkPresentInfoKHR presentInfo{};
        presentInfo.sType = VK_STRUCTURE_TYPE_PRESENT_INFO_KHR;

        // wait till shape drawn
        presentInfo.waitSemaphoreCount = 1;
        presentInfo.pWaitSemaphores = signalSemaphores;
        // what swapchains to submit img to
        VkSwapchainKHR swapChains[] = {swapChain};
        presentInfo.swapchainCount = 1;
        presentInfo.pSwapchains = swapChains;

        presentInfo.pImageIndices = &imageIndex;

        result = vkQueuePresentKHR(presentQueue, &presentInfo);

        if (result == VK_ERROR_OUT_OF_DATE_KHR || result == VK_SUBOPTIMAL_KHR || framebufferResized) {
            // handle resizing
            framebufferResized = false;
            recreateSwapChain();
        } else if (result != VK_SUCCESS) {
            throw std::runtime_error("failed to present swap chain image!");
        }

        // advance frame
        currentFrame = (currentFrame + 1) % MAX_FRAMES_IN_FLIGHT;
    }

    // create VkShaderModule
    VkShaderModule createShaderModule(const std::vector<char>& code) {
        VkShaderModuleCreateInfo createInfo{};
        createInfo.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
        createInfo.codeSize = code.size();
        // file encoded in bytes as uint32_t rather than char ptr - cast
        createInfo.pCode = reinterpret_cast<const uint32_t*>(code.data());

        VkShaderModule shaderModule;
        if (vkCreateShaderModule(device, &createInfo, nullptr, &shaderModule) != VK_SUCCESS) {
            throw std::runtime_error("failed to create shader module!");
        }

        return shaderModule;
    }

    VkSurfaceFormatKHR chooseSwapSurfaceFormat(const std::vector<VkSurfaceFormatKHR>& availableFormats) {
        // format: color channels and types
        // colorspace: indicates if SRGB color space is supported (we want this)
        for (const auto& availableFormat : availableFormats) {
            if (availableFormat.format == VK_FORMAT_B8G8R8A8_SRGB && availableFormat.colorSpace == VK_COLOR_SPACE_SRGB_NONLINEAR_KHR) {
                return availableFormat;
            }
        }
        // we can settle for first format specified
        return availableFormats[0];
    }

    // 4 possible modes
    // VK_PRESENT_MODE_IMMEDIATE_KHR: images submitted are transferred to the screen right away (tearing)
    // VK_PRESENT_MODE_FIFO_KHR: swap chain is a queue
    // VK_PRESENT_MODE_FIFO_RELAXED_KHR: only differs from prior if the application was late and queue was empty at last vertical blank: images transfers right away (tearing)
    // VK_PRESENT_MODE_MAILBOX_KHR: another variation of second mode but avoids tearing
    VkPresentModeKHR chooseSwapPresentMode(const std::vector<VkPresentModeKHR>& availablePresentModes) {
        for (const auto& availablePresentMode : availablePresentModes) {
            if (availablePresentMode == VK_PRESENT_MODE_MAILBOX_KHR) {
                return availablePresentMode;
            }
        }

        return VK_PRESENT_MODE_FIFO_KHR;
    }

    // swap extent is the resolution of the swap chain images and is equal to the resolution of the window we're trying to draw in pixels - match by setting width and height to currentExtent member
    VkExtent2D chooseSwapExtent(const VkSurfaceCapabilitiesKHR& capabilities) {
        if (capabilities.currentExtent.width != std::numeric_limits<uint32_t>::max()) {
            return capabilities.currentExtent;
        } else {
            int width, height;
            // queries the resolution of window in pixels
            glfwGetFramebufferSize(window, &width, &height);

            VkExtent2D actualExtent = {
                static_cast<uint32_t>(width),
                static_cast<uint32_t>(height)
            };

            // clamp bounds the values of width and height between allowed values
            actualExtent.width = std::clamp(actualExtent.width, capabilities.minImageExtent.width, capabilities.maxImageExtent.width);
            actualExtent.height = std::clamp(actualExtent.height, capabilities.minImageExtent.height, capabilities.maxImageExtent.height);

            return actualExtent;
        }
    }

    // fctn to populate swapChainSupportDetails struct
    SwapChainSupportDetails querySwapChainSupport(VkPhysicalDevice device) {
        SwapChainSupportDetails details;
        // basic surface capabilities
        vkGetPhysicalDeviceSurfaceCapabilitiesKHR(device, surface, &details.capabilities);
        // supported surface formats (list of two structures -> 2 fctn calls)
        uint32_t formatCount;
        vkGetPhysicalDeviceSurfaceFormatsKHR(device, surface, &formatCount, nullptr);

        if (formatCount != 0) {
            details.formats.resize(formatCount);
            vkGetPhysicalDeviceSurfaceFormatsKHR(device, surface, &formatCount, details.formats.data());
        }
        // supported presentation mode
        uint32_t presentModeCount;
        vkGetPhysicalDeviceSurfacePresentModesKHR(device, surface, &presentModeCount, nullptr);

        if (presentModeCount != 0) {
            details.presentModes.resize(presentModeCount);
            vkGetPhysicalDeviceSurfacePresentModesKHR(device, surface, &presentModeCount, details.presentModes.data());
        }

        return details;
    }

    // evaluates graphics cards
    bool isDeviceSuitable(VkPhysicalDevice device) {
        QueueFamilyIndices indices = findQueueFamilies(device);

        bool extensionsSupported = checkDeviceExtensionSupport(device);

        bool swapChainAdequate = false;
        if (extensionsSupported) {
            SwapChainSupportDetails swapChainSupport = querySwapChainSupport(device);
            swapChainAdequate = !swapChainSupport.formats.empty() && !swapChainSupport.presentModes.empty();
        }

        return indices.isComplete() && extensionsSupported && swapChainAdequate;
    }

    bool checkDeviceExtensionSupport(VkPhysicalDevice device) {
        // get number of extensions
        uint32_t extensionCount;
        vkEnumerateDeviceExtensionProperties(device, nullptr, &extensionCount, nullptr);
        // allocate an array to hold the extensions details
        std::vector<VkExtensionProperties> availableExtensions(extensionCount);
        // query extension details
        vkEnumerateDeviceExtensionProperties(device, nullptr, &extensionCount, availableExtensions.data());

        std::set<std::string> requiredExtensions(deviceExtensions.begin(), deviceExtensions.end());

        for (const auto& extension : availableExtensions) {
            requiredExtensions.erase(extension.extensionName);
        }

        return requiredExtensions.empty();
    }

    // look for all queue families we need
    QueueFamilyIndices findQueueFamilies(VkPhysicalDevice device) {
        // logic to find queue family indices to populate struct with
        QueueFamilyIndices indices;
        // asign index to queue families that were found

        // retreiving list of qf
        uint32_t queueFamilyCount = 0;
        vkGetPhysicalDeviceQueueFamilyProperties(device, &queueFamilyCount, nullptr);

        std::vector<VkQueueFamilyProperties> queueFamilies(queueFamilyCount);
        vkGetPhysicalDeviceQueueFamilyProperties(device, &queueFamilyCount, queueFamilies.data());

        int i = 0;
        for (const auto& queueFamily : queueFamilies) {
            // need at least one that supports graphics bit
            if (queueFamily.queueFlags & VK_QUEUE_GRAPHICS_BIT) {
                indices.graphicsFamily = i;
            }

            // find queue capable of presenting to window
            VkBool32 presentSupport = false;
            vkGetPhysicalDeviceSurfaceSupportKHR(device, i, surface, &presentSupport);

            if (presentSupport) {
                indices.presentFamily = i;
            }

            if (indices.isComplete()) {
                break;
            }

            i++;
        }

        return indices;
    }

    // you need an extension to inferface with window system
    // will return the required list of extensions based on whether validation layers are enabled or not
    std::vector<const char*> getRequiredExtensions() {
        uint32_t glfwExtensionCount = 0;
        const char** glfwExtensions;
        glfwExtensions = glfwGetRequiredInstanceExtensions(&glfwExtensionCount);

        std::vector<const char*> extensions(glfwExtensions, glfwExtensions + glfwExtensionCount);

        if (enableValidationLayers) {
            extensions.push_back(VK_EXT_DEBUG_UTILS_EXTENSION_NAME);
        }

        return extensions;
    }

    // checks if requested layers are available
    bool checkValidationLayerSupport() {
        uint32_t layerCount;
        vkEnumerateInstanceLayerProperties(&layerCount, nullptr);
        // create a list of all validation layers
        std::vector<VkLayerProperties> availableLayers(layerCount);
        vkEnumerateInstanceLayerProperties(&layerCount, availableLayers.data());

        // check if all validation layers exist in the available layers list
        for (const char* layerName : validationLayers) {
            bool layerFound = false;

            for (const auto& layerProperties : availableLayers) {
                if (strcmp(layerName, layerProperties.layerName) == 0) {
                    layerFound = true;
                    break;
                }
            }

            if (!layerFound) {
                return false;
            }
        }

        return true;
    }

    // load shader files, which are now binary
    static std::vector<char> readFile(const std::string& filename) {
        std::ifstream file(filename, std::ios::ate | std::ios::binary);

        if (!file.is_open()) {
            throw std::runtime_error("failed to open file!");
        }

        size_t fileSize = (size_t) file.tellg();
        std::vector<char> buffer(fileSize);

        file.seekg(0);
        file.read(buffer.data(), fileSize);

        file.close();

        return buffer;
    }

    // (diagnostic message, info message, bug message, invalid message)
    // pCallBack data contains debug message, array of vulkan objs related to message, and number of objs in array
    static VKAPI_ATTR VkBool32 VKAPI_CALL debugCallback(VkDebugUtilsMessageSeverityFlagBitsEXT messageSeverity, VkDebugUtilsMessageTypeFlagsEXT messageType, const VkDebugUtilsMessengerCallbackDataEXT* pCallbackData, void* pUserData) {
        std::cerr << "validation layer: " << pCallbackData->pMessage << std::endl;

        return VK_FALSE;
    }
};

int main() {
    HelloTriangleApplication app;

    try {
        app.run();
    } catch (const std::exception& e) {
        std::cerr << e.what() << std::endl;
        return EXIT_FAILURE;
    }

    return EXIT_SUCCESS;
}

/*
TOC:
(SETUP)
-resource management (create, cleanup)
-integreating GLFW (initWindow)
-initialize Vulkan library by creating an instance
-validation layers (help with debugging)
    -need to be enabled by specifying their name
    -print debug messages to stdio default. but we can provide explicit callback
    -SDK provides validation layers - VK_LAYER_KHRONOS_validation
-selecting a physical device (graphics card)
-queue families
-select logical device (to interface with physical device)
    -queues
    -device features
    -logical device creation
(PRESENTATION)
-window surface (vulkan cannot interface directly with window system on its own
    we need to use the window system integration extension)
    -VK_KHR_surface: instance level extension
    -querying for presentation support (isDeviceSuitable extension)
    -create presentation queue
-swap chain (infrastructure to own buffers we render before we visualize them)
    -checking for swap chain support (isDeviceSuitable extension)
    -enabling device extensions
    -querying details of swap chain support
    -choosing the right settings for the swap chain (surface format, presentation mode, swap extent)
    -creating the swap chain
    -retreiving the swap chain images
-image views (describes how to access an image and which [art to acces])
(GRAPHICS PIPELINE BASICS)
-intro
    -graphics pipeline is a sqnce of ops that take the vertices and textures of you meshes all the way to the pixels in render targets
    -> vertex/index buffer 
    -> input assembler : collects raw vertex data
    -> (p) vertex shader : run for every vertex and applies transformations from model to screen space
    -> (p) tessellation : subdivide geometry to increase mesh quality
    -> (p) geometry shader : run on every primitive (triangle, line, pt) 
    -> rasterization : discretizes primitives into fragments; pizel elements they fill on the fram buffer
    -> (p) fragment shader : invoked for every fragment that survives and determines which framebuffer the fragments are written to (can include texture and lighting)
    -> color blending : applies operations to mix fragments that map to the same pixel
    -(p) for programmable: can upload own code to graphics card; if you want to change pipeline, have to build it from scratch
-shader modules (specified in bytecode format - use glslc.exe to compile) define functionality if the programmable pipeline stages
    -vertex shader (currently hardcoded)
    -fragment shader
-fixed functions: all of the structs that define fixed-function stages
    -a limited amount of the pipeline state can be changed w/o recreating the pipeline at draw time (eg size of viewport, line width, blend constants)
        if you want to use the dynamic state and keep these properties out, you have to fill in the VkPipelineDynamicStateCreateInfo struct
    -viewport and scissor rectangle
    -rasterizer: takes geometry shaped from vertex shader and turns it into fragments to be colored
    -color blending
    -pipeline layout: unfirorm and push values referenced by the shader
-render pass : need to tell vulkan about the framebuffer attachments: how many color and depth buffers, how many samples. how contents should be handled
(DRAWING)
-framebuffers : a framebuffer references all of the VkImageView objs (for now it is only color attachment)
-command buffers : you have to record all the operations you want to perform in command buffer objs
    -command pools: manage memory that is used to store the buffers 
    -command buffer allocation and recording
    -starting a render pass: drawing starts by beginning the render pass with vkcmdBeginRenderPass
-rendering and presentation
    -frame outline: wait for previous frame to finish, get img from swapchain, record a cmd buffer that draws the scene onto the img, submit the recorded cmd buffer, present the swapchain img
    -synchronization on the execution of the GPU is explicit: specify ordering
        -a semaphore is used to add order between queue ops. binary (our use) or timeline
            signaled or unsignaled. op A signals sempahore S when it finished executing, and op B will wait on sempahore S before it begins executing
        -fences synchronize execution on the CPU. also signalled or unsignalled. 
-frames in flight: allow the rendering of one frame to not interfere with the recording of the next -> need multiple cmd buffers, semaphores, fences
(SWAP CHAIN RECREATION)
-swap chain recreation: necessary to handle more circumstances like window resizing

(VERTEX BUFFERS - instead of hardcoding shader)

*/

/*
Its possible to perform automatic resource management using RA2 or smart ptrs provided in the <memory> header
vulkan obkects created directly with vkCreateXX or allocated through other objects with functions like vkAllocateXXX
destroy with vkDestroyXXX or vkFreeXXX. all share pAllocator param that allows you to specify callbacks for a custom memory allocator

many strucuts in vulkan require you to specify the type in the sType member
a lot of info in Vulkan is passed through structs instead of fctn params

pattern:
    ptr to struct with creation info
    ptr to custom allocator callbacks
    ptr to next variable that stores the handle to the new object

almost every operation in Vulkan requires commands to be submitted to a queue

renderdoc?
*/
