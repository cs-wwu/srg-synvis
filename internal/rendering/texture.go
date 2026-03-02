package rendering

import vk "github.com/vulkan-go/vulkan"

type Texture struct {
	sampler vk.Sampler

	image       vk.Image
	imageLayout vk.ImageLayout

	memAlloc *vk.MemoryAllocateInfo
	mem      vk.DeviceMemory
	view     vk.ImageView

	texWidth  int32
	texHeight int32
}

func (t *Texture) Destroy(dev vk.Device) {
	if t.view != vk.NullImageView {
		vk.DestroyImageView(dev, t.view, nil)
		t.view = vk.NullImageView
	}
	if t.mem != vk.NullDeviceMemory {
		vk.FreeMemory(dev, t.mem, nil)
		t.mem = vk.NullDeviceMemory
	}
	if t.image != vk.NullImage {
		vk.DestroyImage(dev, t.image, nil)
		t.image = vk.NullImage
	}
	if t.sampler != vk.NullSampler {
		vk.DestroySampler(dev, t.sampler, nil)
		t.sampler = vk.NullSampler
	}
}

func (t *Texture) DestroyImage(dev vk.Device) {
	vk.FreeMemory(dev, t.mem, nil)
	vk.DestroyImage(dev, t.image, nil)
}
