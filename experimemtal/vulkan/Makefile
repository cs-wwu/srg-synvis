VULKAN_SDK_PATH = ~/VulkanSDK/1.3.280.1/x86_64

CFLAGS = -std=c++17 -O3 -I$(VULKAN_SDK_PATH)/include
LDFLAGS = -lglfw -lvulkan -ldl -lpthread -lX11 -lXxf86vm -lXrandr -lXi

VulkanTest: main.cpp
	g++ $(CFLAGS) -o VulkanTest main.cpp $(LDFLAGS)

.PHONY: test clean

test: VulkanTest
	./VulkanTest

clean:
	rm -f VulkanTest